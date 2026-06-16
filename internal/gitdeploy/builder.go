package gitdeploy

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/gshepptech/mozza/internal/detect"
	"github.com/gshepptech/mozza/internal/store"
)

// buildTimeout is the maximum duration for a single build.
const buildTimeout = 10 * time.Minute

// Builder clones repositories, detects frameworks, and builds Docker images.
type Builder struct {
	store *store.Store
}

// NewBuilder creates a new builder.
func NewBuilder(s *store.Store) *Builder {
	return &Builder{store: s}
}

// Build clones the repo at the given commit, detects the framework,
// builds a Docker image, and updates the build record.
func (b *Builder) Build(ctx context.Context, job BuildJob) error {
	ctx, cancel := context.WithTimeout(ctx, buildTimeout)
	defer cancel()

	start := time.Now()

	// Update build status to building.
	if err := b.store.UpdateBuild(ctx, job.BuildID, "building", "", 0, ""); err != nil {
		return fmt.Errorf("Build: update status to building: %w", err)
	}

	var logBuf bytes.Buffer

	// Create temp directory for the clone.
	cloneDir, err := os.MkdirTemp("", "mozza-build-*")
	if err != nil {
		return b.failBuild(ctx, job.BuildID, start, "failed to create temp dir", err)
	}
	defer os.RemoveAll(cloneDir)

	// Clone the repository.
	fmt.Fprintf(&logBuf, "==> Cloning %s at %s\n", job.RepoURL, job.CommitSHA[:minInt(7, len(job.CommitSHA))])
	if err := b.cloneRepo(ctx, job.RepoURL, job.CommitSHA, cloneDir, &logBuf); err != nil {
		return b.failBuildWithLog(ctx, job.BuildID, start, logBuf.String(), err)
	}

	// Detect framework.
	result := detectFramework(cloneDir, job.BuildID, &logBuf)

	// Determine the image tag.
	appName := extractAppName(job.RepoURL)
	shortSHA := job.CommitSHA[:minInt(7, len(job.CommitSHA))]
	imageTag := fmt.Sprintf("mozza-%s:%s", appName, shortSHA)

	// Ensure a Dockerfile exists in the clone directory.
	if err := ensureDockerfile(cloneDir, result, &logBuf); err != nil {
		return b.failBuildWithLog(ctx, job.BuildID, start, logBuf.String(), err)
	}

	// Build Docker image.
	fmt.Fprintf(&logBuf, "==> Building image %s\n", imageTag)
	if err := b.buildImage(ctx, cloneDir, imageTag, &logBuf); err != nil {
		return b.failBuildWithLog(ctx, job.BuildID, start, logBuf.String(), err)
	}

	duration := time.Since(start)
	fmt.Fprintf(&logBuf, "==> Build complete in %s\n", duration.Round(time.Millisecond))

	// Update build record with success.
	if err := b.store.UpdateBuild(ctx, job.BuildID, "success", logBuf.String(), duration.Milliseconds(), imageTag); err != nil {
		return fmt.Errorf("Build: update success: %w", err)
	}

	slog.Info("build completed",
		"build_id", job.BuildID,
		"image", imageTag,
		"duration", duration,
	)

	return nil
}

// cloneRepo performs a shallow clone of the repository at a specific commit.
func (b *Builder) cloneRepo(ctx context.Context, repoURL, commitSHA, dir string, logBuf *bytes.Buffer) error {
	// Clone with depth 1 to the target directory.
	cloneCmd := exec.CommandContext(ctx, "git", "clone", "--depth", "1", repoURL, dir)
	cloneCmd.Dir = os.TempDir()
	output, err := cloneCmd.CombinedOutput()
	fmt.Fprintf(logBuf, "%s", output)
	if err != nil {
		return fmt.Errorf("git clone: %w", err)
	}

	// Fetch the specific commit if it differs from HEAD.
	fetchCmd := exec.CommandContext(ctx, "git", "fetch", "origin", commitSHA)
	fetchCmd.Dir = dir
	output, err = fetchCmd.CombinedOutput()
	fmt.Fprintf(logBuf, "%s", output)
	if err != nil {
		// If fetch fails (shallow clone limitation), proceed with HEAD.
		slog.Debug("build: fetch specific commit failed, using HEAD",
			"commit", commitSHA,
			"error", err,
		)
		return nil
	}

	// Checkout the specific commit.
	checkoutCmd := exec.CommandContext(ctx, "git", "checkout", commitSHA)
	checkoutCmd.Dir = dir
	output, err = checkoutCmd.CombinedOutput()
	fmt.Fprintf(logBuf, "%s", output)
	if err != nil {
		slog.Debug("build: checkout specific commit failed, using HEAD",
			"commit", commitSHA,
			"error", err,
		)
	}

	return nil
}

