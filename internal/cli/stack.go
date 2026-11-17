package cli

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/gshepptech/mozza/internal/stack"
)

// newStackCmd creates "mozza stack", which brings up an app that ships its own
// Helm charts, and everything those charts assume, on a local cluster.
func newStackCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "stack",
		Short: "Bring up an app and its infrastructure on a local cluster",
		Long: `A stack is a whole environment on a local kind or k3d cluster: the cluster,
the operators and CNI an app's charts assume, its images built from source,
the Secrets it needs, and the charts themselves.

Stacks are local only. The cluster API, published ports and forwards all bind
127.0.0.1, and nothing runs against a cluster that is not on this machine.

STACK is a catalog name (see "mozza stack list") or a path to a stack file.
Without one, ./mozza-stack.yaml is used — the file "mozza stack init" writes.
Definitions in ~/.mozza/stacks and ~/.mozza/addons shadow the built-in ones.`,
	}

	cmd.AddCommand(
		newStackInitCmd(),
		newStackListCmd(),
		newStackPlanCmd(),
		newStackUpCmd(),
		newStackStatusCmd(),
		newStackForwardCmd(),
		newStackSecretsCmd(),
		newStackDownCmd(),
	)
	return cmd
}

func addSourceFlag(cmd *cobra.Command) {
	cmd.Flags().String("source", "", "path to the app's source tree (overrides the stack's source.path)")
}

func addProviderFlag(cmd *cobra.Command) {
	cmd.Flags().String("provider", "", "run on kind or k3d instead of the stack's cluster.provider")
}

func addImageFlag(cmd *cobra.Command) {
	cmd.Flags().StringArray("image", nil,
		"use an existing image instead of building one, as REF=IMAGE (repeatable); IMAGE is pulled if docker lacks it")
}

// imageOverrides parses the repeatable --image REF=IMAGE flag.
func imageOverrides(cmd *cobra.Command) (map[string]string, error) {
	vals, _ := cmd.Flags().GetStringArray("image")
	out := map[string]string{}
	for _, v := range vals {
		ref, src, ok := strings.Cut(v, "=")
		if !ok || ref == "" || src == "" {
			return nil, fmt.Errorf("--image %q: want REF=IMAGE", v)
		}
		out[ref] = src
	}
	return out, nil
}

// stackEngine loads, resolves and wraps one stack for a command that builds
// from the source tree.
func stackEngine(cmd *cobra.Command, ref string) (*stack.Engine, error) {
	return loadStackEngine(cmd, ref, false)
}

// runningStackEngine is stackEngine for commands that act on a stack already
// up, which need no source tree.
func runningStackEngine(cmd *cobra.Command, ref string) (*stack.Engine, error) {
	return loadStackEngine(cmd, ref, true)
}

func loadStackEngine(cmd *cobra.Command, ref string, noSource bool) (*stack.Engine, error) {
	cat := stack.DefaultCatalog()
	s, err := cat.Load(ref)
	if err != nil {
		return nil, err
	}
	source := ""
	if f := cmd.Flags().Lookup("source"); f != nil {
		source = f.Value.String()
	}
	provider, _ := cmd.Flags().GetString("provider")
	if provider == "" && noSource {
		provider = stack.RecordedProvider(stack.StateRoot(), s.Name)
	}
	r, err := cat.Resolve(s, stack.ResolveOptions{Source: source, NoSource: noSource, Provider: provider})
	if err != nil {
		return nil, err
	}
	return stack.NewEngine(r, stack.ExecRunner{}, cmd.OutOrStdout(), stack.StateRoot()), nil
}

// defaultStackFile is the stack file a repository keeps at its root, used
// when a command names no stack.
const defaultStackFile = "mozza-stack.yaml"

func stackRef(args []string) string {
	if len(args) > 0 {
		return args[0]
	}
	return defaultStackFile
}

// interruptible returns a context cancelled by Ctrl-C or SIGTERM, so a long
// helm wait or a forward stops the commands it started instead of orphaning them.
func interruptible() (context.Context, context.CancelFunc) {
	return signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
}

func newStackListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the stacks and addons Mozza knows",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			cat := stack.DefaultCatalog()
			stacks, err := cat.Stacks()
			if err != nil {
				return err
			}
			addons, err := cat.Addons()
			if err != nil {
				return err
			}
			cmd.Println("STACKS")
			for _, e := range stacks {
				cmd.Printf("  %-16s %s%s\n", e.Name, e.Description, originNote(e))
			}
			cmd.Println("\nADDONS (use: <name> in a stack's components)")
			for _, e := range addons {
				cmd.Printf("  %-16s %s%s\n", e.Name, e.Description, originNote(e))
			}
			return nil
		},
	}
}

func originNote(e stack.Entry) string {
	if e.Origin == "built-in" {
		return ""
	}
	return " [" + e.Origin + "]"
}

func newStackPlanCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plan [STACK]",
		Short: "Print every command `up` would run, without running any",
		Long: `Print the commands "mozza stack up" runs on a machine where nothing exists
yet. Nothing is built, created or applied.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := stackEngine(cmd, stackRef(args))
			if err != nil {
				return err
			}
			e.DryRun = true
			e.Rebuild = true
			if e.ImageOverrides, err = imageOverrides(cmd); err != nil {
				return err
			}
			return e.Up(cmd.Context())
		},
	}
	addSourceFlag(cmd)
	addImageFlag(cmd)
	addProviderFlag(cmd)
	return cmd
}

func newStackUpCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "up [STACK]",
		Short: "Create the cluster, build and load images, and install everything",
		Long: `Bring a stack up. Every step is idempotent: re-running "up" after a failure,
or after changing the source, converges rather than starting over. Images
already present in docker are reused unless --rebuild is given.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := stackEngine(cmd, stackRef(args))
			if err != nil {
				return err
			}
			e.Rebuild, _ = cmd.Flags().GetBool("rebuild")
			forward, _ := cmd.Flags().GetBool("forward")
			if e.ImageOverrides, err = imageOverrides(cmd); err != nil {
				return err
			}

			ctx, cancel := interruptible()
			defer cancel()
			if err := e.Up(ctx); err != nil {
				return err
			}
			if !forward {
				return nil
			}
			cmd.Println()
			return e.Forward(ctx)
		},
	}
	addSourceFlag(cmd)
	cmd.Flags().Bool("rebuild", false, "build images even if docker already has them")
	addImageFlag(cmd)
	cmd.Flags().Bool("forward", false, "after bringing the stack up, forward its services to 127.0.0.1 until Ctrl-C")
	addProviderFlag(cmd)
	return cmd
}

func newStackStatusCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status [STACK]",
		Short: "Show the stack's nodes, Helm releases and pods",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := runningStackEngine(cmd, stackRef(args))
			if err != nil {
				return err
			}
			return e.Status(cmd.Context())
		},
	}
	addProviderFlag(cmd)
	return cmd
}

func newStackForwardCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "forward [STACK]",
		Short: "Forward the stack's services to 127.0.0.1 until Ctrl-C",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := runningStackEngine(cmd, stackRef(args))
			if err != nil {
				return err
			}
			ctx, cancel := interruptible()
			defer cancel()
			return e.Forward(ctx)
		},
	}
	addProviderFlag(cmd)
	return cmd
}

func newStackSecretsCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "secrets [STACK]",
		Short: "Print the credentials a stack marks for people (admin passwords and the like)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := runningStackEngine(cmd, stackRef(args))
			if err != nil {
				return err
			}
			shown, err := e.Secrets()
			if err != nil {
				return err
			}
			if len(shown) == 0 {
				cmd.Println("This stack marks no credentials for display.")
				return nil
			}
			for _, s := range shown {
				cmd.Printf("%s = %s\n", s.Ref, s.Value)
			}
			return nil
		},
	}
	addProviderFlag(cmd)
	return cmd
}

func newStackDownCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "down [STACK]",
		Short: "Delete the stack's cluster (only one `mozza stack up` created)",
		Long: `Delete the stack's cluster. Mozza deletes only a cluster it created; a cluster
that already existed when "up" first ran is left alone.

Generated secrets are kept, so the next "up" hands the app the same
credentials. --purge removes them along with the rest of the stack's state.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			e, err := runningStackEngine(cmd, stackRef(args))
			if err != nil {
				return err
			}
			purge, _ := cmd.Flags().GetBool("purge")
			ctx, cancel := interruptible()
			defer cancel()
			return e.Down(ctx, purge)
		},
	}
	cmd.Flags().Bool("purge", false, "also delete the stack's state, including generated secrets")
	addProviderFlag(cmd)
	return cmd
}
