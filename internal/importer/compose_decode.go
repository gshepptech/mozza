package importer

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Compose documents more than one shape for most of the fields below — a port is
// a string or a mapping, a replica count is a number or a string — and a file
// using any of them is a file `docker compose` accepts. Each such field is
// normalised here, as it is unmarshalled, into the single shape the conversion
// functions in compose.go read: a long-form port becomes the short syntax
// "published:target", a long-form mount becomes "source:target", a bare secret
// name becomes a composeSecretRef. Nothing downstream learns a second shape,
// because a field with two representations is a field whose two readers can come
// to different answers about the same compose file.
//
// The scalar fields — Replicas, Internal — decode the way they always did first
// and reach the string form only when that fails, so a file that parses today
// provably takes the path it took before.

// The tags yaml resolves a value to, as they are spelled in its own messages.
// nullTag earns its keep twice over: yaml's sequence decoder drops a null
// element rather than appending a zero value, so the unmarshalers below skip one
// to decode a list exactly as a plain []string field would have.
const (
	nullTag = "!!null"
	seqTag  = "!!seq"
	mapTag  = "!!map"
)

// resolveAlias follows a node through an alias to the node holding the value.
//
// yaml resolves an alias itself when it decodes into a typed field, but a
// yaml.Node field captures the alias node as it stands, and so does a walk over a
// sequence's own Content. Read off one of those, .Kind describes a reference
// rather than a value and .Value is the anchor's name — so an anchor named "off"
// or "5" reads as the value "off" or "5". Sharing config through an x- anchor is
// how compose files avoid repeating themselves, so this is ordinary input rather
// than an edge. yaml rejects an anchor on an alias, so the chain is never longer
// than one hop.
func resolveAlias(n *yaml.Node) *yaml.Node {
	for n.Kind == yaml.AliasNode && n.Alias != nil {
		n = n.Alias
	}

	return n
}

// composeShapeError is a value one of the unmarshalers below could not read: the
// node at fault, and what mozza expected in its place.
//
// It carries the node rather than a field path because yaml offers an unmarshaler
// no path — from inside one, "ports" is reachable and "services.web.ports" is not,
// and there is no service name to be had. composeReadError names the field from
// the document, where the whole path is in hand. The name kept here is what a
// message falls back to when the node cannot be found there, so it says "service
// ports" rather than name a service mozza would be guessing at.
type composeShapeError struct {
	node  *yaml.Node
	field string
	want  string
}

func (e *composeShapeError) Error() string {
	return fmt.Sprintf("compose %s (line %d): expected %s", e.field, e.node.Line, e.want)
}

// composeUnreadable reports a node that fits none of the shapes compose documents
// for the field it stands in.
func composeUnreadable(n *yaml.Node, field, want string) error {
	return &composeShapeError{node: n, field: field, want: want}
}

// composeMirrorError reports a value that would not decode into the mirror one of
// the unmarshalers below reads its own scalars through.
//
// Where the value really is a mapping, yaml's complaint is the better one and is
// handed back for composeReadError to restate: it carries the line of the entry at
// fault rather than the line the mapping opens on, and a key given twice reaches
// the message that already reads "set twice, first at line 3". Restating it as a
// shape complaint threw both away and blamed the whole mapping for a fault one
// level inside it. Only a value that is not a mapping at all is described here.
func composeMirrorError(n *yaml.Node, err error, field, want string) error {
	if n.Kind == yaml.MappingNode {
		return err
	}

	return composeUnreadable(n, field, want)
}

// composeFile is a representation of a docker-compose.yml (v3.x).
//
// Top-level configs and secrets carry no values: only the config names are read
// (to warn that they are dropped), secrets are not read at all, and an empty
// struct accepts every shape an entry takes — file, content, environment, name,
// `external: true` and the legacy `external: {name: real}` — instead of failing
// the whole parse over a field nothing consults.
type composeFile struct {
	Services map[string]composeService       `yaml:"services"`
	Volumes  map[string]composeVolumeConfig  `yaml:"volumes"`
	Networks map[string]composeNetworkConfig `yaml:"networks"`
	Configs  map[string]struct{}             `yaml:"configs"`
	Secrets  map[string]struct{}             `yaml:"secrets"`

	// Include is read for whether it is there and for nothing else. Compose
	// merges the services of every file it names into this one project, on this
	// file's networks; mozza resolves none of them, so its services are not all
	// here to be read. Any shape parses, because the only question asked of it is
	// whether the key was written.
	Include interface{} `yaml:"include"`
}

