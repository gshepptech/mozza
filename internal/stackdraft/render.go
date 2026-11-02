package stackdraft

import (
	"fmt"
	"maps"
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/gshepptech/mozza/internal/stack"
)

// render writes the draft as YAML, with the review list as the file's
// header comment and the reasoning for each section beside it.
func (d *drafter) render() ([]byte, error) {
	replaceInMap(d.values, "@NS@", d.namespace)

	root := &yaml.Node{Kind: yaml.MappingNode}
	add := func(key string, val *yaml.Node, comment string) {
		k := scalar(key)
		if comment != "" {
			k.HeadComment = comment
		}
		root.Content = append(root.Content, k, val)
	}

	root.HeadComment = d.header()

	add("name", scalar(d.name), "")
	add("description", scalar(fmt.Sprintf("%s, drafted by mozza stack init", d.chart.name)), "")
	add("source", mapping("path", scalar(d.sourcePath())), "")
	add("cluster", mapping("provider", scalar(stack.ProviderKind)), "")

	if len(d.images) > 0 {
		n, err := encode(d.images)
		if err != nil {
			return nil, err
		}
		add("images", n, "Built from the tree (or pulled) and loaded into the cluster's nodes.")
	}

	if len(d.secrets) > 0 {
		n, err := encode([]stack.Secret{{Name: d.secretName(), Namespace: d.namespace, Data: maps.Clone(d.secrets)}})
		if err != nil {
			return nil, err
		}
		add("secrets", n, "Empty credential values in the chart, generated once and reused on every up.")
	}

	comps := &yaml.Node{Kind: yaml.SequenceNode}
	for _, a := range d.addons {
		comps.Content = append(comps.Content, mapping("use", scalar(a)))
	}
	app, err := d.appComponent()
	if err != nil {
		return nil, err
	}
	comps.Content = append(comps.Content, app)
	add("components", comps, "")

	if d.access != nil {
		n, err := encode([]stack.Access{*d.access})
		if err != nil {
			return nil, err
		}
		add("access", n, "Forwarded to 127.0.0.1 by `mozza stack forward`.")
	}

	// At the end of the file: a foot comment inside a sequence lands
	// wherever the encoder next breaks a line, which is not under it.
	root.FootComment = d.playbookFoot()

	doc := &yaml.Node{Kind: yaml.DocumentNode, Content: []*yaml.Node{root}}
	var sb strings.Builder
	enc := yaml.NewEncoder(&sb)
	enc.SetIndent(2)
	if err := enc.Encode(doc); err != nil {
		return nil, fmt.Errorf("render draft: %w", err)
	}
	return []byte(sb.String()), nil
}

func (d *drafter) appComponent() (*yaml.Node, error) {
	helm := []any{"chart", scalar(d.chart.dir)}
	if len(d.overlays) > 0 {
		files := &yaml.Node{Kind: yaml.SequenceNode}
		for _, o := range d.overlays {
			files.Content = append(files.Content, scalar(o.path))
		}
		helm = append(helm, "valuesFiles", files)
	}
	pairs := []any{
		"name", scalar(d.name),
		"namespace", scalar(d.namespace),
		"helm", mapping(helm...),
	}
	if len(d.values) > 0 {
		v, err := encode(d.values)
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, "values", v)
	}
	return mapping(pairs...), nil
}

// playbookFoot lists the playbooks found as a commented-out component:
// running a playbook is never a default, but mapping its groups is the
// first thing a person does to opt in.
func (d *drafter) playbookFoot() string {
	if len(d.t.playbooks) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString("Playbooks found in the tree. To run one after the chart, move it under\n")
	b.WriteString("components and map its groups onto local targets (nodes, control_plane,\n")
	b.WriteString("workers, runner):\n")
	shown := d.t.playbooks
	if len(shown) > 5 {
		shown = shown[:5]
	}
	for _, p := range shown {
		dir := filepath.Dir(filepath.Dir(p.path))
		if filepath.Base(filepath.Dir(p.path)) != "playbooks" {
			dir = filepath.Dir(p.path)
		}
		fmt.Fprintf(&b, "\n  - name: %s\n    ansible:\n      playbook: %s\n", slug(strings.TrimSuffix(filepath.Base(p.path), filepath.Ext(p.path))), p.path)
		if dir != "." {
			fmt.Fprintf(&b, "      dir: %s\n", dir)
		}
		var groups []string
		for _, h := range p.hosts {
			if t := suggestTarget(h); t != "" {
				groups = append(groups, fmt.Sprintf("%s: %s", quoteKey(h), t))
			}
		}
		if len(groups) > 0 {
			fmt.Fprintf(&b, "      groups: {%s}\n", strings.Join(groups, ", "))
		}
	}
	if len(d.t.playbooks) > len(shown) {
		fmt.Fprintf(&b, "\n  (%d more playbooks not listed)\n", len(d.t.playbooks)-len(shown))
	}
	return b.String()
}

