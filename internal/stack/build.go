package stack

import (
	"archive/tar"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/moby/patternmatcher"
	"github.com/moby/patternmatcher/ignorefile"
)

// streamedDockerfile is where a streamed context carries its Dockerfile. The
// name is Mozza's own so it cannot collide with a file in the app's tree.
const streamedDockerfile = ".mozza-stack.Dockerfile"

// buildImage builds one image with docker. A plain build hands docker the
// context directory and lets it apply the repository's .dockerignore. A build
// with extra excludes streams the context as a tar instead, filtered by the
// repository's .dockerignore plus the excludes — the only way to trim a
// context without writing an ignore file into a tree Mozza must not modify.
func (e *Engine) buildImage(ctx context.Context, img Image) error {
	b := img.Build
	args := []string{"build", "-t", img.Ref}
	if b.Target != "" {
		args = append(args, "--target", b.Target)
	}
	keys := make([]string, 0, len(b.Args))
	for k := range b.Args {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		args = append(args, "--build-arg", k+"="+b.Args[k])
	}

	if b.Remote() {
		if b.Dockerfile != "" {
			args = append(args, "-f", b.Dockerfile)
		}
		args = append(args, b.Context)
		return e.run(ctx, Cmd{Name: "docker", Args: args})
	}
	if len(b.Exclude) == 0 && len(b.Rewrite) == 0 {
		args = append(args, "-f", b.Dockerfile, b.Context)
		return e.run(ctx, Cmd{Name: "docker", Args: args})
	}

	args = append(args, "-f", streamedDockerfile, "-")
	if e.DryRun {
		for _, rw := range b.Rewrite {
			e.info("rewrite Dockerfile: %s", rewriteLabel(rw))
		}
		return e.run(ctx, Cmd{Name: "docker", Args: args})
	}

	df, err := os.ReadFile(b.Dockerfile) //nolint:gosec // a path from the stack definition
	if err != nil {
		return fmt.Errorf("read Dockerfile: %w", err)
	}
	df = e.rewriteDockerfile(df, b.Rewrite)

	pr, pw := io.Pipe()
	tarErr := make(chan error, 1)
	go func() {
		werr := writeContextTar(pw, b.Context, df, b.Exclude)
		pw.CloseWithError(werr)
		tarErr <- werr
	}()
	err = e.run(ctx, Cmd{Name: "docker", Args: args, Stdin: pr})
	_ = pr.Close()
	// A context that failed to stream reaches docker as a truncated tar, and
	// docker's complaint about it (usually a missing Dockerfile) hides the
	// cause; the streaming error is the one worth reporting.
	if terr := <-tarErr; terr != nil && !errors.Is(terr, io.ErrClosedPipe) {
		return fmt.Errorf("stream build context: %w", terr)
	}
	return err
}

// rewriteDockerfile applies a stack's rewrites, reporting each one that
// changed something so the build log says the Dockerfile was not run as
// written.
func (e *Engine) rewriteDockerfile(df []byte, rewrites []Rewrite) []byte {
	for _, rw := range rewrites {
		re := regexp.MustCompile(rw.Pattern) // validated when the stack loaded
		n := len(re.FindAllIndex(df, -1))
		if n == 0 {
			e.info("rewrite Dockerfile: %s — matched nothing", rewriteLabel(rw))
			continue
		}
		df = re.ReplaceAll(df, []byte(rw.Replace))
		e.info("rewrite Dockerfile: %s — %d change(s)", rewriteLabel(rw), n)
	}
	return df
}

func rewriteLabel(rw Rewrite) string {
	if rw.Why != "" {
		return rw.Why
	}
	return rw.Pattern
}

// writeContextTar writes dir as a build-context tar, skipping everything the
// repository's .dockerignore or the extra patterns exclude, and adds the
// Dockerfile content under streamedDockerfile.
func writeContextTar(w io.Writer, dir string, df []byte, extra []string) error {
	patterns, err := readDockerignore(dir)
	if err != nil {
		return err
	}
	patterns = append(patterns, extra...)
	// The ignore file has been applied here; leaving it in the tar would let
	// the builder apply it a second time and possibly drop the Dockerfile.
	patterns = append(patterns, ".dockerignore", streamedDockerfile)

	pm, err := patternmatcher.New(patterns)
	if err != nil {
		return fmt.Errorf("build context: exclude patterns: %w", err)
	}

	tw := tar.NewWriter(w)
	walkErr := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(dir, path)
		if err != nil || rel == "." {
			return err
		}
		rel = filepath.ToSlash(rel)

		excluded, err := pm.MatchesOrParentMatches(rel)
		if err != nil {
			return err
		}
		if excluded {
			// A directory can be skipped whole only when no "!" pattern
			// could re-include something beneath it.
			if d.IsDir() && !pm.Exclusions() {
				return filepath.SkipDir
			}
			return nil
		}
		return addTarEntry(tw, path, rel, d)
	})
	if walkErr != nil {
		return fmt.Errorf("build context %s: %w", dir, walkErr)
	}

	if err := tw.WriteHeader(&tar.Header{Name: streamedDockerfile, Mode: 0o644, Size: int64(len(df)), Typeflag: tar.TypeReg}); err != nil {
		return err
	}
	if _, err := tw.Write(df); err != nil {
		return err
	}
	return tw.Close()
}

