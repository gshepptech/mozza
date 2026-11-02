package stack

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Engine runs one resolved stack.
type Engine struct {
	stack  *Resolved
	runner Runner
	out    io.Writer
	// dir is the stack's state directory: kubeconfig, generated secrets,
	// rendered values and the kind config.
	dir string

	// DryRun prints every command instead of running it. Queries are not run
	// either, so a dry run shows the full path from an empty machine.
	DryRun bool
	// Rebuild builds images even when docker already has them.
	Rebuild bool
	// ImageOverrides maps a stack image ref to an image to use in its place —
	// local or pullable — instead of building. It covers the source tree that
	// will not build today, and testing an image someone else built.
	ImageOverrides map[string]string

	// freePort is swappable so tests do not bind sockets.
	freePort func() (int, error)

	// ansibleEnv is the running component's ansible.env.
	ansibleEnv map[string]string
}

// NewEngine prepares an engine. stateRoot is usually Home()/run.
func NewEngine(s *Resolved, runner Runner, out io.Writer, stateRoot string) *Engine {
	return &Engine{
		stack:    s,
		runner:   runner,
		out:      out,
		dir:      filepath.Join(stateRoot, s.Name),
		freePort: freeLoopbackPort,
	}
}

// RecordedProvider returns the provider the last `up` of a stack used, so
// commands acting on a running stack find its cluster even when it was
// brought up with --provider.
func RecordedProvider(stateRoot, name string) string {
	b, ok := readFileIfExists(filepath.Join(stateRoot, name, "state.json"))
	if !ok {
		return ""
	}
	var st state
	if json.Unmarshal(b, &st) != nil {
		return ""
	}
	return st.Provider
}

// StateRoot is where per-stack state lives.
func StateRoot() string { return filepath.Join(Home(), "run") }

// state is what Mozza remembers about a stack between commands.
type state struct {
	Provider string `json:"provider"`
	Cluster  string `json:"cluster"`
	// CreatedCluster records that Mozza made the cluster, which is the only
	// case in which `down` will delete it.
	CreatedCluster bool   `json:"createdCluster"`
	Source         string `json:"source,omitempty"`
	// Images maps each loaded ref to the image ID loaded under it, which is
	// how a rebuilt image behind an unchanged tag is noticed. The IDs hold
	// only for the cluster ImagesCluster names (its kube-system UID): a
	// cluster deleted and recreated under the same name has none of them.
	Images        map[string]string `json:"images,omitempty"`
	ImagesCluster string            `json:"imagesCluster,omitempty"`
	UpdatedAt     time.Time         `json:"updatedAt"`
}

func (e *Engine) statePath() string      { return filepath.Join(e.dir, "state.json") }
func (e *Engine) kubeconfigPath() string { return filepath.Join(e.dir, "kubeconfig") }

func (e *Engine) loadState() (state, bool) {
	b, ok := readFileIfExists(e.statePath())
	if !ok {
		return state{}, false
	}
	var st state
	if json.Unmarshal(b, &st) != nil {
		return state{}, false
	}
	return st, true
}

func (e *Engine) saveState(st state) error {
	st.UpdatedAt = time.Now().UTC()
	b, err := json.MarshalIndent(st, "", "  ")
	if err != nil {
		return err
	}
	return e.writeFile(e.statePath(), b)
}

func (e *Engine) step(format string, a ...any) {
	_, _ = fmt.Fprintf(e.out, "==> "+format+"\n", a...)
}

func (e *Engine) info(format string, a ...any) {
	_, _ = fmt.Fprintf(e.out, "    "+format+"\n", a...)
}

// run executes a mutating command, or prints it in a dry run.
func (e *Engine) run(ctx context.Context, c Cmd) error {
	if e.DryRun {
		_, _ = fmt.Fprintf(e.out, "    $ %s\n", c)
		return nil
	}
	return e.runner.Run(ctx, c, e.out)
}

// query executes a read-only command. In a dry run it reports nothing found,
// which is how the plan comes out as the path from an empty machine.
func (e *Engine) query(ctx context.Context, c Cmd) ([]byte, error) {
	if e.DryRun {
		return nil, nil
	}
	return e.runner.Output(ctx, c)
}