// composeService represents a single service in a docker-compose file.
//
// There is deliberately no Configs field. A service's configs were parsed and
// never read, so the only thing the field could do was fail the parse on the
// short syntax; dropping it lets both documented shapes through to the top-level
// warning that already names every declared config.
type composeService struct {
	Image       string              `yaml:"image"`
	Build       interface{}         `yaml:"build"` // string or map
	Ports       composePorts        `yaml:"ports"`
	DependsOn   interface{}         `yaml:"depends_on"` // []string or map[string]...
	Environment interface{}         `yaml:"environment"`
	Volumes     composeMounts       `yaml:"volumes"`
	Restart     string              `yaml:"restart"`
	Deploy      *composeDeploy      `yaml:"deploy"`
	Healthcheck *composeHealthcheck `yaml:"healthcheck"`
	Networks    interface{}         `yaml:"networks"` // []string or map[string]...
	NetworkMode string              `yaml:"network_mode"`
	Secrets     []composeSecretRef  `yaml:"secrets"`

	// Extends and ExternalLinks are read for whether they are there and for
	// nothing else, and any shape parses for the same reason Include does. Each
	// says this service reaches, or is reached over, something the file's
	// `networks:` keys do not record: a base service whose own networks mozza
	// does not resolve, and a container outside the project sharing a network
	// with this one.
	Extends       interface{} `yaml:"extends"` // string or map[string]...
	ExternalLinks interface{} `yaml:"external_links"`
}

type composeDeploy struct {
	Replicas int `yaml:"replicas"`
}

// UnmarshalYAML reads replicas in every shape compose accepts. Compose's own
// schema error calls it "a integer or string", so `replicas: "3"` is as
// documented as `replicas: 3`, and a file using it is one `docker compose config`
// renders without complaint.
//
// The number is decoded first, by the same decoder as before, and the string form
// is reached only when that fails — which is what keeps `3` and the equally valid
// `3.0` on the path they already took. ${VAR} is not expanded anywhere in the
// importer, so it is named in the error rather than guessed at.
func (d *composeDeploy) UnmarshalYAML(n *yaml.Node) error {
	// Every other deploy key — resources, placement, update_config — goes on
	// being ignored rather than rejected.
	var mirror struct {
		Replicas yaml.Node `yaml:"replicas"`
	}
	if err := n.Decode(&mirror); err != nil {
		return composeMirrorError(n, err, "service deploy", "a mapping of deploy settings")
	}

	const field = "service deploy.replicas"

	replicas := resolveAlias(&mirror.Replicas)
	if replicas.IsZero() {
		return nil
	}

	if err := replicas.Decode(&d.Replicas); err == nil {
		return nil
	}

	if replicas.Kind != yaml.ScalarNode {
		return composeUnreadable(replicas, field, "a number of copies, got a list or a mapping")
	}

	raw := replicas.Value
	if count, err := strconv.Atoi(raw); err == nil {
		d.Replicas = count

		return nil
	}

	// ${VAR} is the one wrong value worth naming outright, because compose
	// interpolates it and mozza does not, so the number the author is looking at is
	// one that never reached here. Saying so about a value with no ${ in it would
	// blame interpolation for a plain typo.
	hint := ""
	if strings.Contains(raw, "${") {
		hint = " — mozza does not expand ${VAR} in compose files"
	}

	return composeUnreadable(replicas, field,
		fmt.Sprintf("a number of copies, got %q%s", raw, hint))
}

type composeHealthcheck struct {
	Test composeTest `yaml:"test"`
}

// composeTest is a health check's test command. Compose accepts a bare string as
// the shell form and normalises it to ["CMD-SHELL", s] — literally what `docker
// compose config` prints for it — so that is what it becomes here, leaving
// extractHealthCheck reading the list it has always read.
type composeTest []string

func (t *composeTest) UnmarshalYAML(n *yaml.Node) error {
	const (
		field = "service healthcheck.test"
		want  = "a command string, or a list of command arguments"
	)

	if n.Kind == yaml.ScalarNode {
		var shell string
		if err := n.Decode(&shell); err != nil {
			return composeUnreadable(n, field, want)
		}
		*t = composeTest{"CMD-SHELL", shell}

		return nil
	}

	var args []string
	if err := n.Decode(&args); err != nil {
		return composeUnreadable(n, field, want)
	}
	*t = args

	return nil
}

