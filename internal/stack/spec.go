// Package stack brings up a whole application environment on a local
// Kubernetes cluster: the cluster itself, the operators and CNI the app
// assumes, the images built from its source tree, the Secrets it refuses to
// start without, and finally its own Helm charts.
//
// A recipe describes one app Mozza writes the manifests for. A stack describes
// an app that already ships its own charts. The job there is not compiling,
// it is assembling the environment those charts were written against. The two
// are deliberately separate surfaces: a stack never goes through the recipe
// compiler, and a recipe never creates a cluster.
//
// Everything app-specific lives in data, not code. A stack file names the
// source tree, the images, the charts and the values; reusable pieces such as
// "Calico through the Tigera operator" or "the CloudNativePG operator" are
// addons in the embedded catalog, referenced by name. The engine knows how to
// run kind, k3d, docker, helm and kubectl; it knows nothing about any one app.
//
// Stacks are local by construction. Clusters bind their API server and every
// published port to the loopback interface, port-forwards listen on 127.0.0.1,
// and the engine refuses to touch a kubeconfig whose server is not loopback.
// There is no cloud provider and no option to add one.
package stack

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Provider names accepted in cluster.provider.
const (
	ProviderKind = "kind"
	ProviderK3d  = "k3d"
)

// Stack is one environment: a cluster and everything installed into it.
type Stack struct {
	// Name identifies the stack. It names the state directory and, unless
	// cluster.name says otherwise, the cluster.
	Name string `yaml:"name"`
	// Description is a one-line summary shown by `mozza stack list`.
	Description string `yaml:"description"`
	// Source locates the app's source tree, which relative paths resolve
	// against. A stack built only from public charts and images has none.
	Source Source `yaml:"source"`
	// Cluster describes the local cluster the stack runs on.
	Cluster Cluster `yaml:"cluster"`
	// Images are built (or pulled) on the host and loaded into the cluster,
	// so nothing the stack runs needs a registry the cluster can reach.
	Images []Image `yaml:"images"`
	// Secrets are created before any component installs. Generated values
	// persist in the state directory, so a second `up` reuses them instead of
	// rotating every credential the app already stored.
	Secrets []Secret `yaml:"secrets"`
	// Components install in order. A component is either a reference to a
	// catalog addon (use: calico) or an inline Helm release and/or manifests.
	Components []Component `yaml:"components"`
	// Access lists the services a person opens, forwarded to 127.0.0.1.
	Access []Access `yaml:"access"`
	// Notes are printed after a successful `up`: sign-in hints and the like.
	Notes []string `yaml:"notes"`

	// fileDir is the directory of the stack file when it was loaded from a
	// path; a relative source.path resolves against it.
	fileDir string
}

// Source locates an app's source tree on this machine.
type Source struct {
	// Path is the default location. A leading ~ expands to the home directory.
	Path string `yaml:"path"`
	// Env names an environment variable that overrides Path. Defaults to
	// MOZZA_<NAME>_SOURCE, upper-cased with dashes as underscores.
	Env string `yaml:"env"`
}

// Cluster describes the local cluster a stack runs on.
type Cluster struct {
	// Provider is kind or k3d.
	Provider string `yaml:"provider"`
	// Name is the cluster name; defaults to mozza-<stack name>.
	Name string `yaml:"name"`
	// NodeImage overrides the provider's node image (kindest/node:..., rancher/k3s:...).
	NodeImage string `yaml:"nodeImage"`
	// Workers is the number of worker nodes beyond the control plane.
	Workers int `yaml:"workers"`
	// DisableDefaultCNI leaves the cluster without a pod network so an addon
	// can install its own. Addons that need this set it through `needs`.
	DisableDefaultCNI bool `yaml:"disableDefaultCNI"`
	// PodSubnet overrides the pod CIDR.
	PodSubnet string `yaml:"podSubnet"`
	// ServiceSubnet overrides the Service CIDR.
	ServiceSubnet string `yaml:"serviceSubnet"`
	// NodeLabels are applied to every node.
	NodeLabels map[string]string `yaml:"nodeLabels"`
	// Ports publish node ports on the host's loopback interface.
	Ports []PortMapping `yaml:"ports"`
}

