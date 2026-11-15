package stack

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"

	"gopkg.in/yaml.v3"
)

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