func addTarEntry(tw *tar.Writer, path, rel string, d fs.DirEntry) error {
	info, err := d.Info()
	if err != nil {
		return err
	}
	link := ""
	if info.Mode()&fs.ModeSymlink != 0 {
		if link, err = os.Readlink(path); err != nil {
			return err
		}
	}
	if !info.Mode().IsRegular() && !info.IsDir() && link == "" {
		return nil // sockets, devices and pipes have no place in a build context
	}
	hdr, err := tar.FileInfoHeader(info, link)
	if err != nil {
		return err
	}
	hdr.Name = rel
	if info.IsDir() {
		hdr.Name += "/"
	}
	// Ownership from the host means nothing inside the image; zeroing it keeps
	// the context reproducible across machines.
	hdr.Uid, hdr.Gid, hdr.Uname, hdr.Gname = 0, 0, "", ""
	if werr := tw.WriteHeader(hdr); werr != nil {
		return werr
	}
	if !info.Mode().IsRegular() {
		return nil
	}
	f, err := os.Open(path) //nolint:gosec // walking the stack's own build context
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	// Exactly the size the header promised: a log file still being written
	// grows between stat and read, and one byte more aborts the whole tar.
	if _, err := io.CopyN(tw, f, hdr.Size); err != nil {
		return fmt.Errorf("%s changed while it was being read: %w", rel, err)
	}
	return nil
}

func readDockerignore(dir string) ([]string, error) {
	f, err := os.Open(filepath.Join(dir, ".dockerignore")) //nolint:gosec // the build context's own ignore file
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read .dockerignore: %w", err)
	}
	defer func() { _ = f.Close() }()
	patterns, err := ignorefile.ReadAll(f)
	if err != nil {
		return nil, fmt.Errorf("read .dockerignore: %w", err)
	}
	return patterns, nil
}

// imagePresent reports whether docker already has ref.
func (e *Engine) imagePresent(ctx context.Context, ref string) bool {
	if e.DryRun {
		return false
	}
	_, err := e.runner.Output(ctx, Cmd{Name: "docker", Args: []string{"image", "inspect", "--format", "{{.Id}}", ref}})
	return err == nil
}

// nodePlatform is the platform cluster nodes run, which is the docker host's.
// Images are saved for exactly this platform: a multi-platform image saved
// whole references layers the local store never pulled, and the import fails.
func (e *Engine) nodePlatform(ctx context.Context) (string, error) {
	if e.DryRun {
		return "linux/<host-arch>", nil
	}
	out, err := e.runner.Output(ctx, Cmd{Name: "docker", Args: []string{"version", "--format", "{{.Server.Os}}/{{.Server.Arch}}"}})
	if err != nil {
		return "", fmt.Errorf("docker platform: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// imagePlatform is the platform docker holds ref for. Saving for the node
// platform when only another one is present fails outright, and an image
// someone else built (see --image) is often amd64 on an arm64 host.
func (e *Engine) imagePlatform(ctx context.Context, ref, fallback string) string {
	if e.DryRun {
		return fallback
	}
	out, err := e.runner.Output(ctx, Cmd{Name: "docker", Args: []string{"image", "inspect", "--format", "{{.Os}}/{{.Architecture}}", ref}})
	if err != nil {
		return fallback
	}
	if p := strings.TrimSpace(string(out)); p != "/" && p != "" {
		return p
	}
	return fallback
}

// loadImage copies one local image into every cluster node through a
// single-platform archive, then removes the archive.
func (e *Engine) loadImage(ctx context.Context, p provider, ref, platform string) error {
	tarPath := filepath.Join(e.dir, "images", sanitizeRef(ref)+".tar")
	if err := e.mkdirAll(filepath.Dir(tarPath)); err != nil {
		return err
	}
	defer func() {
		if !e.DryRun {
			_ = os.Remove(tarPath)
		}
	}()
	if err := e.run(ctx, Cmd{Name: "docker", Args: []string{"save", "--platform", platform, "-o", tarPath, ref}}); err != nil {
		return err
	}
	// A node's containerd occasionally drops the import stream under load
	// ("error reading from server: EOF"); the same import then succeeds.
	if err := p.importArchive(ctx, tarPath); err != nil {
		e.info("import of %s failed (%v); retrying once", ref, err)
		return p.importArchive(ctx, tarPath)
	}
	return nil
}

func sanitizeRef(ref string) string {
	return strings.NewReplacer("/", "_", ":", "_", "@", "_").Replace(ref)
}