// PortMapping publishes a node port on 127.0.0.1.
type PortMapping struct {
	// Host is the port on 127.0.0.1.
	Host int `yaml:"host"`
	// Node is the port on the node container (typically a NodePort).
	Node int `yaml:"node"`
}

// Image is one container image the cluster needs present locally.
type Image struct {
	// Ref is the image reference the charts are told to use.
	Ref string `yaml:"ref"`
	// Build builds Ref from the source tree. Without it, Ref is pulled.
	Build *Build `yaml:"build"`
}

// Build describes a docker build.
type Build struct {
	// Context is the build context, relative to the source tree, or a git
	// URL (https://host/repo.git#ref) for an image whose source is not
	// checked out; docker clones it itself.
	Context string `yaml:"context"`
	// Dockerfile is relative to the source tree; defaults to <context>/Dockerfile.
	// With a git context it is relative to the repository root.
	Dockerfile string `yaml:"dockerfile"`
	// Target selects a multi-stage target.
	Target string `yaml:"target"`
	// Args are --build-arg values.
	Args map[string]string `yaml:"args"`
	// Exclude lists .dockerignore-style patterns kept out of the context on
	// top of the repository's own .dockerignore. Setting it makes Mozza stream
	// the context itself rather than handing docker a directory, which is how
	// a stack trims a context without writing a file into the app's tree.
	Exclude []string `yaml:"exclude"`
	// Rewrite applies regular-expression replacements to the Dockerfile as it
	// is streamed to docker. It is for a Dockerfile that cannot build as
	// written today (pinned package versions a mirror no longer serves), and
	// it changes only the copy Mozza sends, never the file in the app's tree.
	Rewrite []Rewrite `yaml:"rewrite"`
}

// Remote reports whether the context is a git URL rather than a directory.
func (b *Build) Remote() bool {
	return strings.HasPrefix(b.Context, "https://") || strings.HasPrefix(b.Context, "git@")
}

// Rewrite is one regular-expression replacement. Replace may use $1-style
// references to Pattern's groups.
type Rewrite struct {
	Pattern string `yaml:"pattern"`
	Replace string `yaml:"replace"`
	// Why says what the rewrite works around; it is printed when it applies.
	Why string `yaml:"why"`
}

// Secret is a Kubernetes Secret created before the components install.
type Secret struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
	// Type is the Secret type; defaults to Opaque.
	Type string `yaml:"type"`
	// Data maps a key to how its value is obtained.
	Data map[string]Value `yaml:"data"`
}

// Value is how one secret key gets its value. Exactly one of Generate, Value
// or FromEnv is set.
type Value struct {
	// Generate names a generator: hex, base64, password, age-key.
	Generate string `yaml:"generate"`
	// Length is the generator's size: bytes for hex and base64, characters
	// for password. Ignored by age-key.
	Length int `yaml:"length"`
	// Value is a literal. Use for non-sensitive fixed values such as a username.
	Value string `yaml:"value"`
	// FromEnv reads the value from the named environment variable at `up` time.
	FromEnv string `yaml:"fromEnv"`
	// Show marks a value a person needs (an admin password), so `up` and
	// `mozza stack secrets` print it. Everything else stays out of the output.
	Show bool `yaml:"show"`
}

// Component is one installable unit, installed in list order.
type Component struct {
	// Use names a catalog addon. When set, every other field except Version
	// and Values must be empty: the addon defines them.
	Use string `yaml:"use"`
	// Version overrides the addon's default version.
	Version string `yaml:"version"`

	// Name identifies the component in output and is the Helm release name.
	Name string `yaml:"name"`
	// Namespace is where the Helm release and namespaced manifests go.
	Namespace string `yaml:"namespace"`
	// Needs lists what the cluster must provide for this component to work.
	Needs Needs `yaml:"needs"`
	// Manifests are applied before the Helm release, in order.
	Manifests []Manifest `yaml:"manifests"`
	// Helm installs a chart.
	Helm *Helm `yaml:"helm"`
	// Values are Helm values merged over the chart's (and, under use, the addon's).
	Values map[string]any `yaml:"values"`
	// Ansible runs a playbook after the manifests and the chart.
	Ansible *Ansible `yaml:"ansible"`
	// Wait conditions are checked after everything else in the component.
	Wait []Wait `yaml:"wait"`
}