type composeVolumeConfig struct {
	Driver string `yaml:"driver"`
}

type composeNetworkConfig struct {
	Driver     string `yaml:"driver"`
	Name       string `yaml:"name"`
	Internal   bool   `yaml:"internal"`
	External   bool   `yaml:"external"`
	Attachable bool   `yaml:"attachable"`
}

// UnmarshalYAML reads a network's settings, taking each flag in every shape
// compose accepts. Compose calls them "a boolean or string", so
// `internal: "true"` is as documented as `internal: true`, and external carries a
// third form on top: `external: {name: legacy_net}`, the legacy spelling that
// names the network to join. All three render as `external: true` under
// `docker compose config`.
//
// Each boolean is decoded first, by the same decoder as before, which is what
// keeps yaml's own vocabulary — including bare yes/no/on/off, and TRUE in any
// case — working byte for byte. Only when that fails is the value read as a
// string, and then only compose's own vocabulary is honoured: it accepts these
// twelve spellings and rejects "0", "1" and anything else, so mozza saying so
// out loud is the same answer compose gives.
func (c *composeNetworkConfig) UnmarshalYAML(n *yaml.Node) error {
	var mirror struct {
		Driver     string    `yaml:"driver"`
		Name       string    `yaml:"name"`
		Internal   yaml.Node `yaml:"internal"`
		External   yaml.Node `yaml:"external"`
		Attachable yaml.Node `yaml:"attachable"`
	}
	if err := n.Decode(&mirror); err != nil {
		return composeMirrorError(n, err, "networks", "a mapping of network settings")
	}

	c.Driver = mirror.Driver
	c.Name = mirror.Name

	if err := readNetworkFlag(
		&mirror.Internal, "networks.internal", "true or false, got a list or a mapping", &c.Internal,
	); err != nil {
		return err
	}

	if err := readNetworkFlag(
		&mirror.Attachable, "networks.attachable", "true or false, got a list or a mapping",
		&c.Attachable,
	); err != nil {
		return err
	}

	// The legacy long form is a mapping holding the real network's name, the same
	// value compose normalises into the network's own `name:`. It is read as the
	// flag it stands for and its contents left alone: an external network's
	// boundary cannot be derived whatever the network is called, so carrying the
	// name here would change which suppression fires and nothing else.
	external := resolveAlias(&mirror.External)
	if external.Kind == yaml.MappingNode {
		c.External = true

		return nil
	}

	return readNetworkFlag(external, "networks.external", "true or false, got a list", &c.External)
}

// readNetworkFlag reads one of a network's boolean flags off the node holding it,
// leaving the flag untouched when the key is absent.
//
// notScalar is what to say when the value is neither true nor false nor a string
// standing for one, because that is not the same set for both flags: a mapping is
// unreadable under internal but is external's legacy long form, and is read before
// this is reached. Naming a shape the caller would in fact have accepted is worse
// than saying nothing.
func readNetworkFlag(n *yaml.Node, field, notScalar string, flag *bool) error {
	value := resolveAlias(n)
	if value.IsZero() {
		return nil
	}

	if err := value.Decode(flag); err == nil {
		return nil
	}

	if value.Kind != yaml.ScalarNode {
		return composeUnreadable(value, field, notScalar)
	}

	raw := value.Value
	switch strings.ToLower(raw) {
	case "true", "yes", "on":
		*flag = true
	case "false", "no", "off":
		*flag = false
	default:
		return composeUnreadable(value, field, fmt.Sprintf("true or false, got %q", raw))
	}

	return nil
}

// composeSecretRef is a secret reference within a service, in either shape
// compose documents: the bare name `- api-key`, or a mapping carrying a source.
type composeSecretRef struct {
	Source string `yaml:"source"`
	Target string `yaml:"target"`
}

func (s *composeSecretRef) UnmarshalYAML(n *yaml.Node) error {
	const (
		field = "service secrets"
		want  = "a secret name, or an entry with a source"
	)

	if n.Kind == yaml.ScalarNode {
		var name string
		if err := n.Decode(&name); err != nil {
			return composeUnreadable(n, field, want)
		}
		s.Source = name

		return nil
	}

	// An alias type so decoding the mapping does not call this method again.
	type plain composeSecretRef

	var ref plain
	if err := n.Decode(&ref); err != nil {
		return composeUnreadable(n, field, want)
	}
	*s = composeSecretRef(ref)

	return nil
}

