package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/gshepptech/mozza/internal/stackdraft"
)

func newStackInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init [DIR]",
		Short: "Draft a stack file from an app's source tree",
		Long: `Scan DIR (default: the current directory) for the app's Helm chart, its values
overlays, its Dockerfiles, the credentials the chart leaves empty, the
operators its templates need, and any Ansible playbooks, and write a stack file
that deploys it.

What the scan cannot settle is listed as review notes at the top of the file.
The draft always loads; run "mozza stack plan" to see what it will do, then
"mozza stack up".`,
		Args: cobra.MaximumNArgs(1),
		RunE: runStackInit,
	}
	cmd.Flags().String("name", "", "stack name (default: the directory's name)")
	cmd.Flags().StringP("output", "o", "", "file to write (default: DIR/"+defaultStackFile+"; - for stdout)")
	cmd.Flags().Bool("force", false, "overwrite an existing stack file")
	return cmd
}

func runStackInit(cmd *cobra.Command, args []string) error {
	dir := "."
	if len(args) > 0 {
		dir = args[0]
	}
	name, _ := cmd.Flags().GetString("name")
	out, _ := cmd.Flags().GetString("output")
	force, _ := cmd.Flags().GetBool("force")
	if out == "" {
		out = filepath.Join(dir, defaultStackFile)
	}

	outDir := filepath.Dir(out)
	if out == "-" {
		outDir = ""
	}
	res, err := stackdraft.Draft(dir, stackdraft.Options{Name: name, OutputDir: outDir, Render: true})
	if err != nil {
		return err
	}

	if out == "-" {
		_, err := cmd.OutOrStdout().Write(res.YAML)
		return err
	}
	if !force {
		if _, err := os.Stat(out); err == nil {
			return fmt.Errorf("%s already exists; pass --force to replace it, or -o to write elsewhere", out)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return err
		}
	}
	if err := os.WriteFile(out, res.YAML, 0o644); err != nil { //nolint:gosec // a stack file is meant to be committed and shared
		return fmt.Errorf("write %s: %w", out, err)
	}

	cmd.PrintErrf("Wrote %s\n", out)
	if n := len(res.Review); n > 0 {
		cmd.PrintErrf("%d item(s) to review are listed at the top of the file.\n", n)
	}
	cmd.PrintErrf("Next: mozza stack plan %s\n", out)
	return nil
}