// Needs is what a component requires of the cluster. It is folded into the
// cluster definition before the cluster is created, which is why a CNI addon
// can switch off the default CNI without the stack author knowing it must.
type Needs struct {
	DisableDefaultCNI bool   `yaml:"disableDefaultCNI"`
	PodSubnet         string `yaml:"podSubnet"`
}

// Manifest is a set of Kubernetes objects to apply. Exactly one of URL, Path
// or Inline is set.
type Manifest struct {
	URL    string `yaml:"url"`
	Path   string `yaml:"path"`
	Inline string `yaml:"inline"`
	// ServerSide applies server-side, which large CRD bundles need: their
	// last-applied annotation overflows the client-side apply limit.
	ServerSide bool `yaml:"serverSide"`
	// WaitFor names objects to wait for before this manifest applies, such as
	// a CRD the manifest instantiates.
	WaitFor []Wait `yaml:"waitFor"`
}

// Helm describes one chart install.
type Helm struct {
	// Chart is a path relative to the source tree, a chart name in Repo, or an
	// oci:// reference.
	Chart string `yaml:"chart"`
	// Repo is a chart repository URL. Passed per install with --repo so the
	// user's `helm repo` list is never modified.
	Repo string `yaml:"repo"`
	// Version pins the chart version (repo and OCI charts).
	Version string `yaml:"version"`
	// ValuesFiles are paths relative to the source tree, applied in order.
	ValuesFiles []string `yaml:"valuesFiles"`
	// Timeout bounds helm's --wait; defaults to 10m.
	Timeout string `yaml:"timeout"`
	// NoWait skips helm's --wait, for charts whose readiness is checked by
	// explicit wait conditions instead.
	NoWait bool `yaml:"noWait"`
}

// Ansible runs one playbook against the stack's own cluster.
//
// Mozza writes the inventory itself, and it names only local targets: the
// cluster's node containers and the container the playbook runs in. A
// playbook's groups (k8s-servers, jumphost, ...) are mapped onto those
// targets with Groups. An inventory of the playbook's own is never used,
// which is what keeps a playbook written for real machines from reaching
// them.
type Ansible struct {
	// Playbook is relative to the source tree.
	Playbook string `yaml:"playbook"`
	// Dir is where ansible-playbook runs, so the tree's ansible.cfg, roles/
	// and group_vars/ are found; defaults to the playbook's directory.
	Dir string `yaml:"dir"`
	// Requirements is a galaxy requirements file, installed into the stack's
	// state directory before the playbook runs.
	Requirements string `yaml:"requirements"`
	// Groups maps a group the playbook targets to a local target: nodes,
	// control_plane, workers or runner. "all" needs no mapping.
	Groups map[string]string `yaml:"groups"`
	// ExtraVars are passed with -e. They may use ${secret:ns/name/key}.
	ExtraVars map[string]any `yaml:"extraVars"`
	// Tags and SkipTags select tasks, as --tags and --skip-tags do.
	Tags     []string `yaml:"tags"`
	SkipTags []string `yaml:"skipTags"`
	// Env sets environment variables in the runner. ANSIBLE_* variables
	// override the tree's ansible.cfg, which is how a stack adjusts a config
	// written for real machines without editing it.
	Env map[string]string `yaml:"env"`
	// Limit narrows the hosts, as --limit does. A playbook on "all" reaches
	// the nodes and the runner; "nodes" keeps it to the nodes.
	Limit string `yaml:"limit"`
}