// composePorts is a service's ports, each normalised to the short syntax that
// parsePort and writePort already read.
//
// protocol, mode and host_ip are dropped. parsePort drops the protocol from short
// syntax too, and a recipe has nowhere to name a host interface — so a long-form
// port publishing on 127.0.0.1 imports as the port it publishes, which is the
// answer the short form ought to give and does not yet.
type composePorts []string

func (p *composePorts) UnmarshalYAML(n *yaml.Node) error {
	const (
		field = "service ports"
		want  = `a list of ports such as "8080:80", or a list of entries with target and published`
	)

	if n.Kind != yaml.SequenceNode {
		return composeUnreadable(n, field, want)
	}

	for _, item := range n.Content {
		item = resolveAlias(item)
		if item.Tag == nullTag {
			continue
		}

		spec, ok := portSpec(item)
		if !ok {
			return composeUnreadable(item, field, want)
		}

		*p = append(*p, spec)
	}

	return nil
}

// portSpec renders one entry of a compose ports list as short syntax, reporting
// false when the entry is neither shape compose documents.
func portSpec(item *yaml.Node) (string, bool) {
	if item.Kind == yaml.ScalarNode {
		var short string
		if err := item.Decode(&short); err != nil {
			return "", false
		}

		return short, true
	}

	if item.Kind != yaml.MappingNode {
		return "", false
	}

	// published is read back as a string whether compose wrote it as one or as
	// a number.
	var long struct {
		Target    string `yaml:"target"`
		Published string `yaml:"published"`
	}
	if err := item.Decode(&long); err != nil {
		return "", false
	}

	// A long-form port with no target names no container port, and compose refuses
	// the file for it ("is missing a target port"). Carrying it anyway would put
	// "8080:" in front of writePort, which writes the port after the colon — an
	// "open to the public on port" line with no number, in a recipe that then
	// validates. Failing here is the same answer compose gives.
	if long.Target == "" {
		return "", false
	}

	if long.Published == "" {
		return long.Target, true
	}

	return long.Published + ":" + long.Target, true
}

// composeMounts is a service's volumes, each normalised to the short syntax that
// splitVolume already reads.
//
// type, read_only and the driver blocks are dropped: splitVolume never read them
// from short syntax either, and what a slice does with a volume is decided by the
// engine it belongs to, not by how compose asked for it.
type composeMounts []string

func (m *composeMounts) UnmarshalYAML(n *yaml.Node) error {
	const (
		field = "service volumes"
		want  = `a list of mounts such as "pgdata:/var/lib/postgresql/data",` +
			` or a list of entries with source and target`
	)

	if n.Kind != yaml.SequenceNode {
		return composeUnreadable(n, field, want)
	}

	for _, item := range n.Content {
		item = resolveAlias(item)
		if item.Tag == nullTag {
			continue
		}

		spec, ok := mountSpec(item)
		if !ok {
			return composeUnreadable(item, field, want)
		}

		*m = append(*m, spec)
	}

	return nil
}

// mountSpec renders one entry of a compose volumes list as short syntax,
// reporting false when the entry is neither shape compose documents.
func mountSpec(item *yaml.Node) (string, bool) {
	if item.Kind == yaml.ScalarNode {
		var short string
		if err := item.Decode(&short); err != nil {
			return "", false
		}

		return short, true
	}

	if item.Kind != yaml.MappingNode {
		return "", false
	}

	var long struct {
		Source string `yaml:"source"`
		Target string `yaml:"target"`
	}
	if err := item.Decode(&long); err != nil {
		return "", false
	}

	// A mount with no target names no path to mount at, and compose refuses the file
	// for it ("is missing a mount target"). "pgdata:" would reach splitVolume as a
	// source with an empty path, which matches no mount and imports as a volume
	// quietly reported uncarried.
	if long.Target == "" {
		return "", false
	}

	// An anonymous or tmpfs mount names no source, and a bare target is what the
	// short syntax calls that.
	if long.Source == "" {
		return long.Target, true
	}

	return long.Source + ":" + long.Target, true
}