// writeFile writes an engine-owned file, private to the user: kubeconfigs,
// rendered values and generated secrets all pass through here.
func (e *Engine) writeFile(path string, b []byte) error {
	if e.DryRun {
		return nil
	}
	if err := e.mkdirAll(filepath.Dir(path)); err != nil {
		return err
	}
	if err := os.WriteFile(path, b, 0o600); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}
	return nil
}

func (e *Engine) mkdirAll(dir string) error {
	if e.DryRun {
		return nil
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}
	return nil
}

func (e *Engine) kubectl(args ...string) Cmd {
	return Cmd{Name: "kubectl", Args: append([]string{"--kubeconfig", e.kubeconfigPath(), "--context", e.stack.KubeContext()}, args...)}
}

func (e *Engine) helm(args ...string) Cmd {
	return Cmd{Name: "helm", Args: append(args, "--kubeconfig", e.kubeconfigPath(), "--kube-context", e.stack.KubeContext())}
}

// Preflight checks every tool the stack needs is on PATH and docker answers.
func (e *Engine) Preflight(ctx context.Context) error {
	p, err := newProvider(e)
	if err != nil {
		return err
	}
	var missing []string
	for _, bin := range []string{"docker", p.binary(), "kubectl", "helm"} {
		if _, err := exec.LookPath(bin); err != nil {
			missing = append(missing, bin)
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("missing tools on PATH: %s", strings.Join(missing, ", "))
	}
	if e.DryRun {
		return nil
	}
	if _, err := e.runner.Output(ctx, Cmd{Name: "docker", Args: []string{"info", "--format", "{{.ServerVersion}}"}}); err != nil {
		return fmt.Errorf("docker is not running: %w", err)
	}
	return nil
}

// Up brings the whole stack up. Every step is idempotent, so re-running Up
// after a failure, or after changing the source, converges instead of
// starting over.
func (e *Engine) Up(ctx context.Context) error {
	s := e.stack
	start := time.Now()

	if err := e.Preflight(ctx); err != nil {
		return err
	}
	if err := e.mkdirAll(e.dir); err != nil {
		return err
	}

	// Images build before the cluster exists: a build is the step most likely
	// to fail, and failing it should not leave a half-made cluster behind.
	if err := e.buildImages(ctx); err != nil {
		return err
	}

	p, err := newProvider(e)
	if err != nil {
		return err
	}
	created, err := e.ensureCluster(ctx, p)
	if err != nil {
		return err
	}

	changed, err := e.loadImages(ctx, p, created || e.clusterUID(ctx) != e.recordedImagesCluster())
	if err != nil {
		return err
	}
	if err := e.labelNodes(ctx); err != nil {
		return err
	}
	if err := e.applySecrets(ctx); err != nil {
		return err
	}
	for _, c := range s.Components {
		if err := e.installComponent(ctx, c); err != nil {
			return fmt.Errorf("component %s: %w", c.Name, err)
		}
	}
	if err := e.restartFor(ctx, changed); err != nil {
		return err
	}

	e.step("%s is up (%s)", s.Name, time.Since(start).Round(time.Second))
	return e.Summary(ctx)
}

func (e *Engine) buildImages(ctx context.Context) error {
	known := map[string]bool{}
	for _, img := range e.stack.Images {
		known[img.Ref] = true
	}
	for ref := range e.ImageOverrides {
		if !known[ref] {
			return fmt.Errorf("--image %s: the stack has no image %s", ref, ref)
		}
	}

	for _, img := range e.stack.Images {
		if src, ok := e.ImageOverrides[img.Ref]; ok {
			if err := e.useImage(ctx, img.Ref, src); err != nil {
				return err
			}
			continue
		}
		if img.Build == nil {
			if !e.Rebuild && e.imagePresent(ctx, img.Ref) {
				e.step("Image %s: present", img.Ref)
				continue
			}
			e.step("Pulling %s", img.Ref)
			if err := e.run(ctx, Cmd{Name: "docker", Args: []string{"pull", img.Ref}}); err != nil {
				return fmt.Errorf("pull %s: %w", img.Ref, err)
			}
			continue
		}
		if !e.Rebuild && e.imagePresent(ctx, img.Ref) {
			e.step("Image %s: present (use --rebuild to build it again)", img.Ref)
			continue
		}
		e.step("Building %s", img.Ref)
		if err := e.buildImage(ctx, img); err != nil {
			return fmt.Errorf("build %s: %w (to use an image you already have instead, pass --image %s=<image>)",
				img.Ref, err, img.Ref)
		}
	}
	return nil
}

// useImage tags src as ref, pulling src first if docker does not have it.
func (e *Engine) useImage(ctx context.Context, ref, src string) error {
	e.step("Image %s: using %s", ref, src)
	if !e.imagePresent(ctx, src) {
		if err := e.run(ctx, Cmd{Name: "docker", Args: []string{"pull", src}}); err != nil {
			return fmt.Errorf("pull %s for %s: %w", src, ref, err)
		}
	}
	if err := e.run(ctx, Cmd{Name: "docker", Args: []string{"tag", src, ref}}); err != nil {
		return fmt.Errorf("tag %s as %s: %w", src, ref, err)
	}
	return nil
}

// ensureCluster creates the cluster or adopts the existing one, and reports
// whether it was created by this call.
func (e *Engine) ensureCluster(ctx context.Context, p provider) (bool, error) {
	s := e.stack
	st, _ := e.loadState()

	exists, err := p.exists(ctx)
	if err != nil {
		return false, err
	}
	if exists {
		e.step("Cluster %s (%s): exists", s.ClusterName(), s.Cluster.Provider)
		if err := p.kubeconfig(ctx, e.kubeconfigPath()); err != nil {
			return false, err
		}
	} else {
		e.step("Creating cluster %s (%s)", s.ClusterName(), s.Cluster.Provider)
		if err := p.create(ctx); err != nil {
			return false, err
		}
		if s.Cluster.Provider == ProviderK3d {
			if err := p.kubeconfig(ctx, e.kubeconfigPath()); err != nil {
				return false, err
			}
		}
		st.CreatedCluster = true
	}

	st.Provider, st.Cluster, st.Source = s.Cluster.Provider, s.ClusterName(), s.SourceDir
	if err := e.saveState(st); err != nil {
		return false, err
	}

	if !e.DryRun {
		kc, ok := readFileIfExists(e.kubeconfigPath())
		if !ok {
			return false, fmt.Errorf("provider wrote no kubeconfig at %s", e.kubeconfigPath())
		}
		if err := assertLoopback(kc); err != nil {
			return false, err
		}
	}
	return !exists, e.waitAPI(ctx)
}

// waitAPI waits for the API server to answer. Nodes may stay NotReady until a
// CNI addon lands, so readiness of the API, not of nodes, is the bar here.
func (e *Engine) waitAPI(ctx context.Context) error {
	if e.DryRun {
		return nil
	}
	deadline := time.Now().Add(3 * time.Minute)
	for {
		if _, err := e.runner.Output(ctx, e.kubectl("get", "--raw", "/readyz")); err == nil {
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("the cluster API did not become ready within 3m")
		}
		if err := sleep(ctx, 2*time.Second); err != nil {
			return err
		}
	}
}

// loadImages loads images into the cluster and returns the refs whose image
// changed since the last `up` loaded them. An image whose ID matches the one
// last loaded is skipped unless the cluster is new, which keeps a re-run of a
// stack with many images from re-importing every one of them.
func (e *Engine) loadImages(ctx context.Context, p provider, fresh bool) ([]string, error) {
	if len(e.stack.Images) == 0 {
		return nil, nil
	}
	platform, err := e.nodePlatform(ctx)
	if err != nil {
		return nil, err
	}
	st, _ := e.loadState()
	if st.Images == nil || fresh {
		st.Images = map[string]string{}
	}
	var changed []string
	for _, img := range e.stack.Images {
		id := e.imageID(ctx, img.Ref)
		if prev, ok := st.Images[img.Ref]; ok && prev == id && id != "" && !fresh {
			e.step("Image %s: already in the cluster", img.Ref)
			continue
		}
		e.step("Loading %s into the cluster", img.Ref)
		imgPlatform := e.imagePlatform(ctx, img.Ref, platform)
		if imgPlatform != platform {
			e.info("%s is %s, nodes are %s: it runs only where the host emulates %s",
				img.Ref, imgPlatform, platform, imgPlatform)
		}
		if err := e.loadImage(ctx, p, img.Ref, imgPlatform); err != nil {
			return nil, fmt.Errorf("load %s: %w", img.Ref, err)
		}
		if prev, ok := st.Images[img.Ref]; ok && prev != id {
			changed = append(changed, img.Ref)
		}
		st.Images[img.Ref] = id
	}
	st.ImagesCluster = e.clusterUID(ctx)
	return changed, e.saveState(st)
}

// clusterUID identifies this incarnation of the cluster: the kube-system
// namespace's UID changes whenever the cluster is recreated, name or not.
func (e *Engine) clusterUID(ctx context.Context) string {
	if e.DryRun {
		return ""
	}
	out, err := e.runner.Output(ctx, e.kubectl("get", "namespace", "kube-system", "-o", "jsonpath={.metadata.uid}"))
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (e *Engine) recordedImagesCluster() string {
	st, _ := e.loadState()
	return st.ImagesCluster
}

func (e *Engine) imageID(ctx context.Context, ref string) string {
	if e.DryRun {
		return ""
	}
	out, err := e.runner.Output(ctx, Cmd{Name: "docker", Args: []string{"image", "inspect", "--format", "{{.Id}}", ref}})
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

// restartFor restarts every workload running one of refs. A rebuilt image
// loaded under the tag a Deployment already names changes nothing in the pod
// spec, so neither helm nor Kubernetes would roll it on their own.
func (e *Engine) restartFor(ctx context.Context, refs []string) error {
	if len(refs) == 0 {
		return nil
	}
	want := map[string]bool{}
	for _, r := range refs {
		want[r] = true
	}
	out, err := e.runner.Output(ctx, e.kubectl("get", "deployments,statefulsets,daemonsets", "-A", "-o",
		`jsonpath={range .items[*]}{.kind}{" "}{.metadata.namespace}{" "}{.metadata.name}{" "}{range .spec.template.spec.initContainers[*]}{.image}{","}{end}{range .spec.template.spec.containers[*]}{.image}{","}{end}{"\n"}{end}`))
	if err != nil {
		return fmt.Errorf("find workloads to restart: %w", err)
	}
	for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
		f := strings.Fields(line)
		if len(f) < 4 {
			continue
		}
		for img := range strings.SplitSeq(f[3], ",") {
			if !want[img] {
				continue
			}
			e.step("Restarting %s %s/%s for its rebuilt image %s", strings.ToLower(f[0]), f[1], f[2], img)
			if err := e.run(ctx, e.kubectl("rollout", "restart", "-n", f[1], strings.ToLower(f[0])+"/"+f[2])); err != nil {
				return err
			}
			break
		}
	}
	return nil
}

func (e *Engine) labelNodes(ctx context.Context) error {
	labels := e.stack.Cluster.NodeLabels
	if len(labels) == 0 {
		return nil
	}
	args := []string{"label", "nodes", "--all", "--overwrite"}
	for _, k := range sortedKeys(labels) {
		args = append(args, k+"="+labels[k])
	}
	e.step("Labelling nodes")
	return e.run(ctx, e.kubectl(args...))
}

func (e *Engine) applySecrets(ctx context.Context) error {
	if len(e.stack.Secrets) == 0 {
		return nil
	}
	store, err := openSecretStore(e.dir)
	if err != nil {
		return err
	}
	e.step("Applying %d secret(s)", len(e.stack.Secrets))
	for _, sec := range e.stack.Secrets {
		data, err := store.resolve(sec)
		if err != nil {
			return err
		}
		e.info("%s/%s", sec.Namespace, sec.Name)
		if nerr := e.ensureNamespace(ctx, sec.Namespace); nerr != nil {
			return nerr
		}
		manifest, err := secretManifest(sec, data)
		if err != nil {
			return err
		}
		if err := e.run(ctx, withStdin(e.kubectl("apply", "-f", "-"), manifest)); err != nil {
			return fmt.Errorf("secret %s/%s: %w", sec.Namespace, sec.Name, err)
		}
	}
	if e.DryRun {
		return nil
	}
	// Saved only after every Secret applied, so a failed run never records a
	// value that the cluster does not hold.
	return store.save()
}

func (e *Engine) ensureNamespace(ctx context.Context, ns string) error {
	manifest := []byte("apiVersion: v1\nkind: Namespace\nmetadata:\n  name: " + ns + "\n")
	return e.run(ctx, withStdin(e.kubectl("apply", "-f", "-"), manifest))
}

func withStdin(c Cmd, b []byte) Cmd {
	c.Stdin = bytes.NewReader(b)
	return c
}

func (e *Engine) installComponent(ctx context.Context, c Component) error {
	e.step("Installing %s", c.Name)

	for _, m := range c.Manifests {
		for _, w := range m.WaitFor {
			if err := e.wait(ctx, w); err != nil {
				return err
			}
		}
		if err := e.applyManifest(ctx, c, m); err != nil {
			return err
		}
	}

	if c.Helm != nil {
		if err := e.helmInstall(ctx, c); err != nil {
			return err
		}
	}

	if c.Ansible != nil {
		if err := e.runAnsible(ctx, c); err != nil {
			return err
		}
	}

	for _, w := range c.Wait {
		if err := e.wait(ctx, w); err != nil {
			return err
		}
	}
	return nil
}

func (e *Engine) applyManifest(ctx context.Context, c Component, m Manifest) error {
	args := []string{"apply"}
	if m.ServerSide {
		args = append(args, "--server-side", "--force-conflicts")
	}
	if c.Namespace != "" {
		if err := e.ensureNamespace(ctx, c.Namespace); err != nil {
			return err
		}
		args = append(args, "-n", c.Namespace)
	}
	switch {
	case m.URL != "":
		e.info("apply %s", m.URL)
		return e.run(ctx, e.kubectl(append(args, "-f", m.URL)...))
	case m.Path != "":
		e.info("apply %s", m.Path)
		return e.run(ctx, e.kubectl(append(args, "-f", m.Path)...))
	default:
		e.info("apply inline manifest")
		return e.run(ctx, withStdin(e.kubectl(append(args, "-f", "-")...), []byte(m.Inline)))
	}
}

// helmInstall runs `helm upgrade --install`. Values from the stack are written
// to a file in the state directory and passed last, so they win over the
// chart's own values files exactly as a final -f does.
func (e *Engine) helmInstall(ctx context.Context, c Component) error {
	h := c.Helm
	chart := h.Chart

	if isLocalChart(h) {
		staged, err := e.stageChart(ctx, c.Name, chart)
		if err != nil {
			return err
		}
		chart = staged
	}

	if err := e.clearStuckRelease(ctx, c); err != nil {
		return err
	}

	args := []string{"upgrade", "--install", c.Name, chart, "--namespace", c.Namespace, "--create-namespace"}
	if h.Repo != "" {
		args = append(args, "--repo", h.Repo)
	}
	if h.Version != "" {
		args = append(args, "--version", h.Version)
	}
	for _, f := range h.ValuesFiles {
		args = append(args, "-f", f)
	}
	if len(c.Values) > 0 {
		values, err := e.expandSecretRefs(c.Values)
		if err != nil {
			return err
		}
		b, err := yaml.Marshal(values)
		if err != nil {
			return fmt.Errorf("render values: %w", err)
		}
		vf := filepath.Join(e.dir, "values", c.Name+".yaml")
		if err := e.writeFile(vf, b); err != nil {
			return err
		}
		args = append(args, "-f", vf)
	}
	if !h.NoWait {
		timeout := h.Timeout
		if timeout == "" {
			timeout = "10m"
		}
		args = append(args, "--wait", "--timeout", timeout)
	}
	return e.run(ctx, e.helm(args...))
}

// clearStuckRelease recovers a release an interrupted run left mid-operation.
// Helm refuses every later upgrade with "another operation is in progress"
// until it is cleared, which would turn one Ctrl-C into a stack that can never
// converge again. A release that never finished installing has nothing worth
// keeping and is uninstalled; one stuck mid-upgrade is rolled back to its last
// good revision.
func (e *Engine) clearStuckRelease(ctx context.Context, c Component) error {
	if e.DryRun {
		return nil
	}
	out, err := e.runner.Output(ctx, e.helm("status", c.Name, "--namespace", c.Namespace, "-o", "json"))
	if err != nil {
		return nil //nolint:nilerr // no release yet is the common case, and upgrade --install handles it
	}
	var st struct {
		Info struct {
			Status string `json:"status"`
		} `json:"info"`
	}
	if json.Unmarshal(out, &st) != nil {
		return nil //nolint:nilerr // an unreadable status names no stuck release; the upgrade reports any real fault
	}
	switch st.Info.Status {
	case "pending-install":
		e.info("release %s never finished installing; removing it to start clean", c.Name)
		return e.run(ctx, e.helm("uninstall", c.Name, "--namespace", c.Namespace, "--wait"))
	case "pending-upgrade", "pending-rollback":
		e.info("release %s was interrupted mid-%s; rolling back to its last good revision",
			c.Name, strings.TrimPrefix(st.Info.Status, "pending-"))
		return e.run(ctx, e.helm("rollback", c.Name, "--namespace", c.Namespace, "--wait"))
	}
	return nil
}

// expandSecretRefs replaces ${secret:ns/name/key} in Helm values with the
// value the stack's Secret holds. It serves charts that take a credential as
// a plain value rather than a Secret reference; the rendered values file is
// written 0600 into the state directory, never into the source tree or argv.
func (e *Engine) expandSecretRefs(values map[string]any) (map[string]any, error) {
	store, err := openSecretStore(e.dir)
	if err != nil {
		return nil, err
	}
	known := map[string]bool{}
	for _, sec := range e.stack.Secrets {
		for k := range sec.Data {
			known[secretKey(sec, k)] = true
		}
	}
	out := deepCopyMap(values)
	var errs []error
	walkStrings(reflect.ValueOf(out), func(s string) string {
		return varRE.ReplaceAllStringFunc(s, func(m string) string {
			ref, ok := strings.CutPrefix(m[2:len(m)-1], "secret:")
			if !ok {
				return m
			}
			if !known[ref] {
				errs = append(errs, fmt.Errorf("values refer to ${secret:%s}, which no secret in the stack defines", ref))
				return m
			}
			val, ok := store.values[ref]
			if !ok {
				val = e.literalSecret(ref)
			}
			if e.DryRun && val == "" {
				return "<generated>"
			}
			return val
		})
	})
	return out, errors.Join(errs...)
}

// literalSecret returns a non-generated secret value (value or fromEnv).
func (e *Engine) literalSecret(ref string) string {
	for _, sec := range e.stack.Secrets {
		for k, v := range sec.Data {
			if secretKey(sec, k) != ref {
				continue
			}
			if v.FromEnv != "" {
				return os.Getenv(v.FromEnv)
			}
			return v.Value
		}
	}
	return ""
}

// stageChart returns the chart path helm should install. A chart without
// dependencies installs straight from the source tree. A chart with
// dependencies is copied into the state directory first, because
// `helm dependency build` writes charts/ and Chart.lock next to Chart.yaml and
// the source tree belongs to the app, not to Mozza.
func (e *Engine) stageChart(ctx context.Context, name, chart string) (string, error) {
	meta, err := os.ReadFile(filepath.Join(chart, "Chart.yaml")) //nolint:gosec // a path from the stack definition
	if err != nil {
		if e.DryRun {
			return chart, nil
		}
		return "", fmt.Errorf("chart %s: %w", chart, err)
	}
	var cm struct {
		Dependencies []any `yaml:"dependencies"`
	}
	if err := yaml.Unmarshal(meta, &cm); err != nil {
		return "", fmt.Errorf("chart %s: Chart.yaml: %w", chart, err)
	}
	if len(cm.Dependencies) == 0 {
		return chart, nil
	}

	staged := filepath.Join(e.dir, "charts", name)
	if !e.DryRun {
		if err := os.RemoveAll(staged); err != nil {
			return "", fmt.Errorf("stage chart: %w", err)
		}
		if err := os.CopyFS(staged, os.DirFS(chart)); err != nil {
			return "", fmt.Errorf("stage chart %s: %w", chart, err)
		}
	}
	e.info("building chart dependencies in %s", staged)
	if err := e.run(ctx, Cmd{Name: "helm", Args: []string{"dependency", "build", staged}}); err != nil {
		return "", err
	}
	return staged, nil
}

// wait checks one readiness condition. kubectl wait fails at once when the
// object does not exist yet — a CRD an operator has not registered, a
// Deployment a controller has not created — so the object is first polled
// into existence and only then waited on.
func (e *Engine) wait(ctx context.Context, w Wait) error {
	timeout := 5 * time.Minute
	if w.Timeout != "" {
		d, err := time.ParseDuration(w.Timeout)
		if err != nil {
			return fmt.Errorf("wait %s: timeout: %w", w.Resource, err)
		}
		timeout = d
	}

	ns := []string{}
	if w.Namespace != "" {
		ns = []string{"-n", w.Namespace}
	}
	target := []string{w.Resource}
	if w.Selector != "" {
		target = append(target, "-l", w.Selector)
	}

	e.info("waiting for %s", describeWait(w))
	if e.DryRun {
		_, _ = fmt.Fprintf(e.out, "    $ %s\n", e.kubectl(append(append(append([]string{"get"}, ns...), target...), "-o", "name")...))
	}

	deadline := time.Now().Add(timeout)
	if !e.DryRun {
		for {
			out, err := e.runner.Output(ctx, e.kubectl(append(append(append([]string{"get"}, ns...), target...), "-o", "name")...))
			if err == nil && len(bytes.TrimSpace(out)) > 0 {
				break
			}
			if time.Now().After(deadline) {
				return fmt.Errorf("%s did not appear within %s", describeWait(w), timeout)
			}
			if err := sleep(ctx, 3*time.Second); err != nil {
				return err
			}
		}
	}

	remaining := max(time.Until(deadline), 10*time.Second).Round(time.Second).String()
	switch {
	case w.Rollout:
		return e.run(ctx, e.kubectl(append(append(append([]string{"rollout", "status"}, ns...), target...), "--timeout", remaining)...))
	case w.For != "":
		return e.run(ctx, e.kubectl(append(append(append([]string{"wait"}, ns...), target...), "--for", w.For, "--timeout", remaining)...))
	default:
		return nil
	}
}

func describeWait(w Wait) string {
	d := w.Resource
	if w.Selector != "" {
		d += " -l " + w.Selector
	}
	if w.Namespace != "" {
		d = w.Namespace + "/" + d
	}
	switch {
	case w.Rollout:
		d += " to roll out"
	case w.For != "":
		d += " (" + w.For + ")"
	default:
		d += " to exist"
	}
	return d
}

// Summary prints how to reach the stack: endpoints, credentials marked for
// display, the kubeconfig, and the stack's notes.
func (e *Engine) Summary(_ context.Context) error {
	s := e.stack
	_, _ = fmt.Fprintln(e.out)
	if len(s.Access) > 0 {
		_, _ = fmt.Fprintf(e.out, "Open it (forwards to 127.0.0.1 until Ctrl-C):\n  mozza stack forward %s\n\n", s.Name)
		for _, a := range s.Access {
			_, _ = fmt.Fprintf(e.out, "  %-12s %s\n", a.Name, accessURL(a))
		}
		_, _ = fmt.Fprintln(e.out)
	}

	store, err := openSecretStore(e.dir)
	if err != nil {
		return err
	}
	if shown := store.shown(s.Secrets); len(shown) > 0 {
		_, _ = fmt.Fprintln(e.out, "Credentials:")
		for _, sh := range shown {
			_, _ = fmt.Fprintf(e.out, "  %s = %s\n", sh.Ref, sh.Value)
		}
		_, _ = fmt.Fprintln(e.out)
	}
	for _, n := range s.Notes {
		_, _ = fmt.Fprintf(e.out, "  %s\n", n)
	}
	if len(s.Notes) > 0 {
		_, _ = fmt.Fprintln(e.out)
	}
	_, _ = fmt.Fprintf(e.out, "kubectl:\n  export KUBECONFIG=%s\n", e.kubeconfigPath())
	return nil
}

func accessURL(a Access) string {
	return "http://127.0.0.1:" + strconv.Itoa(localPort(a)) + a.Path
}

func localPort(a Access) int {
	if a.LocalPort != 0 {
		return a.LocalPort
	}
	return a.Port
}

// Secrets returns the values a stack marks for display.
func (e *Engine) Secrets() ([]ShownSecret, error) {
	store, err := openSecretStore(e.dir)
	if err != nil {
		return nil, err
	}
	return store.shown(e.stack.Secrets), nil
}

// Down deletes the stack's cluster — only if Mozza created it — and, with
// purge, the stack's state including generated secrets.
func (e *Engine) Down(ctx context.Context, purge bool) error {
	p, err := newProvider(e)
	if err != nil {
		return err
	}
	st, known := e.loadState()
	exists, err := p.exists(ctx)
	if err != nil {
		return err
	}

	switch {
	case !exists:
		e.step("Cluster %s does not exist", e.stack.ClusterName())
	case !known || !st.CreatedCluster:
		return fmt.Errorf("cluster %s was not created by `mozza stack up %s` (it existed already, or another stack made it); "+
			"take it down with that stack, or delete it with %s yourself", e.stack.ClusterName(), e.stack.Name, p.binary())
	default:
		e.step("Deleting cluster %s", e.stack.ClusterName())
		if err := p.delete(ctx); err != nil {
			return err
		}
	}

	if purge {
		e.step("Removing state in %s (generated secrets included)", e.dir)
		if !e.DryRun {
			if err := os.RemoveAll(e.dir); err != nil {
				return fmt.Errorf("purge: %w", err)
			}
		}
		return nil
	}
	if known {
		st.CreatedCluster = false
		return e.saveState(st)
	}
	return nil
}

// Status prints the cluster's nodes, releases and any pod that is not healthy.
func (e *Engine) Status(ctx context.Context) error {
	p, err := newProvider(e)
	if err != nil {
		return err
	}
	exists, err := p.exists(ctx)
	if err != nil {
		return err
	}
	if !exists {
		e.step("Cluster %s does not exist — run `mozza stack up %s`", e.stack.ClusterName(), e.stack.Name)
		return nil
	}
	if err := p.kubeconfig(ctx, e.kubeconfigPath()); err != nil {
		return err
	}
	e.step("Nodes")
	_ = e.runner.Run(ctx, e.kubectl("get", "nodes", "-o", "wide"), e.out)
	e.step("Releases")
	_ = e.runner.Run(ctx, e.helm("list", "-A"), e.out)
	e.step("Pods")
	_ = e.runner.Run(ctx, e.kubectl("get", "pods", "-A"), e.out)
	return nil
}

// Forward port-forwards every access entry to 127.0.0.1 and keeps the
// forwards alive until ctx ends. kubectl port-forward exits when the pod
// behind it restarts, so each forward is restarted rather than left dead.
func (e *Engine) Forward(ctx context.Context) error {
	s := e.stack
	if len(s.Access) == 0 {
		return fmt.Errorf("stack %s lists nothing to access", s.Name)
	}
	if _, ok := readFileIfExists(e.kubeconfigPath()); !ok {
		return fmt.Errorf("no kubeconfig for %s — run `mozza stack up %s` first", s.Name, s.Name)
	}

	var wg sync.WaitGroup
	for _, a := range s.Access {
		wg.Add(1)
		go func(a Access) {
			defer wg.Done()
			e.keepForwarding(ctx, a)
		}(a)
	}
	for _, a := range s.Access {
		if a.Health == "" {
			continue
		}
		go e.reportHealth(ctx, a)
	}

	_, _ = fmt.Fprintf(e.out, "Forwarding (Ctrl-C to stop):\n")
	for _, a := range s.Access {
		_, _ = fmt.Fprintf(e.out, "  %-12s %s\n", a.Name, accessURL(a))
	}
	wg.Wait()
	return nil
}

func (e *Engine) keepForwarding(ctx context.Context, a Access) {
	pf := e.kubectl("port-forward", "--address", "127.0.0.1", "-n", a.Namespace,
		"svc/"+a.Service, fmt.Sprintf("%d:%d", localPort(a), a.Port))
	for ctx.Err() == nil {
		// Forward chatter ("Handling connection for ...") is noise here.
		err := e.runner.Run(ctx, pf, io.Discard)
		if ctx.Err() != nil {
			return
		}
		_, _ = fmt.Fprintf(e.out, "    %s: forward ended (%v), retrying\n", a.Name, err)
		if sleep(ctx, 2*time.Second) != nil {
			return
		}
	}
}

func (e *Engine) reportHealth(ctx context.Context, a Access) {
	url := "http://127.0.0.1:" + strconv.Itoa(localPort(a)) + a.Health
	client := &http.Client{Timeout: 3 * time.Second}
	deadline := time.Now().Add(2 * time.Minute)
	for ctx.Err() == nil && time.Now().Before(deadline) {
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		resp, err := client.Do(req)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode < 500 {
				_, _ = fmt.Fprintf(e.out, "    %s: %s answered %d\n", a.Name, url, resp.StatusCode)
				return
			}
		}
		if sleep(ctx, 2*time.Second) != nil {
			return
		}
	}
	if ctx.Err() == nil {
		_, _ = fmt.Fprintf(e.out, "    %s: %s did not answer within 2m\n", a.Name, url)
	}
}

func sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