// Wait is one readiness condition, checked with kubectl.
type Wait struct {
	// Resource is kind/name (crd/foo, deployment/bar) or a kind with Selector.
	Resource string `yaml:"resource"`
	// Namespace scopes Resource.
	Namespace string `yaml:"namespace"`
	// Selector is a label selector, used with a bare kind in Resource.
	Selector string `yaml:"selector"`
	// For is the kubectl wait condition (condition=Available,
	// jsonpath={.status.phase}=Running). Empty means "exists".
	For string `yaml:"for"`
	// Rollout waits for `kubectl rollout status` instead of For.
	Rollout bool `yaml:"rollout"`
	// Timeout defaults to 5m.
	Timeout string `yaml:"timeout"`
}

// Access is one service forwarded to the host.
type Access struct {
	// Name labels the endpoint in output.
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace"`
	// Service is the Service to forward to.
	Service string `yaml:"service"`
	// Port is the Service port.
	Port int `yaml:"port"`
	// LocalPort is the port on 127.0.0.1; defaults to Port.
	LocalPort int `yaml:"localPort"`
	// Path is appended to the printed URL.
	Path string `yaml:"path"`
	// Health is an HTTP path polled after the forward starts to confirm the
	// app answers. Empty skips the check.
	Health string `yaml:"health"`
}

