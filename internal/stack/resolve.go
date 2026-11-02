package stack

import (
	"errors"
	"fmt"
	"hash/fnv"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// ResolveOptions are the per-run inputs to Resolve.
type ResolveOptions struct {
	// Source overrides the stack's source path (the --source flag).
	Source string
	// Provider overrides cluster.provider (the --provider flag).
	Provider string
	// NoSource skips locating the source tree, for commands that act on a
	// running stack (status, forward, down) and must work even after the
	// checkout has moved.
	NoSource bool
}

// Resolved is a stack ready to run: addons inlined, cluster needs folded in,
// the source tree located, and every ${var} expanded.
type Resolved struct {
	*Stack
	// SourceDir is the absolute source tree, or "" for a stack without one.
	SourceDir string
}

// Resolve turns a loaded stack into one the engine can run. It returns a new
// stack; the input is not modified.
func (c Catalog) Resolve(s *Stack, opts ResolveOptions) (*Resolved, error) {
	// A YAML round trip is the deep copy: the stack is plain data, and
	// Resolve rewrites paths through pointers it must not share with s.
	b, err := yaml.Marshal(s)
	if err != nil {
		return nil, fmt.Errorf("resolve: %w", err)
	}
	var out Stack
	if uerr := yaml.Unmarshal(b, &out); uerr != nil {
		return nil, fmt.Errorf("resolve: %w", uerr)
	}
	if opts.Provider != "" {
		if opts.Provider != ProviderKind && opts.Provider != ProviderK3d {
			return nil, fmt.Errorf("--provider %q must be %q or %q", opts.Provider, ProviderKind, ProviderK3d)
		}
		out.Cluster.Provider = opts.Provider
	}

	copied := out.Components
	out.Components = nil

	var src string
	if !opts.NoSource {
		if src, err = sourceDir(s, opts.Source); err != nil {
			return nil, err
		}
	}

	for _, comp := range copied {
		if comp.Use == "" {
			out.Components = append(out.Components, comp)
			continue
		}
		expanded, err := c.expandAddon(comp)
		if err != nil {
			return nil, err
		}
		out.Components = append(out.Components, expanded)
	}

	if err := foldNeeds(&out.Cluster, out.Components); err != nil {
		return nil, err
	}
	assignSubnets(&out)

	vars := map[string]string{
		"source":  src,
		"stack":   out.Name,
		"cluster": out.ClusterName(),
		"context": out.KubeContext(),
		// The provider's default StorageClass, so a stack can name it without
		// tying itself to one provider.
		"storageClass": defaultStorageClass[out.Cluster.Provider],
	}
	// Each forwarded endpoint's origin, for apps that must be told the URL a
	// browser reaches them on (CORS, cookie and redirect checks).
	for _, a := range out.Access {
		vars["access."+a.Name+".url"] = "http://127.0.0.1:" + strconv.Itoa(localPort(a))
	}
	if err := expandAll(&out, vars); err != nil {
		return nil, err
	}

	if src != "" {
		if err := rebasePaths(&out, src); err != nil {
			return nil, err
		}
	} else if !opts.NoSource && needsSource(&out) {
		return nil, fmt.Errorf("stack %s refers to files in its source tree but has no source.path", out.Name)
	}

	return &Resolved{Stack: &out, SourceDir: src}, nil
}

var defaultStorageClass = map[string]string{ //nolint:gochecknoglobals // immutable lookup table
	ProviderKind: "standard",
	ProviderK3d:  "local-path",
}

// SourceEnv is the environment variable that overrides a stack's source path.
func (s *Stack) SourceEnv() string {
	if s.Source.Env != "" {
		return s.Source.Env
	}
	return "MOZZA_" + strings.ToUpper(strings.ReplaceAll(s.Name, "-", "_")) + "_SOURCE"
}

// sourceDir picks the source tree: the flag, then the environment, then the
// stack's default. A source that is named but missing is an error rather than
// a skip, because every relative path in the stack would then be wrong.
func sourceDir(s *Stack, flag string) (string, error) {
	p := flag
	origin := "--source"
	if p == "" {
		p = os.Getenv(s.SourceEnv())
		origin = s.SourceEnv()
	}
	if p == "" {
		p = s.Source.Path
		origin = "source.path"
	}
	if p == "" {
		return "", nil
	}

	p = expandHome(p)
	// A stack file kept next to its app says where the app is relative to
	// itself, not to wherever mozza happens to run.
	if origin == "source.path" && s.fileDir != "" && !filepath.IsAbs(p) {
		p = filepath.Join(s.fileDir, p)
	}
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", fmt.Errorf("source: %w", err)
	}
	st, err := os.Stat(abs) //nolint:gosec // the user names their own source tree
	if err != nil || !st.IsDir() {
		return "", fmt.Errorf("source tree %s (from %s) is not a directory; point at a checkout with --source or %s",
			abs, origin, s.SourceEnv())
	}
	return abs, nil
}

