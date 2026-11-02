package stack

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

// Cmd is one external command. Everything a stack does to the machine goes
// through one of these, which is what lets `mozza stack plan` print the whole
// run and lets tests assert on it without a cluster.
type Cmd struct {
	Name string
	Args []string
	// Stdin feeds the command. Secret material travels here, never in Args.
	Stdin io.Reader
	Dir   string
	// Env is added to the inherited environment.
	Env []string
}

func (c Cmd) String() string {
	parts := append([]string{c.Name}, c.Args...)
	for i, p := range parts {
		if p == "" || strings.ContainsAny(p, " \t\"'$*{}[]") {
			parts[i] = fmt.Sprintf("%q", p)
		}
	}
	return strings.Join(parts, " ")
}

// Runner executes commands.
type Runner interface {
	// Run executes cmd, streaming its stdout and stderr to out.
	Run(ctx context.Context, cmd Cmd, out io.Writer) error
	// Output executes cmd and returns its stdout. Stderr is folded into the
	// error so a failed query explains itself.
	Output(ctx context.Context, cmd Cmd) ([]byte, error)
}

// ExecRunner runs commands on this machine.
type ExecRunner struct{}

// Run implements Runner.
func (ExecRunner) Run(ctx context.Context, c Cmd, out io.Writer) error {
	cmd := exec.CommandContext(ctx, c.Name, c.Args...) //nolint:gosec // the engine builds every argv itself
	cmd.Dir = c.Dir
	cmd.Stdin = c.Stdin
	cmd.Env = withEnv(c.Env)
	cmd.Stdout = out
	cmd.Stderr = out
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%s: %w", c.Name+" "+firstArg(c.Args), err)
	}
	return nil
}

// Output implements Runner.
func (ExecRunner) Output(ctx context.Context, c Cmd) ([]byte, error) {
	cmd := exec.CommandContext(ctx, c.Name, c.Args...) //nolint:gosec // the engine builds every argv itself
	cmd.Dir = c.Dir
	cmd.Stdin = c.Stdin
	cmd.Env = withEnv(c.Env)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	b, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return b, fmt.Errorf("%s %s: %w: %s", c.Name, firstArg(c.Args), err, msg)
		}
		return b, fmt.Errorf("%s %s: %w", c.Name, firstArg(c.Args), err)
	}
	return b, nil
}

// withEnv returns nil (inherit) when there is nothing to add.
func withEnv(extra []string) []string {
	if len(extra) == 0 {
		return nil
	}
	return append(os.Environ(), extra...)
}

func firstArg(args []string) string {
	if len(args) == 0 {
		return ""
	}
	return args[0]
}