var nameRE = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]{0,30}[a-z0-9])?$`)

// ansibleTargets are the local targets an Ansible group can map onto.
var ansibleTargets = map[string]bool{ //nolint:gochecknoglobals // immutable lookup table
	"nodes":         true,
	"control_plane": true,
	"workers":       true,
	"runner":        true,
}

var generators = map[string]bool{ //nolint:gochecknoglobals // immutable lookup table
	"hex":      true,
	"base64":   true,
	"password": true,
	"age-key":  true,
}

// Validate reports every problem in a stack at once, so an author fixes a file
// in one pass rather than one error per run. It checks shape only; whether a
// chart or image exists is learned by running it.
func (s *Stack) Validate() error {
	var errs []error
	add := func(format string, a ...any) { errs = append(errs, fmt.Errorf(format, a...)) }

	if !nameRE.MatchString(s.Name) {
		add("name %q must be lowercase letters, digits and dashes, at most 32 characters", s.Name)
	}

	switch s.Cluster.Provider {
	case ProviderKind, ProviderK3d:
	default:
		add("cluster.provider %q must be %q or %q", s.Cluster.Provider, ProviderKind, ProviderK3d)
	}
	if s.Cluster.Name != "" && !nameRE.MatchString(s.Cluster.Name) {
		add("cluster.name %q must be lowercase letters, digits and dashes, at most 32 characters", s.Cluster.Name)
	}
	if s.Cluster.Workers < 0 {
		add("cluster.workers must not be negative")
	}
	for i, p := range s.Cluster.Ports {
		if !validPort(p.Host) || !validPort(p.Node) {
			add("cluster.ports[%d]: host and node must be 1-65535", i)
		}
	}

	for i, img := range s.Images {
		if img.Ref == "" {
			add("images[%d]: ref is required", i)
		}
		if img.Build != nil && img.Build.Context == "" {
			add("images[%d] (%s): build.context is required", i, img.Ref)
		}
		if img.Build != nil && img.Build.Remote() && (len(img.Build.Exclude) > 0 || len(img.Build.Rewrite) > 0) {
			add("images[%d] (%s): exclude and rewrite need a local context, not a git URL", i, img.Ref)
		}
		if img.Build != nil {
			for j, rw := range img.Build.Rewrite {
				if _, err := regexp.Compile(rw.Pattern); err != nil || rw.Pattern == "" {
					add("images[%d] (%s): rewrite[%d]: pattern %q is not a regular expression", i, img.Ref, j, rw.Pattern)
				}
			}
		}
	}

	for i, sec := range s.Secrets {
		if sec.Name == "" || sec.Namespace == "" {
			add("secrets[%d]: name and namespace are required", i)
		}
		if len(sec.Data) == 0 {
			add("secrets[%d] (%s): data is empty", i, sec.Name)
		}
		for key, v := range sec.Data {
			if err := v.validate(); err != nil {
				add("secrets[%d] (%s) key %q: %w", i, sec.Name, key, err)
			}
		}
	}

	for i, c := range s.Components {
		if err := c.validate(); err != nil {
			add("components[%d]: %w", i, err)
		}
	}

	for i, a := range s.Access {
		if a.Name == "" || a.Namespace == "" || a.Service == "" {
			add("access[%d]: name, namespace and service are required", i)
		}
		if !validPort(a.Port) || (a.LocalPort != 0 && !validPort(a.LocalPort)) {
			add("access[%d] (%s): port and localPort must be 1-65535", i, a.Name)
		}
	}

	return errors.Join(errs...)
}

func (v Value) validate() error {
	set := 0
	for _, s := range []string{v.Generate, v.Value, v.FromEnv} {
		if s != "" {
			set++
		}
	}
	if set != 1 {
		return errors.New("set exactly one of generate, value or fromEnv")
	}
	if v.Generate != "" && !generators[v.Generate] {
		return fmt.Errorf("unknown generator %q (hex, base64, password, age-key)", v.Generate)
	}
	if v.Length < 0 {
		return errors.New("length must not be negative")
	}
	return nil
}

func (c Component) validate() error {
	if c.Use != "" {
		if c.Name != "" || c.Namespace != "" || c.Helm != nil || len(c.Manifests) > 0 || len(c.Wait) > 0 || c.Ansible != nil {
			return fmt.Errorf("use: %s takes only version and values; the addon defines the rest", c.Use)
		}
		return nil
	}
	if c.Version != "" {
		return fmt.Errorf("%s: version applies only with use; pin an inline chart with helm.version", c.Name)
	}
	if c.Name == "" {
		return errors.New("name is required")
	}
	if c.Helm == nil && len(c.Manifests) == 0 && len(c.Wait) == 0 && c.Ansible == nil {
		return fmt.Errorf("%s: needs helm, manifests, ansible or wait", c.Name)
	}
	if c.Ansible != nil {
		if c.Ansible.Playbook == "" {
			return fmt.Errorf("%s: ansible.playbook is required", c.Name)
		}
		for group, target := range c.Ansible.Groups {
			if !ansibleTargets[target] {
				return fmt.Errorf("%s: ansible.groups.%s: %q is not a local target (nodes, control_plane, workers, runner)",
					c.Name, group, target)
			}
		}
	}
	if c.Helm != nil {
		if c.Helm.Chart == "" {
			return fmt.Errorf("%s: helm.chart is required", c.Name)
		}
		if c.Namespace == "" {
			return fmt.Errorf("%s: namespace is required for a helm release", c.Name)
		}
	}
	if len(c.Values) > 0 && c.Helm == nil {
		return fmt.Errorf("%s: values given but there is no helm chart", c.Name)
	}
	for i, m := range c.Manifests {
		set := 0
		for _, s := range []string{m.URL, m.Path, m.Inline} {
			if s != "" {
				set++
			}
		}
		if set != 1 {
			return fmt.Errorf("%s: manifests[%d]: set exactly one of url, path or inline", c.Name, i)
		}
		if m.URL != "" && !strings.HasPrefix(m.URL, "https://") {
			return fmt.Errorf("%s: manifests[%d]: url must be https", c.Name, i)
		}
	}
	for i, w := range c.Wait {
		if w.Resource == "" {
			return fmt.Errorf("%s: wait[%d]: resource is required", c.Name, i)
		}
	}
	return nil
}

func validPort(p int) bool { return p >= 1 && p <= 65535 }

// ClusterName is the cluster's name, defaulting to mozza-<stack>.
func (s *Stack) ClusterName() string {
	if s.Cluster.Name != "" {
		return s.Cluster.Name
	}
	return "mozza-" + s.Name
}

// KubeContext is the context name the provider writes for the cluster.
func (s *Stack) KubeContext() string {
	return s.Cluster.Provider + "-" + s.ClusterName()
}