func expandHome(p string) string {
	if p == "~" || strings.HasPrefix(p, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, strings.TrimPrefix(p, "~"))
		}
	}
	return p
}

// expandAddon inlines a catalog addon, applying the stack's version pin and
// merging its values over the addon's.
func (c Catalog) expandAddon(use Component) (Component, error) {
	a, err := c.addon(use.Use)
	if err != nil {
		return Component{}, fmt.Errorf("component use: %s: %w", use.Use, err)
	}

	comp := a.Component
	version := comp.Version
	if use.Version != "" {
		version = use.Version
	}
	comp.Version = ""

	// Values are deep-copied before merging: the addon's maps come from a
	// decode that is reused by nothing else today, but sharing them would make
	// a second use of the same addon see the first one's overrides.
	comp.Values = mergeValues(deepCopyMap(comp.Values), use.Values)

	if err := expandAll(&comp, map[string]string{"version": version}); err != nil {
		return Component{}, fmt.Errorf("addon %s: %w", use.Use, err)
	}
	return comp, nil
}

// foldNeeds merges every component's cluster needs into the cluster
// definition. Two components that need different pod subnets cannot share a
// cluster, so that is an error rather than a silent last-one-wins.
func foldNeeds(cl *Cluster, comps []Component) error {
	for _, c := range comps {
		if c.Needs.DisableDefaultCNI {
			cl.DisableDefaultCNI = true
		}
		if c.Needs.PodSubnet == "" {
			continue
		}
		if cl.PodSubnet != "" && cl.PodSubnet != c.Needs.PodSubnet {
			return fmt.Errorf("component %s needs pod subnet %s but the cluster uses %s",
				c.Name, c.Needs.PodSubnet, cl.PodSubnet)
		}
		cl.PodSubnet = c.Needs.PodSubnet
	}
	return nil
}

// assignSubnets gives a cluster pod and Service ranges of its own, derived
// from the stack name, wherever the stack or an addon did not pin them.
//
// Provider defaults are the same for every cluster (k3s uses 10.42/10.43,
// kind 10.244/10.96), and on a Docker host that already runs another cluster
// those ranges can be claimed: on the machine this was built on, in-cluster
// traffic to 10.43.0.1 reached a different API server, every system pod
// failed TLS, and the cluster never came up. Distinct ranges per stack make
// stacks safe to run side by side and alongside other local clusters.
func assignSubnets(s *Stack) {
	h := fnv.New32a()
	_, _ = h.Write([]byte(s.Name))
	n := h.Sum32() % 40
	if s.Cluster.ServiceSubnet == "" {
		s.Cluster.ServiceSubnet = fmt.Sprintf("10.%d.0.0/16", 160+n)
	}
	if s.Cluster.PodSubnet == "" {
		s.Cluster.PodSubnet = fmt.Sprintf("10.%d.0.0/16", 200+n)
	}
}

var varRE = regexp.MustCompile(`\$\{([^}]*)\}`)

// expandAll replaces ${name} in every string reachable from v, including map
// values and keys of Helm values. ${env.NAME} reads the environment. An
// unknown variable is an error so a typo fails the run instead of shipping a
// literal "${sorce}" into a chart. ${secret:ns/name/key} is left for the
// engine, because a generated value does not exist until `up` runs.
func expandAll(v any, vars map[string]string) error {
	var errs []error
	expand := func(s string) string {
		return varRE.ReplaceAllStringFunc(s, func(m string) string {
			key := m[2 : len(m)-1]
			if env, ok := strings.CutPrefix(key, "env."); ok {
				return os.Getenv(env)
			}
			// Secret references resolve at install time, once the value
			// exists; see Engine.expandSecretRefs.
			if strings.HasPrefix(key, "secret:") {
				return m
			}
			if val, ok := vars[key]; ok {
				return val
			}
			errs = append(errs, fmt.Errorf("unknown variable %s", m))
			return m
		})
	}
	walkStrings(reflect.ValueOf(v), expand)
	return errors.Join(errs...)
}

