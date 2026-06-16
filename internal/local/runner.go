package local

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os/exec"
)

// Runner executes Docker Compose commands to manage local deployments.
type Runner struct {
	// composeBin is the resolved path to the docker compose binary.
	composeBin string
	// composeArgs holds the base arguments (e.g., ["compose"] for "docker compose").
	composeArgs []string
	// dir is the working directory for compose commands.
	dir string
}

// NewRunner creates a Runner that auto-detects the available Docker Compose
// binary. It checks for "docker compose" (v2 plugin) first, then falls back
// to the standalone "docker-compose" binary. Returns an error if neither is
// found.
func NewRunner(dir string) (*Runner, error) {
	bin, args, err := detectComposeBinary()
	if err != nil {
		return nil, fmt.Errorf("NewRunner: %w", err)
	}

	slog.Debug("detected compose binary", "bin", bin, "args", args)

	return &Runner{
		composeBin:  bin,
		composeArgs: args,
		dir:         dir,
	}, nil
}

// Up runs "docker compose up -d" in the runner's working directory, streaming
// stdout and stderr to the provided writer.
func (r *Runner) Up(ctx context.Context, w io.Writer) error {
	args := append(r.composeArgs, "up", "-d")

	slog.Info("starting compose services", "cmd", r.composeBin, "args", args)

	cmd := exec.CommandContext(ctx, r.composeBin, args...)
	cmd.Dir = r.dir
	cmd.Stdout = w
	cmd.Stderr = w

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Up: %w", err)
	}

	return nil
}

// Down runs "docker compose down" in the runner's working directory, streaming
// stdout and stderr to the provided writer.
func (r *Runner) Down(ctx context.Context, w io.Writer) error {
	args := append(r.composeArgs, "down")

	slog.Info("stopping compose services", "cmd", r.composeBin, "args", args)

	cmd := exec.CommandContext(ctx, r.composeBin, args...)
	cmd.Dir = r.dir
	cmd.Stdout = w
	cmd.Stderr = w

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Down: %w", err)
	}

	return nil
}

// Logs runs "docker compose logs" in the runner's working directory, streaming
// stdout and stderr to the provided writer. If tail is greater than zero, the
// --tail flag limits output to the last N lines per service.
func (r *Runner) Logs(ctx context.Context, w io.Writer, tail int) error {
	args := append(r.composeArgs, "logs")
	if tail > 0 {
		args = append(args, "--tail", fmt.Sprintf("%d", tail))
	}

	slog.Debug("fetching compose logs", "cmd", r.composeBin, "args", args)

	cmd := exec.CommandContext(ctx, r.composeBin, args...)
	cmd.Dir = r.dir
	cmd.Stdout = w
	cmd.Stderr = w

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("Logs: %w", err)
	}

	return nil
}

// detectComposeBinary probes for an available Docker Compose binary.
// It prefers "docker compose" (v2 plugin) over standalone "docker-compose".
func detectComposeBinary() (string, []string, error) {
	// Try "docker compose" (v2 plugin) first.
	if dockerPath, err := exec.LookPath("docker"); err == nil {
		if checkComposePlugin(dockerPath) {
			return dockerPath, []string{"compose"}, nil
		}
	}

	// Fall back to standalone "docker-compose".
	if composePath, err := exec.LookPath("docker-compose"); err == nil {
		return composePath, nil, nil
	}

	return "", nil, fmt.Errorf("detectComposeBinary: neither 'docker compose' nor 'docker-compose' found in PATH")
}

// checkComposePlugin verifies that "docker compose version" succeeds,
// confirming the compose v2 plugin is installed.
func checkComposePlugin(dockerPath string) bool {
	cmd := exec.Command(dockerPath, "compose", "version")
	return cmd.Run() == nil
}