// suggestTarget maps a playbook host pattern onto a local target by name.
func suggestTarget(host string) string {
	h := strings.ToLower(host)
	switch {
	case h == "all":
		return ""
	case h == "localhost" || h == "127.0.0.1" || strings.Contains(h, "jump") || strings.Contains(h, "bastion"):
		return "runner"
	case strings.Contains(h, "server") || strings.Contains(h, "master") || strings.Contains(h, "control"):
		return "control_plane"
	case strings.Contains(h, "worker") || strings.Contains(h, "agent"):
		return "workers"
	default:
		return "nodes"
	}
}

func quoteKey(s string) string {
	if strings.ContainsAny(s, " :{}[],#&*!|>'\"%@`") {
		return fmt.Sprintf("%q", s)
	}
	return s
}

func (d *drafter) header() string {
	var b strings.Builder
	fmt.Fprintf(&b, "Drafted by `mozza stack init` from %s.\n", d.t.root)
	b.WriteString("Run `mozza stack plan` to see every command, then `mozza stack up`.\n")
	b.WriteString("Reference: docs/stacks.md in the mozza repository.")
	if len(d.review) > 0 {
		b.WriteString("\n\nReview before the first up:")
		for _, r := range d.review {
			b.WriteString("\n  - " + wrap(r, 90, "    "))
		}
	}
	return b.String()
}

// sourcePath is "." when the file lives at the root of the tree it
// describes, otherwise the tree's absolute path with ~ for the home dir.
func (d *drafter) sourcePath() string {
	if d.opts.OutputDir != "" {
		if out, err := filepath.Abs(d.opts.OutputDir); err == nil && out == d.t.root {
			return "."
		}
	}
	if home, err := os.UserHomeDir(); err == nil {
		if rel, err := filepath.Rel(home, d.t.root); err == nil && !strings.HasPrefix(rel, "..") {
			return "~/" + filepath.ToSlash(rel)
		}
	}
	return d.t.root
}

func scalar(s string) *yaml.Node {
	return &yaml.Node{Kind: yaml.ScalarNode, Value: s}
}

// mapping builds a mapping node from alternating string keys and nodes.
func mapping(pairs ...any) *yaml.Node {
	n := &yaml.Node{Kind: yaml.MappingNode}
	for i := 0; i+1 < len(pairs); i += 2 {
		key, _ := pairs[i].(string)
		val, _ := pairs[i+1].(*yaml.Node)
		n.Content = append(n.Content, scalar(key), val)
	}
	return n
}

// encode turns a value into a node, dropping zero-valued fields so the
// draft reads like a hand-written stack rather than a struct dump.
func encode(v any) (*yaml.Node, error) {
	var n yaml.Node
	if err := n.Encode(v); err != nil {
		return nil, fmt.Errorf("render draft: %w", err)
	}
	prune(&n)
	return &n, nil
}

func prune(n *yaml.Node) {
	switch n.Kind { //nolint:exhaustive // scalars and aliases have nothing to prune
	case yaml.MappingNode:
		var kept []*yaml.Node
		for i := 0; i+1 < len(n.Content); i += 2 {
			k, v := n.Content[i], n.Content[i+1]
			prune(v)
			if isZero(v) {
				continue
			}
			kept = append(kept, k, v)
		}
		n.Content = kept
		n.Style = 0
	case yaml.SequenceNode, yaml.DocumentNode:
		for _, c := range n.Content {
			prune(c)
		}
	default:
	}
}

func isZero(n *yaml.Node) bool {
	switch n.Kind { //nolint:exhaustive // documents and aliases are never zero
	case yaml.ScalarNode:
		return (n.Tag == "!!str" && n.Value == "") || (n.Tag == "!!int" && n.Value == "0") ||
			(n.Tag == "!!bool" && n.Value == "false") || n.Tag == "!!null"
	case yaml.MappingNode, yaml.SequenceNode:
		return len(n.Content) == 0
	default:
		return false
	}
}

func replaceInMap(m map[string]any, old, repl string) {
	for k, v := range m {
		switch val := v.(type) {
		case map[string]any:
			replaceInMap(val, old, repl)
		case string:
			m[k] = strings.ReplaceAll(val, old, repl)
		}
	}
}

// wrap breaks s into lines of at most width, indenting continuations.
func wrap(s string, width int, indent string) string {
	words := strings.Fields(s)
	var lines []string
	var cur string
	for _, w := range words {
		if cur != "" && len(cur)+1+len(w) > width {
			lines = append(lines, cur)
			cur = w
			continue
		}
		if cur == "" {
			cur = w
		} else {
			cur += " " + w
		}
	}
	if cur != "" {
		lines = append(lines, cur)
	}
	return strings.Join(lines, "\n"+indent)
}