// walkStrings rewrites every settable string under v in place.
func walkStrings(v reflect.Value, f func(string) string) {
	switch v.Kind() { //nolint:exhaustive // only kinds that can hold text are walked
	case reflect.Pointer, reflect.Interface:
		if v.IsNil() {
			return
		}
		if v.Kind() == reflect.Interface {
			// Values inside interfaces (Helm value maps) are not addressable,
			// so they are rebuilt and stored back.
			inner := v.Elem()
			if inner.Kind() == reflect.String && v.CanSet() {
				v.Set(reflect.ValueOf(f(inner.String())))
				return
			}
			walkStrings(inner, f)
			return
		}
		walkStrings(v.Elem(), f)
	case reflect.Struct:
		for i := range v.NumField() {
			if v.Type().Field(i).IsExported() {
				walkStrings(v.Field(i), f)
			}
		}
	case reflect.Slice:
		for i := range v.Len() {
			walkStrings(v.Index(i), f)
		}
	case reflect.Map:
		for _, k := range v.MapKeys() {
			val := v.MapIndex(k)
			nv := reflect.New(val.Type()).Elem()
			nv.Set(val)
			walkStrings(nv, f)
			nk := k
			if k.Kind() == reflect.String {
				nk = reflect.ValueOf(f(k.String())).Convert(k.Type())
			}
			if nk.Interface() != k.Interface() {
				v.SetMapIndex(k, reflect.Value{})
			}
			v.SetMapIndex(nk, nv)
		}
	case reflect.String:
		if v.CanSet() {
			v.SetString(f(v.String()))
		}
	default:
		// Numbers, booleans and the rest carry no text to expand.
	}
}

// rebasePaths makes every source-relative path absolute.
func rebasePaths(s *Stack, src string) error {
	abs := func(p string) string {
		if p == "" || filepath.IsAbs(p) {
			return p
		}
		return filepath.Join(src, p)
	}

	for i := range s.Images {
		b := s.Images[i].Build
		if b == nil || b.Remote() {
			continue
		}
		b.Context = abs(b.Context)
		if b.Dockerfile == "" {
			b.Dockerfile = filepath.Join(b.Context, "Dockerfile")
		} else {
			b.Dockerfile = abs(b.Dockerfile)
		}
	}
	for i := range s.Components {
		c := &s.Components[i]
		if c.Helm != nil {
			if isLocalChart(c.Helm) {
				c.Helm.Chart = abs(c.Helm.Chart)
			}
			for j, f := range c.Helm.ValuesFiles {
				c.Helm.ValuesFiles[j] = abs(f)
			}
		}
		for j := range c.Manifests {
			c.Manifests[j].Path = abs(c.Manifests[j].Path)
		}
		if a := c.Ansible; a != nil {
			a.Playbook = abs(a.Playbook)
			a.Requirements = abs(a.Requirements)
			if a.Dir == "" {
				a.Dir = filepath.Dir(a.Playbook)
			} else {
				a.Dir = abs(a.Dir)
			}
			// The playbook runs in a container that sees only the source
			// tree, so everything it reads must live inside it.
			for _, p := range []string{a.Playbook, a.Dir, a.Requirements} {
				if p != "" && !within(src, p) {
					return fmt.Errorf("component %s: %s is outside the source tree %s", c.Name, p, src)
				}
			}
		}
	}
	return nil
}

// within reports whether path is dir or below it.
func within(dir, path string) bool {
	rel, err := filepath.Rel(dir, path)
	return err == nil && rel != ".." && !strings.HasPrefix(rel, "../")
}

// needsSource reports whether a stack without a source tree still refers to
// files inside one.
func needsSource(s *Stack) bool {
	for _, img := range s.Images {
		if img.Build != nil && !img.Build.Remote() && !filepath.IsAbs(img.Build.Context) {
			return true
		}
	}
	for _, c := range s.Components {
		if c.Helm != nil {
			if isLocalChart(c.Helm) && !filepath.IsAbs(c.Helm.Chart) {
				return true
			}
			for _, f := range c.Helm.ValuesFiles {
				if !filepath.IsAbs(f) {
					return true
				}
			}
		}
		for _, m := range c.Manifests {
			if m.Path != "" && !filepath.IsAbs(m.Path) {
				return true
			}
		}
		if c.Ansible != nil {
			// A playbook always runs from a mounted source tree.
			return true
		}
	}
	return false
}

// isLocalChart reports whether a chart reference is a directory on disk rather
// than a repository chart name or an OCI reference.
func isLocalChart(h *Helm) bool {
	return h.Repo == "" && !strings.HasPrefix(h.Chart, "oci://")
}

// mergeValues deep-merges over into base, the way successive -f files merge in
// Helm: maps merge key by key, anything else in over replaces base.
func mergeValues(base, over map[string]any) map[string]any {
	if base == nil {
		base = map[string]any{}
	}
	for k, ov := range over {
		bm, bok := base[k].(map[string]any)
		om, ook := ov.(map[string]any)
		if bok && ook {
			base[k] = mergeValues(bm, om)
			continue
		}
		base[k] = ov
	}
	return base
}

func deepCopyMap(m map[string]any) map[string]any {
	if m == nil {
		return nil
	}
	out := maps.Clone(m)
	for k, v := range out {
		if sub, ok := v.(map[string]any); ok {
			out[k] = deepCopyMap(sub)
		}
	}
	return out
}