// buildImage runs docker build in the given directory.
func (b *Builder) buildImage(ctx context.Context, dir, imageTag string, logBuf *bytes.Buffer) error {
	cmd := exec.CommandContext(ctx, "docker", "build", "-t", imageTag, ".")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	fmt.Fprintf(logBuf, "%s", output)
	if err != nil {
		return fmt.Errorf("docker build: %w", err)
	}
	return nil
}

// failBuild updates the build record with a failure status.
func (b *Builder) failBuild(ctx context.Context, buildID int64, start time.Time, msg string, err error) error {
	duration := time.Since(start)
	logs := fmt.Sprintf("Error: %s: %v", msg, err)
	if updateErr := b.store.UpdateBuild(ctx, buildID, "failed", logs, duration.Milliseconds(), ""); updateErr != nil {
		slog.Error("build: failed to update build status", "error", updateErr, "build_id", buildID)
	}
	return fmt.Errorf("Build: %s: %w", msg, err)
}

// failBuildWithLog updates the build record with a failure status and accumulated logs.
func (b *Builder) failBuildWithLog(ctx context.Context, buildID int64, start time.Time, logs string, err error) error {
	duration := time.Since(start)
	fullLogs := fmt.Sprintf("%s\nError: %v", logs, err)
	if updateErr := b.store.UpdateBuild(ctx, buildID, "failed", fullLogs, duration.Milliseconds(), ""); updateErr != nil {
		slog.Error("build: failed to update build status", "error", updateErr, "build_id", buildID)
	}
	return fmt.Errorf("Build: %w", err)
}

// extractAppName derives an app name from a repo URL.
// e.g., "https://github.com/user/my-app" -> "my-app"
func extractAppName(repoURL string) string {
	repoURL = normalizeRepoURL(repoURL)
	parts := strings.Split(repoURL, "/")
	if len(parts) == 0 {
		return "app"
	}
	name := parts[len(parts)-1]
	if name == "" {
		return "app"
	}
	// Sanitize: lowercase, replace non-alphanumeric with dashes.
	name = strings.ToLower(name)
	var sb strings.Builder
	for _, c := range name {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-' {
			sb.WriteRune(c)
		} else {
			sb.WriteRune('-')
		}
	}
	result := strings.Trim(sb.String(), "-")
	if result == "" {
		return "app"
	}
	return result
}

// detectFramework scans the clone directory for a framework and logs the result.
func detectFramework(cloneDir string, buildID int64, logBuf *bytes.Buffer) *detect.Result {
	fmt.Fprintf(logBuf, "==> Detecting framework\n")
	result, err := detect.ScanBest(cloneDir)
	if err != nil {
		fmt.Fprintf(logBuf, "    Detection error: %v\n", err)
		slog.Warn("build: framework detection failed", "error", err, "build_id", buildID)
	}
	if result != nil {
		fmt.Fprintf(logBuf, "    Detected: %s (%s)\n", result.Framework, result.Confidence)
	} else {
		fmt.Fprintf(logBuf, "    No framework detected, using generic Dockerfile\n")
	}
	return result
}

// ensureDockerfile writes a Dockerfile into cloneDir if one does not already exist.
func ensureDockerfile(cloneDir string, result *detect.Result, logBuf *bytes.Buffer) error {
	dockerfilePath := filepath.Join(cloneDir, "Dockerfile")

	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) && result != nil && result.Dockerfile != "" {
		fmt.Fprintf(logBuf, "==> Writing auto-generated Dockerfile\n")
		if writeErr := os.WriteFile(dockerfilePath, []byte(result.Dockerfile), 0o644); writeErr != nil {
			fmt.Fprintf(logBuf, "    Failed to write Dockerfile: %v\n", writeErr)
		}
	}

	if _, err := os.Stat(dockerfilePath); os.IsNotExist(err) {
		fmt.Fprintf(logBuf, "==> No Dockerfile found, generating minimal one\n")
		dockerfile := generateMinimalDockerfile(result)
		if writeErr := os.WriteFile(dockerfilePath, []byte(dockerfile), 0o644); writeErr != nil {
			return fmt.Errorf("write Dockerfile: %w", writeErr)
		}
	}
	return nil
}

// generateMinimalDockerfile creates a basic Dockerfile when none exists.
func generateMinimalDockerfile(result *detect.Result) string {
	if result == nil {
		return `FROM alpine:3.19
WORKDIR /app
COPY . .
EXPOSE 8080
CMD ["sh"]
`
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("FROM %s\n", result.BaseImage))
	sb.WriteString("WORKDIR /app\n")
	sb.WriteString("COPY . .\n")
	if result.BuildCmd != "" {
		sb.WriteString(fmt.Sprintf("RUN %s\n", result.BuildCmd))
	}
	sb.WriteString(fmt.Sprintf("EXPOSE %d\n", result.Port))
	if result.StartCmd != "" {
		sb.WriteString(fmt.Sprintf("CMD %s\n", result.StartCmd))
	}
	return sb.String()
}
