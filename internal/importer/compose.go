package importer

import (
	"errors"
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"

	"github.com/gshepptech/mozza/internal/recipe"
)

// Compose documents more than one shape for most of the fields below — a port is
// a string or a mapping, a replica count is a number or a string — and a file
// using any of them is a file `docker compose` accepts. Each such field is
// normalised here, as it is unmarshalled, into the single shape the conversion
// functions below already read: a long-form port becomes the short syntax
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

// yaml reports a shape it could not read in Go's own terms — "cannot unmarshal
// !!str `web` into map[string]importer.composeService" — naming a type and a tag
// that appear nowhere in a compose file. Compose's own validator reports the
// same fault as "services must be a mapping", so that is the shape used below:
// the field named by the dotted path compose itself prints, and the value named
// by what the file was expected to hold.
//
// The unmarshalers above never come through here. yaml hands an unmarshaler's
// error straight back rather than collecting it as a type error, so their
// wording reaches the reader as they wrote it.

// composeTypes says what each modelled type holds, in compose's words. The keys
// come from the types themselves so that renaming or retiring one cannot quietly
// orphan an entry and put its Go name back in front of a reader.
var composeTypes = map[string]string{ //nolint:gochecknoglobals // immutable lookup table
	reflect.TypeFor[composeFile]().String():                     "a mapping of compose settings",
	reflect.TypeFor[map[string]composeService]().String():       "a mapping of service names to their settings",
	reflect.TypeFor[composeService]().String():                  "a mapping of service settings",
	reflect.TypeFor[composeHealthcheck]().String():              "a mapping with a test",
	reflect.TypeFor[[]composeSecretRef]().String():              "a list of secrets",
	reflect.TypeFor[map[string]composeVolumeConfig]().String():  "a mapping of volume names to their settings",
	reflect.TypeFor[composeVolumeConfig]().String():             "a mapping of volume settings",
	reflect.TypeFor[map[string]composeNetworkConfig]().String(): "a mapping of network names to their settings",
	reflect.TypeFor[map[string]struct{}]().String():             "a mapping of names to their settings",
	reflect.TypeFor[struct{}]().String():                        "a mapping of settings",
}

// composeReadError restates yaml's own complaints in compose's vocabulary.
// Anything it has no quarrel with — a syntax error, or one of the unmarshalers'
// messages — is already in the reader's language and passes through untouched.
func composeReadError(content []byte, err error) error {
	// One of the unmarshalers knows the value it balked at but not the path to it,
	// so the field is named here, where the document is in hand.
	var shape *composeShapeError
	if errors.As(err, &shape) {
		return composeNamedShapeError(content, shape)
	}

	// Two of yaml's errors describe a compose file in terms no compose file uses:
	// a Go value it could not make a map key from, and a yaml tag. Neither
	// carries a line, so neither replacement pretends to.
	switch msg := err.Error(); {
	case strings.HasPrefix(msg, "yaml: invalid map key: "):
		return errors.New("compose file: a mapping key must be a name, not a list or a mapping")
	case strings.HasPrefix(msg, "yaml: !!binary value contains invalid base64 data"):
		return errors.New("compose file: a value is tagged as binary but is not valid base64")
	}

	var typeErr *yaml.TypeError
	if !errors.As(err, &typeErr) {
		return err
	}

	// Read again for the field names alone. The content has already parsed once,
	// so a failure here costs the names, not the message.
	var doc yaml.Node
	if docErr := yaml.Unmarshal(content, &doc); docErr != nil {
		doc = yaml.Node{}
	}

	msgs := make([]string, 0, len(typeErr.Errors))
	for _, reported := range typeErr.Errors {
		msgs = append(msgs, composeReadMessage(&doc, reported))
	}

	return errors.New(strings.Join(msgs, "; "))
}

// composeNamedShapeError names the field one of the unmarshalers balked at the
// way compose's own validator names it — services.web.ports — by finding the node
// it reported in the document it came from.
//
// The node is the field's own value, so anything sharing its line from further in
// belongs to a structure mozza never looked inside. A node the document does not
// answer to keeps the unmarshaler's own name for the field, which says "service"
// rather than a service name mozza has no way to know.
func composeNamedShapeError(content []byte, e *composeShapeError) error {
	var doc yaml.Node
	if docErr := yaml.Unmarshal(content, &doc); docErr != nil {
		return e
	}

	want := composeTarget{
		line:      e.node.Line,
		kind:      e.node.Kind,
		value:     e.node.Value,
		outermost: true,
	}

	field, at := composeField(&doc, want, "")
	if at == nil || field == "" {
		return e
	}

	return fmt.Errorf("compose %s (line %d): expected %s", field, e.node.Line, e.want)
}

// composeReadMessage restates one of yaml's type errors. Both shapes it reports
// open with the line they are about, and say either that a value could not be
// read or that a key was set twice.
func composeReadMessage(doc *yaml.Node, reported string) string {
	digits, rest, split := strings.Cut(strings.TrimPrefix(reported, "line "), ": ")
	line, err := strconv.Atoi(digits)
	if !split || err != nil {
		return "compose file: mozza could not read this file"
	}

	if spec, duplicate := strings.CutPrefix(rest, "mapping key "); duplicate {
		return composeDuplicateMessage(doc, line, spec)
	}

	if spec, shape := strings.CutPrefix(rest, "cannot unmarshal "); shape {
		return composeShapeMessage(doc, line, spec)
	}

	return fmt.Sprintf("compose file (line %d): mozza could not read this value", line)
}

// composeShapeMessage restates "cannot unmarshal !!str `web` into
// map[string]importer.composeService".
func composeShapeMessage(doc *yaml.Node, line int, spec string) string {
	const into = " into "

	cut := strings.LastIndex(spec, into)
	if cut < 0 {
		return fmt.Sprintf("compose file (line %d): mozza could not read this value", line)
	}

	tag, quoted, _ := strings.Cut(spec[:cut], " ")
	value := strings.Trim(quoted, "`")

	goType := spec[cut+len(into):]
	shape, collection := composeShapeOf(goType)
	target := composeTarget{line: line, kind: kindOfTag(tag), value: value, outermost: collection}

	field := "file"
	// The node yaml found says more about itself than the message does: it holds
	// the whole of a value yaml shortened, and its own tag.
	if found, at := composeField(doc, target, ""); found != "" {
		field, tag, value = found, at.ShortTag(), at.Value
	}

	if want, named := composeTypes[goType]; named {
		shape = want
	}

	return fmt.Sprintf("compose %s (line %d): expected %s, got %s",
		field, line, shape, composeGot(tag, value))
}

// composeDuplicateMessage restates `mapping key "image" already defined at line 3`.
func composeDuplicateMessage(doc *yaml.Node, line int, spec string) string {
	name, first, split := strings.Cut(spec, " already defined at line ")
	if !split {
		return fmt.Sprintf("compose file (line %d): mozza could not read this value", line)
	}

	field := "file"
	want := composeTarget{line: line, kind: yaml.ScalarNode, value: strings.Trim(name, `"`), outermost: true}
	if found, _ := composeField(doc, want, ""); found != "" {
		field = found
	}

	return fmt.Sprintf("compose %s (line %d): set twice, first at line %s", field, line, first)
}

// composeShapeOf describes a type composeTypes has no words for, from its shape
// alone — vague is survivable, a field name mozza guessed at is not — and
// reports whether that shape holds other values.
//
// The second answer settles which node a message is about when several share a
// line, as flow style allows and as a block mapping's first entry does
// routinely. Reading into a collection, the node yaml balked at is the field's
// own value — the outermost match, because anything deeper belongs to a
// structure mozza never looked inside. Reading into a single value, the node is
// a collection standing where a scalar belongs, and every block around it is a
// field mozza read quite happily, so there the innermost match is the one.
func composeShapeOf(goType string) (string, bool) {
	if strings.HasPrefix(goType, "map[") {
		return "a mapping", true
	}
	if strings.HasPrefix(goType, "[") {
		return "a list", true
	}

	return "a single value", false
}

// composeGot names what the compose file holds where the expected shape was.
func composeGot(tag, value string) string {
	switch tag {
	case seqTag:
		return "a list"
	case mapTag:
		return "a mapping"
	case nullTag:
		return "nothing"
	}

	if value == "" {
		return "an empty value"
	}

	return reportedRecipeValue(value)
}

// kindOfTag reports which kind of node a tag in one of yaml's messages belongs
// to. Only the two collection tags name a kind of their own; every other tag a
// compose file can carry is a scalar's.
func kindOfTag(tag string) yaml.Kind {
	if tag == seqTag {
		return yaml.SequenceNode
	}
	if tag == mapTag {
		return yaml.MappingNode
	}

	return yaml.ScalarNode
}

// composeTarget is the node one of yaml's messages is about: the line it names,
// enough of the node to tell it from a neighbour sharing that line, and which of
// them to take when that is not enough.
type composeTarget struct {
	value     string
	line      int
	kind      yaml.Kind
	outermost bool
}

// matches reports whether a node is the one the message is about. yaml shortens
// a value longer than ten characters, so only the part it kept is compared.
func (t composeTarget) matches(n *yaml.Node) bool {
	return n.Line == t.line && n.Kind == t.kind &&
		strings.HasPrefix(n.Value, strings.TrimSuffix(t.value, "..."))
}

// composeField names the field at a line the way compose's own validator names
// it — services.web.image — by finding the node yaml reported in the document it
// was reading. It returns that node too, because the node holds the value in
// full where the message holds only the start of it.
//
// A mapping's key is tried only after the whole of its own value, because block
// style puts a key on its value's line, so the key is the answer only when
// nothing else on that line is. A document or a sequence names no field of its
// own, so a match inside one answers to the field its parent gave it — and a
// match directly inside the document answers to no field at all.
func composeField(n *yaml.Node, want composeTarget, prefix string) (string, *yaml.Node) {
	if n.Kind != yaml.MappingNode {
		for _, item := range n.Content {
			if field, at := composeMatch(item, want, prefix); at != nil {
				return field, at
			}
		}

		return "", nil
	}

	for i := 0; i+1 < len(n.Content); i += 2 {
		key, value := n.Content[i], n.Content[i+1]

		field := composeJoin(prefix, key)

		if found, at := composeMatch(value, want, field); at != nil {
			return found, at
		}
		if want.matches(key) {
			return field, key
		}
	}

	return "", nil
}

// composeJoin adds one mapping key to a dotted field path. A key that is not a
// name — a list or a mapping standing where compose expects one — names no field
// of its own, so the path stays the one its parent gave it rather than trailing a
// dot with nothing after it.
func composeJoin(prefix string, key *yaml.Node) string {
	if key.Kind != yaml.ScalarNode || key.Value == "" {
		return prefix
	}
	if prefix == "" {
		return key.Value
	}

	return prefix + "." + key.Value
}

// composeMatch tries a node and everything under it, in whichever order the
// target asks for.
func composeMatch(n *yaml.Node, want composeTarget, field string) (string, *yaml.Node) {
	if want.outermost && want.matches(n) {
		return field, n
	}

	if found, at := composeField(n, want, field); at != nil {
		return found, at
	}

	if want.matches(n) {
		return field, n
	}

	return "", nil
}

// Warning represents a non-fatal issue found during compose import.
type Warning struct {
	Feature  string `json:"feature"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // "info", "warn", "error"
}

// knownEngines maps well-known Docker image prefixes to recipe engine shorthands.
var knownEngines = map[string]string{
	"postgres": "postgres",
	"redis":    "redis",
	"mysql":    "mysql",
	"mariadb":  "mysql",
}

// ComposeToRecipeAST converts docker-compose.yml content into a Recipe AST
// along with any warnings about unsupported features.
func ComposeToRecipeAST(yamlContent []byte) (*recipe.Recipe, []Warning, error) {
	var cf composeFile
	if err := yaml.Unmarshal(yamlContent, &cf); err != nil {
		return nil, nil, fmt.Errorf("parsing compose YAML: %w", composeReadError(yamlContent, err))
	}
	if len(cf.Services) == 0 {
		return nil, nil, fmt.Errorf("no services found in compose file")
	}

	names := sortedKeys(cf.Services)

	appName := names[0]
	if len(names) > 1 {
		appName = names[0] + "-stack"
	}

	r := &recipe.Recipe{
		Name:   appName,
		Slices: make([]recipe.Slice, 0, len(names)),
	}
	var warnings []Warning

	// Collect named volumes defined at top level.
	namedVolumes := make(map[string]bool, len(cf.Volumes))
	for v := range cf.Volumes {
		namedVolumes[v] = true
	}

	for _, name := range names {
		svc := cf.Services[name]
		slice, svcWarnings := convertService(name, svc, namedVolumes)
		r.Slices = append(r.Slices, slice)
		warnings = append(warnings, svcWarnings...)
	}

	// Warn about networks.
	warnings = append(warnings, networkWarnings(cf)...)
	warnings = append(warnings, convertNetworks(cf.Networks)...)

	// Warn about top-level configs. Nothing maps a config to anything: no
	// environment reference and no mount reach the recipe, and a service's own
	// configs are not read at all — so this is the only thing standing between a
	// declared config and its silent disappearance, and it has to say that rather
	// than describe a mapping that never happened.
	for cname := range cf.Configs {
		warnings = append(warnings, Warning{
			Feature: "configs",
			Message: fmt.Sprintf(
				"config %q dropped — compose configs are not carried into the recipe;"+
					" add `mount config \"<path>\" at <target>` to the slice that needs it",
				cname),
			Severity: "warn",
		})
	}

	return r, warnings, nil
}

// ComposeToRecipe converts docker-compose.yml content into a .mozza recipe string.
// This is the recipe `mozza import` writes to disk and the one the server deploys
// (cli/import.go, server/compose_handlers.go); ComposeToRecipeAST reads the same
// file into the AST every warning comes from.
//
// The two are separate implementations over one set of composeFile types, neither
// wrapping the other, so a capability added to one of these functions and not the
// other is a capability that works on one import path only. Networks are the one
// thing that cannot diverge here: neither writes a policy, and what a compose
// network keeps apart is reported by networkWarnings, on the AST path that carries
// every other warning too.
func ComposeToRecipe(composeYAML string) (string, error) {
	var cf composeFile
	content := []byte(composeYAML)
	if err := yaml.Unmarshal(content, &cf); err != nil {
		return "", fmt.Errorf("parsing compose YAML: %w", composeReadError(content, err))
	}
	if len(cf.Services) == 0 {
		return "", fmt.Errorf("no services found in compose file")
	}
	if err := checkComposeValues(cf); err != nil {
		return "", err
	}

	names := sortedKeys(cf.Services)

	appName := names[0]
	if len(names) > 1 {
		appName = names[0] + "-stack"
	}

	// Collect named volumes.
	namedVolumes := make(map[string]bool, len(cf.Volumes))
	for v := range cf.Volumes {
		namedVolumes[v] = true
	}

	var b strings.Builder
	fmt.Fprintf(&b, "App: %s\n", appName)

	for _, name := range names {
		svc := cf.Services[name]
		b.WriteString("\n")
		b.WriteString(capitalize(name) + ":\n")

		image := svc.Image
		if image == "" {
			if svc.Build != nil {
				b.WriteString("  # WARNING: build context detected, needs pre-built image\n")
			} else {
				b.WriteString("  # WARNING: no image specified, build required\n")
			}
			continue
		}

		// Check if this is a known engine (postgres, redis, mysql).
		engineName, version := parseEngineImage(image)
		if engineName != "" {
			line := "  " + engineName + " " + version
			if size := storageFor(engineName, svc.Volumes, namedVolumes); size != "" {
				line += ", " + size
			}
			b.WriteString(line + "\n")
		} else {
			fmt.Fprintf(&b, "  from image %s\n", image)
		}

		// Ports.
		publishedPort := 0
		for _, p := range svc.Ports {
			writePort(&b, p)
			if n, _ := parsePort(p); publishedPort == 0 && n > 0 {
				publishedPort = n
			}
		}

		// Health check.
		if svc.Healthcheck != nil {
			writeHealthcheck(&b, svc.Healthcheck, publishedPort)
		}

		// Replicas.
		if svc.Deploy != nil && svc.Deploy.Replicas > 1 {
			fmt.Fprintf(&b, "  run %d copies\n", svc.Deploy.Replicas)
		}

		// Restart policy.
		if svc.Restart != "" && svc.Restart != "no" {
			fmt.Fprintf(&b, "  restart %s\n", svc.Restart)
		}

		// depends_on -> needs.
		deps := parseDependsOn(svc.DependsOn)
		if len(deps) > 0 {
			fmt.Fprintf(&b, "  needs %s\n", strings.Join(deps, " and "))
		}

		// Named volumes.
		writeVolumes(&b, engineName, svc.Volumes, namedVolumes)

		// Environment variables.
		envs := parseEnvironment(svc.Environment)
		envKeys := sortedKeys(envs)
		for _, k := range envKeys {
			fmt.Fprintf(&b, "  set %s to \"%s\"\n", k, envs[k])
		}

		// Secrets.
		for _, s := range svc.Secrets {
			fmt.Fprintf(&b, "  secret %s from %s\n",
				strings.ToUpper(strings.ReplaceAll(s.Source, "-", "_")),
				s.Source)
		}
	}

	return b.String(), nil
}

// checkComposeValues refuses a compose file holding a value that cannot be
// written into recipe text without changing what the recipe says.
//
// It runs once, over the whole file, before a line is written — several of the
// emitters below take a bare *strings.Builder, and threading an error back out
// of each of them buys the same guarantee for a much larger change. Every value
// is checked in the shape it will be emitted in, through the same derivation
// helpers the emitters use, because the two differ often enough to matter: a
// healthcheck's test command legitimately holds quotes and spaces
// (`curl -f 'http://...'`), and only the path extracted from it is ever written.
func checkComposeValues(cf composeFile) error {
	namedVolumes := make(map[string]bool, len(cf.Volumes))
	for v := range cf.Volumes {
		namedVolumes[v] = true
	}

	// Sorted so a file with two bad values always names the same one first.
	for _, name := range sortedKeys(cf.Services) {
		if err := checkComposeService(name, cf.Services[name], namedVolumes); err != nil {
			return err
		}
	}

	return nil
}

// recipeValueCheck is one value bound to the predicate for the position it is
// written into. Building the list before running it keeps which class governs
// which field the visible content of checkComposeService, rather than something
// spread across a dozen error returns.
type recipeValueCheck struct {
	safe  func(field, value string) error
	field string
	value string
}

// checkComposeService refuses a single service's unwritable values. The checks
// are built in the order ComposeToRecipe emits them, so a file with several bad
// values names the one that would have gone wrong first.
func checkComposeService(name string, svc composeService, namedVolumes map[string]bool) error {
	// The name is checked before the rest because it is both a value in its own
	// right — it becomes the App line and the section header — and the prefix
	// every field path below is built from, which a newline in it would corrupt.
	if err := recipeSafeWord("services", name); err != nil {
		return err
	}
	field := "services." + name

	// The image is written whole by `from image`; an engine shorthand's version
	// is a substring of it, so checking the image covers both.
	checks := []recipeValueCheck{{recipeSafeWord, field + ".image", svc.Image}}

	for _, p := range svc.Ports {
		port, _ := portDirective(p)
		checks = append(checks, recipeValueCheck{recipeSafeWord, field + ".ports", port})
	}

	if svc.Healthcheck != nil {
		// The path is written as a bare word when the check is kept and into a
		// comment when it is dropped, and which of those happens turns on the
		// service's ports rather than on the path — so it answers to the
		// stricter of the two positions either way.
		path, _ := extractHealthCheck(svc.Healthcheck)
		checks = append(checks, recipeValueCheck{recipeSafeWord, field + ".healthcheck.test", path})
	}

	checks = append(checks, recipeValueCheck{recipeSafeWord, field + ".restart", svc.Restart})

	for _, dep := range parseDependsOn(svc.DependsOn) {
		checks = append(checks, recipeValueCheck{recipeSafeWord, field + ".depends_on", dep})
	}

	checks = append(checks, composeVolumeChecks(field, svc.Volumes, namedVolumes)...)

	envs := parseEnvironment(svc.Environment)
	for _, k := range sortedKeys(envs) {
		// The key is written unquoted and the value between quotes, so a value
		// may hold the spaces a sentence needs and the key may not.
		checks = append(checks,
			recipeValueCheck{recipeSafeWord, field + ".environment", k},
			recipeValueCheck{recipeSafeString, field + ".environment." + k, envs[k]})
	}

	for _, s := range svc.Secrets {
		// Written twice on one line, uppercased and raw; neither spelling adds
		// a byte the other does not have.
		checks = append(checks, recipeValueCheck{recipeSafeWord, field + ".secrets", s.Source})
	}

	for _, c := range checks {
		if err := c.safe(c.field, c.value); err != nil {
			return err
		}
	}

	return nil
}

// composeVolumeChecks returns the checks for a service's mounts. Only a named
// volume is checked, because writeVolumes writes nothing at all for any other
// kind of mount — a bind mount's host path never reaches the recipe, and
// rejecting a space in one would fail imports that work today.
func composeVolumeChecks(field string, volumes []string, namedVolumes map[string]bool) []recipeValueCheck {
	checks := make([]recipeValueCheck, 0, len(volumes)*2)

	for _, v := range volumes {
		volName, mountPath := splitVolume(v)
		if !namedVolumes[volName] {
			continue
		}

		// Both land in a comment, which only a line ending can escape.
		checks = append(checks,
			recipeValueCheck{recipeSafeComment, field + ".volumes", volName},
			recipeValueCheck{recipeSafeComment, field + ".volumes", mountPath})
	}

	return checks
}

// convertService converts a single compose service to a recipe Slice.
func convertService(
	name string,
	svc composeService,
	namedVolumes map[string]bool,
) (recipe.Slice, []Warning) {
	var warnings []Warning

	sliceName := capitalize(name)
	slice := recipe.Slice{Name: sliceName}

	// Image / build.
	if svc.Image == "" {
		if svc.Build != nil {
			warnings = append(warnings, Warning{
				Feature:  "build",
				Message:  fmt.Sprintf("service %q uses build context — needs a pre-built image", name),
				Severity: "warn",
			})
		}
		return slice, warnings
	}

	engineName, version := parseEngineImage(svc.Image)
	if engineName != "" {
		slice.Engine = engineName
		slice.Version = version
		slice.Storage = storageFor(engineName, svc.Volumes, namedVolumes)
	} else {
		slice.Image = svc.Image
	}

	// Ports.
	for _, p := range svc.Ports {
		port, public := parsePort(p)
		if port > 0 {
			if slice.Port == 0 {
				slice.Port = port
			}
			if public {
				slice.Public = true
			}
		}
	}

	// Health check. The healthcheck URL carries the port the service really
	// listens on, so it can supply one when no published port did. Without a port
	// the check cannot be aimed, and importing it anyway would write a recipe that
	// mozza validate rejects — so the check is dropped and the user told why.
	if svc.Healthcheck != nil {
		path, urlPort := extractHealthCheck(svc.Healthcheck)
		if slice.Port == 0 {
			slice.Port = urlPort
		}
		switch {
		case path == "":
			// Not an HTTP health check — there is nothing to carry into the recipe.
		case slice.Port == 0:
			warnings = append(warnings, Warning{
				Feature: "healthcheck",
				Message: fmt.Sprintf(
					"service %q health check %s dropped — no published port and none in the check URL; publish the port in compose and import again",
					name, path),
				Severity: "warn",
			})
		default:
			slice.Health = path
		}
	}

	// Replicas.
	if svc.Deploy != nil && svc.Deploy.Replicas > 1 {
		slice.Replicas = svc.Deploy.Replicas
	}

	// Restart policy.
	if svc.Restart != "" && svc.Restart != "no" {
		slice.RestartPolicy = svc.Restart
	}

	// depends_on.
	slice.Needs = parseDependsOn(svc.DependsOn)

	// Named volumes. The size a volume implies is already on the engine shorthand
	// (storageFor, above), the only storage form the importer emits — so an
	// imported service gets one volume at most, at its engine's own path, and
	// everything compose declared beyond that is said aloud rather than left to be
	// found as missing persistence in a running cluster. The engine-less warning
	// below names the per-copy storage directive a reader can add by hand.
	//
	// No Mounts entry goes with any of it. A recipe's mounts name an object that
	// already exists — a pvc mount compiles to a claimName taken unchanged from the
	// source (k8s/deployment.go addMounts) — and mozza creates no claim under a
	// compose volume's name, so the pod would wait on one that is never made. That
	// is why the text builder writes a comment here and why this path must leave
	// the same nothing behind it: the two are one import, and a mount on one of
	// them is a volume the other never asked for.
	for _, v := range svc.Volumes {
		volName, mountPath := splitVolume(v)
		if !namedVolumes[volName] {
			continue
		}

		if mountIsCarried(slice.Engine, mountPath) {
			continue
		}

		at := ""
		if mountPath != "" {
			at = " at " + mountPath
		}

		var message string
		if slice.Engine == "" {
			message = fmt.Sprintf(
				"service %q volume %s%s dropped — this service has no database or cache engine to carry it; add \"each copy needs its own storage of 10Gi\" to deploy the service as a StatefulSet with a 10Gi volume per copy, mounted at %s",
				name, volName, at, statefulLanding(mountPath))
		} else {
			message = fmt.Sprintf(
				"service %q volume %s%s not carried — a %s slice gets one volume, mounted at %s",
				name, volName, at, slice.Engine, engineMountPaths[slice.Engine])
		}

		warnings = append(warnings, Warning{
			Feature:  "volumes",
			Message:  message,
			Severity: "warn",
		})
	}

	// Environment.
	envs := parseEnvironment(svc.Environment)
	if len(envs) > 0 {
		slice.Env = envs
	}

	// Secrets.
	for _, s := range svc.Secrets {
		envVar := strings.ToUpper(strings.ReplaceAll(s.Source, "-", "_"))
		slice.Secrets = append(slice.Secrets, recipe.SecretRef{
			EnvVar:     envVar,
			SecretName: s.Source,
			Key:        s.Source,
		})
	}

	// Networks are deliberately not read here. Nothing a compose network says
	// reaches the recipe: networkWarnings reports the boundary rather than carrying
	// it, and it does that over the whole file, because who else is on a service's
	// networks is not a question that can be asked from inside one service.

	return slice, warnings
}

// convertNetworks produces warnings for compose networks that need attention.
func convertNetworks(networks map[string]composeNetworkConfig) []Warning {
	var warnings []Warning
	for name, net := range networks {
		if net.Internal {
			warnings = append(warnings, Warning{
				Feature: "networks",
				Message: fmt.Sprintf(
					"internal network %q — compose keeps its members from reaching anything outside it;"+
						" a recipe's network policy is ingress-only, so that outbound restriction is not carried",
					name),
				Severity: "info",
			})
		}
	}
	return warnings
}

// defaultNetwork is the network compose puts a service on when it names none.
const defaultNetwork = "default"

// explicitNetworks names the networks a service joins that draw a boundary at all,
// which is every network it names except the default one.
//
// Compose's default network needs no declaring and no joining: a service that says
// nothing lands there, and one that writes `networks: [default]` lands in exactly
// the same place. So naming it is not a request to be isolated, and a boundary
// derived as if it were would exclude every service that never wrote the word —
// which compose lets straight through.
func explicitNetworks(svc composeService) []string {
	nets := parseServiceNetworks(svc.Networks)
	explicit := make([]string, 0, len(nets))
	for _, net := range nets {
		if net != defaultNetwork {
			explicit = append(explicit, net)
		}
	}

	return explicit
}

// joinedNetworks names the explicit networks a service joins, in name order and
// each of them once. The list form lets a service name the same network twice; the
// mapping form cannot, and the two spellings have to describe the same file.
func joinedNetworks(svc composeService) []string {
	nets := explicitNetworks(svc)
	sort.Strings(nets)

	return slices.Compact(nets)
}

// joinsDefault reports whether a service names the default network among the ones
// it joins, which explicitNetworks drops and the warnings have to know about.
func joinsDefault(svc composeService) bool {
	return slices.Contains(parseServiceNetworks(svc.Networks), defaultNetwork)
}

// networkMembers maps each explicit network the file's services join to the
// services this file declares on it, in name order.
//
// This is what the `networks:` keys say and nothing more. A service can be on a
// network without naming its key — through `extends`, through `include`, through
// `network_mode`, through a `${VAR}` mozza does not expand, through an override
// file compose loads and mozza is never handed, or because a second key's `name:`
// resolved to the same real network — so a membership read here is a lower bound.
// That is enough to report a boundary and not enough to write one, which is why
// nothing derived from it reaches the recipe.
func networkMembers(cf composeFile) map[string][]string {
	members := map[string][]string{}
	for _, name := range sortedKeys(cf.Services) {
		for _, net := range joinedNetworks(cf.Services[name]) {
			members[net] = append(members[net], name)
		}
	}

	return members
}

// networkWarnings reports every boundary this compose file draws with its networks
// and that the recipe does not carry.
//
// No compose file produces a network policy. The two vocabularies are not the same
// thing: a compose network says who may reach whom, while a recipe's `only accept
// traffic from` is a directed ingress allow-list whose *presence* is a deny — the
// compiled policy admits the components it names and nothing else
// (k8s/networkpolicy.go). So a derived policy is correct only when its list is every
// source compose already allows, and membership read off the `networks:` keys is a
// lower bound rather than that set. Deriving one anyway got the peer set wrong on
// eight distinct compose shapes in a row, and every miss is the same outcome: a
// policy that silently black-holes a service while the deploy reports success.
//
// So the boundary is named rather than carried — which network, which services this
// file puts on it, that mozza has not carried it, and the directive that writes it by
// hand. Naming the declared members is true however incomplete the file is; claiming
// they are all of them is what could not be.
//
// One warning per network, because the network is the boundary — then one per
// service on any of them, because the remedy is not. A file with one service is
// silent: its networks keep out nothing that exists here.
func networkWarnings(cf composeFile) []Warning {
	if len(cf.Services) < 2 {
		return nil
	}

	members := networkMembers(cf)

	warnings := make([]Warning, 0, len(members)+len(cf.Services))
	for _, net := range sortedKeys(members) {
		warnings = append(warnings, boundaryWarning(cf, net, members[net]))
	}

	for _, name := range sortedKeys(cf.Services) {
		if len(joinedNetworks(cf.Services[name])) > 0 {
			warnings = append(warnings, remedyWarning(cf, name, members))
		}
	}

	return warnings
}

// boundaryWarning says what one network keeps apart and that the recipe keeps none
// of it. What to write instead belongs to the service rather than the network, and
// is remedyWarning's.
func boundaryWarning(cf composeFile, net string, members []string) Warning {
	subject, keeps := "service", "compose keeps out everything that did not join"
	if len(members) > 1 {
		subject = "services"
		keeps = "compose lets them reach each other and keeps out everything that did not join"
	}

	message := fmt.Sprintf(
		"network %s draws a boundary the recipe does not carry — this file declares %s %s on it,"+
			" and %s", named(net), subject, namedAnd(members), keeps)

	if why, short := unreadMembers(cf, net, members); short {
		message += fmt.Sprintf("; %s, so the services named here may not be all of them", why)
	}

	return Warning{Feature: "networks", Message: message, Severity: "warn"}
}

// remedyWarning is the instruction for one service: the single directive that
// carries every network it joins, or why no directive is recommended for it.
//
// The remedy is grouped by service and not by network, and that is the whole of why
// it exists as its own warning. A slice carries one `only accept traffic from`, and
// the presence of that directive is a deny — so a service on two networks does not
// get a directive per network, it gets one whose list is the union of both. Reported
// per network, the two lists arrive as two instructions that each look complete, and
// following either on its own writes a policy narrower than compose: validate
// passes, the deploy reports success, and the traffic the other list carried is gone
// with nothing anywhere to read. Grouped per service, there is one instruction and
// it can be followed on its own.
//
// A list wider than compose allows is a weaker boundary; a narrower one is an
// outage. So where the declared membership may be short the list is still every peer
// this file records, and the warning says what the directive would leave out.
func remedyWarning(cf composeFile, service string, members map[string][]string) Warning {
	nets := joinedNetworks(cf.Services[service])
	message := fmt.Sprintf("service %s is on %s", named(service), namedAnd(nets))

	// A slice the compiler gives an Ingress must not carry the directive. The ingress
	// controller joins no compose network, so it is in no allow-list read off one, and
	// a policy under such a slice denies it: the pods stay up and the route stays in
	// place while the site goes dark, with nothing anywhere to read. The predicate is
	// the compiler's own ingress guard (k8s/compiler.go) less the conditions an import
	// cannot fail — no compose file names a domain, and a published port is what
	// convertService reads as Public. It is said out loud, because a service passed
	// over in silence is one the reader has to notice the absence of.
	if publishesPort(cf.Services[service]) {
		message += fmt.Sprintf(
			" and publishes a port, so its slice is reachable from outside the cluster and a"+
				" network policy under %s would deny the ingress that serves it — no directive"+
				" is recommended", capitalize(service))

		return Warning{Feature: "networks", Message: message, Severity: "warn"}
	}

	// A key holding a ${VAR} is not a network mozza can name — compose expands it
	// before it reads the project and mozza expands nothing — so the members under it
	// are whoever happened to write the same unexpanded token. Where that is every
	// network a service joins, there is no membership to draw a list from at all.
	// Where it is only some of them, the others are networks with members this file
	// really does declare, and the list drawn from them is true and possibly short —
	// which is what the caveat below is for.
	var short []string
	unnamed := 0
	for _, net := range nets {
		if interpolated(net) {
			unnamed++
		}
		if _, incomplete := unreadMembers(cf, net, members[net]); incomplete {
			short = append(short, net)
		}
	}

	if unnamed == len(nets) {
		message += " — no directive is recommended, because every network it joins is named by a" +
			" `${VAR}` mozza does not expand, and a list drawn from a network mozza cannot name is" +
			" not one to follow"

		return Warning{Feature: "networks", Message: message, Severity: "warn"}
	}

	boundaries := "that boundary"
	if len(nets) > 1 {
		boundaries = "those boundaries"
	}

	message += fmt.Sprintf(
		" — to carry %s by hand, add `only accept traffic from %s` under %s, which is every"+
			" service this file declares on a network with it",
		boundaries, quotedAnd(peers(nets, members)), capitalize(service))

	if len(short) > 0 {
		message += fmt.Sprintf("; %s may have members this file does not declare, and the"+
			" directive denies anything it leaves out", namedAnd(short))
	}

	return Warning{Feature: "networks", Message: message, Severity: "warn"}
}

// peers names every slice the directive for a service on these networks must admit:
// each service this file declares on any of them, the subject included.
//
// The subject is in its own list on purpose. Compose lets a service reach its own
// replicas over a shared network, and mozza gives a multi-copy stateful slice a
// headless service so its pods can find each other — a directive that left the slice
// off would deny the traffic that service exists to route.
func peers(nets []string, members map[string][]string) []string {
	seen := map[string]bool{}
	var services []string
	for _, net := range nets {
		for _, name := range members[net] {
			if !seen[name] {
				seen[name] = true
				services = append(services, name)
			}
		}
	}
	sort.Strings(services)

	refs := make([]string, 0, len(services))
	for _, name := range services {
		refs = append(refs, sliceRef(name))
	}

	return refs
}

// unreadMembers names something that can put a service on this network without a
// `networks:` key recording it, or reports that nothing in the file does.
//
// Nothing here decides what is emitted — no compose file produces a network policy —
// so a construct missed costs a sentence rather than a service that silently goes
// dark. It is the remedy that needs it: a reader who writes the directive from the
// names above denies whatever these bring in, so where the declared membership may be
// short, the warning says so before recommending it.
//
// Order picks which of several true reasons the reader gets, and never whether one is
// given: this network's own declaration, then a service on it, then the file.
func unreadMembers(cf composeFile, net string, members []string) (string, bool) {
	// The drivers are enumerated the safe way round. `bridge` and `overlay` make a
	// segment of compose's own; everything else either bridges to a segment that
	// already exists — macvlan and ipvlan put a container straight onto the physical
	// network — or is a third-party plugin mozza has no basis to reason about.
	cfg := cf.Networks[net]
	switch {
	case interpolated(net):
		return "this network's name holds a `${VAR}` mozza does not expand, and this may be one of" +
			" the other networks declared here", true

	case cfg.External:
		return "this network is declared `external:`, and its other members live outside this" +
			" compose file", true

	case cfg.Attachable:
		return "this network is declared `attachable:`, and containers compose did not start may" +
			" join it", true

	case !composeCreates(cfg.Driver):
		return fmt.Sprintf("this network's %s driver puts it on a segment compose does not create",
			named(cfg.Driver)), true

	// A network's `name:` is the network compose actually creates, and nothing stops
	// a second key from resolving to the same one — so the services on a network are
	// not necessarily the ones written under the key it is reached by. Last of the
	// per-network reasons: the three above say the members are not only compose's,
	// which is the larger fact when both hold.
	case cfg.Name != "":
		return "this network sets a `name:` of its own, which is the network compose actually" +
			" creates, and a second key can resolve to that same network", true
	}

	// Also on the default network, where compose puts every service that named none —
	// so the services reaching this one are not only the ones under this key.
	for _, name := range members {
		if joinsDefault(cf.Services[name]) {
			return fmt.Sprintf("service %s also joins the default network, where compose puts every"+
				" service that declares no networks of its own", named(name)), true
		}
	}

	return unreadFile(cf)
}

// unreadFile names a construct anywhere in the file through which a service reaches
// another over a network no `networks:` key records, or reports that the file holds
// none. Each of these leaves every membership in the file possibly short, not only
// the one belonging to the service carrying it.
func unreadFile(cf composeFile) (string, bool) {
	if cf.Include != nil {
		return "this file pulls services in with `include`, which mozza does not resolve", true
	}

	for _, name := range sortedKeys(cf.Services) {
		svc := cf.Services[name]

		switch {
		case svc.Extends != nil:
			return fmt.Sprintf("service %s takes its configuration from another with `extends`,"+
				" whose networks mozza does not read", named(name)), true

		// Every value but `none` reaches, because the set that does is open: the
		// compose specification calls the available values platform-specific, and
		// beyond `host` and the `service:` and `container:` forms a bare network name
		// is accepted too and joins that network outright. Measured against docker
		// 29.4.0, `none` is the only value that reaches nothing — even `bridge`
		// reaches a service on a user-defined network at its address.
		case svc.NetworkMode != "" && svc.NetworkMode != "none":
			return fmt.Sprintf("service %s takes its network namespace from `network_mode`, and"+
				" reaches services over networks it never joined by name", named(name)), true

		case svc.ExternalLinks != nil:
			return fmt.Sprintf("service %s declares `external_links`, which reach containers this"+
				" file does not define over a network shared with them", named(name)), true

		case interpolatedNetwork(svc):
			return fmt.Sprintf("service %s joins a network whose name holds a `${VAR}` mozza does"+
				" not expand, and it may be this one", named(name)), true
		}
	}

	// Some other key names the network it creates, and a key that names none still
	// resolves to <project>_<key> — which that one's `name:` is free to be. Telling
	// the two that collide from the two that do not needs the project name, which
	// comes from the directory the file sits in, and the importer is handed content
	// rather than a path.
	if renamed := renamedNetwork(cf); renamed != "" {
		return fmt.Sprintf("network %s sets a `name:` of its own, which is the network compose"+
			" actually creates, and a second key can resolve to that same network",
			named(renamed)), true
	}

	return "", false
}

// composeCreates reports whether a network driver makes a segment holding only what
// compose put on it.
func composeCreates(driver string) bool {
	return driver == "" || driver == "bridge" || driver == "overlay"
}

// interpolated reports whether a network name is one compose would have expanded and
// mozza did not, so the key read here is a literal token rather than the network it
// stands for. Mozza expands ${VAR} nowhere, in any field.
func interpolated(net string) bool {
	return strings.Contains(net, "${")
}

// interpolatedNetwork reports whether a service names any of its networks that way,
// which puts it on a network none of the file's other keys can be held to.
func interpolatedNetwork(svc composeService) bool {
	return slices.ContainsFunc(parseServiceNetworks(svc.Networks), interpolated)
}

// named quotes a compose key the way a sentence quotes one.
//
// Not strconv.Quote: that escapes for Go's lexer, so a network key holding a quote
// character comes back as `net \"with\" quotes` — a backslash the reader has no use
// for in prose that is not a directive. quotedAnd keeps strconv.Quote, because there
// the escaping is what makes the directive parse.
func named(key string) string {
	return `"` + key + `"`
}

// namedAnd renders compose keys as a sentence lists them, joined the way the
// grammar's own lists are so a name reads the same in prose as in the directive.
func namedAnd(keys []string) string {
	quoted := make([]string, 0, len(keys))
	for _, key := range keys {
		quoted = append(quoted, named(key))
	}

	return strings.Join(quoted, " and ")
}

// renamedNetwork names the first network the file declares that carries a `name:`
// of its own, or "" when none does. An undeclared network has no config and so no
// name, which is right: compose rejects a file naming one.
func renamedNetwork(cf composeFile) string {
	for _, net := range sortedKeys(cf.Networks) {
		if cf.Networks[net].Name != "" {
			return net
		}
	}

	return ""
}

// publishesPort reports whether compose publishes one of a service's ports to the
// host, which is what convertService reads as Public and the compiler turns into
// an Ingress. Asked of the compose service rather than the slice it becomes so
// both entry points get the same answer from the same input.
func publishesPort(svc composeService) bool {
	for _, p := range svc.Ports {
		if port, public := parsePort(p); port > 0 && public {
			return true
		}
	}

	return false
}

// sliceRef is the name a directive must use to name the slice a compose service
// becomes. capitalize gives the section header, plan.Build lowercases that
// (plan/build.go convertSlice) and plan.Validate compares what a policy names
// against the result case-sensitively — so the name to write is the lowercased
// header rather than the compose service name. The two differ for a service named
// after a reserved word, where "app" heads a section called "App-svc" and is
// referred to as "app-svc".
func sliceRef(service string) string {
	return strings.ToLower(capitalize(service))
}

// quotedAnd renders names as the recipe grammar's own list: quoted, joined by
// "and".
//
// Every name is quoted, always. The grammar takes an identifier or a string
// (recipe/parser.go parseIdentDirective), and an unquoted name that collides with
// a keyword lexes as that keyword instead — public, secret, image, port, copies,
// domain, health, from, and, traffic and accept all do, and the point of quoting
// unconditionally is that the list of which ones is not worth keeping accurate.
func quotedAnd(names []string) string {
	quoted := make([]string, 0, len(names))
	for _, name := range names {
		quoted = append(quoted, strconv.Quote(name))
	}

	return strings.Join(quoted, " and ")
}

// parseServiceNetworks extracts network names from the service networks field.
func parseServiceNetworks(raw interface{}) []string {
	if raw == nil {
		return nil
	}
	switch v := raw.(type) {
	case []interface{}:
		nets := make([]string, 0, len(v))
		for _, n := range v {
			if s, ok := n.(string); ok {
				nets = append(nets, s)
			}
		}
		return nets
	case map[string]interface{}:
		nets := make([]string, 0, len(v))
		for name := range v {
			nets = append(nets, name)
		}
		sort.Strings(nets)
		return nets
	}
	return nil
}

// parsePort extracts the container port and whether it is public from a port spec.
func parsePort(portSpec string) (int, bool) {
	portSpec = strings.TrimSpace(portSpec)
	portSpec = strings.Split(portSpec, "/")[0] // strip protocol

	if strings.Contains(portSpec, ":") {
		parts := strings.SplitN(portSpec, ":", 2)
		port := parsePortNumber(parts[1])
		return port, true
	}
	return parsePortNumber(portSpec), false
}

// parsePortNumber converts a port string to int, returning 0 on failure.
func parsePortNumber(s string) int {
	var port int
	for _, ch := range s {
		if ch < '0' || ch > '9' {
			break
		}
		port = port*10 + int(ch-'0')
	}
	return port
}

// splitVolume splits one compose short-syntax volume — [SOURCE:]TARGET[:MODE] —
// into its source and the container path it asks for.
//
// Splitting on every colon and keeping the first two fields leaves the MODE
// behind. A mode — "ro", "z", "cached", or a comma-separated run of them — says
// how the container sees the volume, never where; read as part of the path it
// would put "/var/lib/mysql:rw" in front of a reader as though it were a
// directory, and have the importer call the mount uncarried for not matching
// "/var/lib/mysql".
//
// A Windows source keeps its drive letter, so a bind mount from a Windows host
// is not read as a volume named "C".
//
// An anonymous volume that carries a mode and no source — "/data:ro" — reads as
// a source of "/data" and a target of "ro". No caller acts on it: a compose
// volume name cannot contain "/", so that source matches no declared volume.
func splitVolume(v string) (string, string) {
	fields := strings.Split(v, ":")
	if len(fields) > 1 && len(fields[0]) == 1 && strings.HasPrefix(fields[1], `\`) {
		fields = append([]string{fields[0] + ":" + fields[1]}, fields[2:]...)
	}

	if len(fields) > 1 {
		return fields[0], fields[1]
	}

	return fields[0], ""
}

// storageFor reports the size a service's engine shorthand should carry, or ""
// when it should carry none. The shorthand — "postgres 16, 10Gi" — is the only
// storage form the importer emits; the grammar's other one, "each copy needs its
// own storage of 10Gi", is left for the reader to add, which is what a dropped
// volume's warning asks them to do. Both import paths take their size from this
// function so they cannot come to different answers about the same service.
func storageFor(engine string, volumes []string, namedVolumes map[string]bool) string {
	switch engine {
	case "postgres", "mysql":
		return "10Gi"
	case "redis":
		// A cache is ephemeral unless compose gave it a volume, and asking for
		// storage it was never meant to have would provision a stray PVC.
		for _, v := range volumes {
			if volName, _ := splitVolume(v); namedVolumes[volName] {
				return "10Gi"
			}
		}
	}
	return ""
}

// engineMountPaths is where the compiler really mounts an engine slice's volume,
// mirroring the mount path plan.Build reads out of its own engine defaults. It
// takes no notice of the path compose asked for, so the two agree only by
// coincidence — TestEngineMountPathsMatchWhereTheCompilerMounts holds this table
// against the plan the compiler actually builds.
var engineMountPaths = map[string]string{ //nolint:gochecknoglobals // immutable lookup table
	"postgres": "/var/lib/postgresql/data",
	"mysql":    "/var/lib/mysql",
	"redis":    "/data",
}

// mountIsCarried reports whether the compiler will really mount a volume at the
// path compose named. A slice's storage is one PVC at the engine's own path, so
// a compose mount anywhere else — or on a slice with no engine to supply a path
// — is not carried, however plainly the compose file asked for it.
func mountIsCarried(engine, mountPath string) bool {
	canonical, known := engineMountPaths[engine]

	return known && mountPath == canonical
}

// statefulLanding names where "each copy needs its own storage" puts a volume,
// and the compose path it displaces. A slice with no engine has no engine path
// for the compiler to use, so that volume lands at /data whatever compose asked
// for — advice that did not say so would leave the app writing to a directory
// nothing is mounted on.
func statefulLanding(composePath string) string {
	if composePath == "" || composePath == "/data" {
		return "/data"
	}

	return "/data rather than " + composePath
}

// writeVolumes writes what a service's named volumes can carry into the recipe.
// A slice carries one volume at most — an engine slice's rides on its shorthand's
// size and lands at the engine's own path, and a slice with no engine has no
// shorthand for the importer to write a size on, only the per-copy directive the
// warning below asks the reader to add — so everything compose declared beyond
// that is named as uncarried instead of vanishing between the compose file and
// the cluster.
func writeVolumes(b *strings.Builder, engine string, volumes []string, namedVolumes map[string]bool) {
	for _, v := range volumes {
		volName, mountPath := splitVolume(v)
		if !namedVolumes[volName] {
			continue
		}

		at := ""
		if mountPath != "" {
			at = " at " + mountPath
		}

		switch {
		case mountIsCarried(engine, mountPath):
			fmt.Fprintf(b, "  # mount %s at %s\n", volName, mountPath)
		case engine != "":
			fmt.Fprintf(b,
				"  # WARNING: volume %s%s not carried — a %s slice gets one volume, mounted at %s\n",
				volName, at, engine, engineMountPaths[engine])
		default:
			// The remedy belongs in the file, as it does for a dropped health
			// check: this recipe outlives the import output, and the reader of it
			// is no longer looking at compose.
			fmt.Fprintf(b,
				"  # WARNING: volume %s%s dropped — this slice has no database or cache engine to carry it; add \"each copy needs its own storage of 10Gi\" to get a 10Gi volume per copy, mounted at %s\n",
				volName, at, statefulLanding(mountPath))
		}
	}
}

// parseEngineImage checks if an image string is a known database/cache engine
// and returns the engine name and version. Returns ("", "") if not a known engine.
func parseEngineImage(image string) (engine, version string) {
	// Split image into name and tag: "postgres:16" -> ("postgres", "16").
	parts := strings.SplitN(image, ":", 2)
	imageName := parts[0]
	tag := "latest"
	if len(parts) == 2 {
		tag = parts[1]
	}

	// Strip registry prefix if present (e.g., "docker.io/library/postgres").
	segments := strings.Split(imageName, "/")
	baseName := segments[len(segments)-1]

	eng, ok := knownEngines[baseName]
	if !ok {
		return "", ""
	}

	// Use the tag as the version. Strip Alpine/bookworm/etc. suffixes.
	version = tag
	for _, suffix := range []string{"-alpine", "-bookworm", "-bullseye", "-slim", "-jammy"} {
		version = strings.TrimSuffix(version, suffix)
	}
	if version == "latest" || version == "" {
		switch eng {
		case "postgres":
			version = "16"
		case "redis":
			version = "7"
		case "mysql":
			version = "8"
		}
	}

	return eng, version
}

// portDirective reduces a compose port spec to the port a recipe directive
// names, and reports whether compose published it. Formats: "8080:80", "8080",
// "8080:80/udp".
//
// It is separate from writePort because checkComposeValues has to check the
// value that reaches the recipe rather than the spec it was derived from, and a
// second copy of this derivation is a second answer to the same question.
func portDirective(portSpec string) (string, bool) {
	portSpec = strings.TrimSpace(portSpec)

	// Strip protocol suffix for now.
	portSpec = strings.Split(portSpec, "/")[0]

	if strings.Contains(portSpec, ":") {
		return strings.SplitN(portSpec, ":", 2)[1], true
	}

	return portSpec, false
}

// writePort writes a port directive to the builder.
func writePort(b *strings.Builder, portSpec string) {
	port, published := portDirective(portSpec)
	if published {
		fmt.Fprintf(b, "  open to the public on port %s\n", port)
	} else {
		fmt.Fprintf(b, "  on port %s\n", port)
	}
}

// writeHealthcheck writes a health check directive, preceded by the port the check
// needs when no published port has already supplied one. publishedPort is the
// container port already written for this service, or 0 when there was none.
func writeHealthcheck(b *strings.Builder, hc *composeHealthcheck, publishedPort int) {
	path, urlPort := extractHealthCheck(hc)
	if path == "" {
		return
	}

	if publishedPort == 0 && urlPort == 0 {
		// mozza validate rejects a health check with no port to aim at, so explain
		// the gap in the recipe rather than write a line that cannot compile. The
		// remedy belongs here too: this file outlives the import output, and by now
		// the reader is looking at a recipe, not at compose.
		fmt.Fprintf(b,
			"  # WARNING: health check %s dropped — no published port and none in the check URL; add \"on port <n>\", then restore \"health check %s\"\n",
			path, path)

		return
	}

	if publishedPort == 0 {
		fmt.Fprintf(b, "  on port %d\n", urlPort)
	}
	fmt.Fprintf(b, "  health check %s\n", path)
}

// extractHealthCheck extracts the HTTP path and the target port from a compose
// healthcheck command such as ["CMD", "curl", "-f", "http://localhost:3000/health"].
// It returns ("", 0) when no part of the command holds a localhost URL with a path.
//
// The port matters as much as the path: a recipe's health check inherits the
// slice's port, so a check imported without one cannot be aimed and `mozza
// validate` rejects it. A zero port means the URL named none — callers decide what
// to do about it, because the importer will not invent one.
func extractHealthCheck(hc *composeHealthcheck) (string, int) {
	for _, part := range hc.Test {
		_, rest, found := strings.Cut(part, "http://localhost")
		if !found {
			continue
		}

		// rest is ":3000/health || exit 1", or "/health" when the URL omits the
		// port, so a port is present only directly after the colon.
		port := 0
		if afterColon, hasPort := strings.CutPrefix(rest, ":"); hasPort {
			port = parsePortNumber(afterColon)
		}

		slashIdx := strings.Index(rest, "/")
		if slashIdx < 0 {
			continue
		}
		path := rest[slashIdx:]
		if spaceIdx := strings.IndexAny(path, " |"); spaceIdx > 0 {
			path = path[:spaceIdx]
		}

		return path, port
	}

	return "", 0
}

// parseDependsOn handles both list and map forms of depends_on.
func parseDependsOn(raw interface{}) []string {
	if raw == nil {
		return nil
	}

	switch v := raw.(type) {
	case []interface{}:
		deps := make([]string, 0, len(v))
		for _, d := range v {
			if s, ok := d.(string); ok {
				deps = append(deps, s)
			}
		}
		return deps
	case map[string]interface{}:
		deps := make([]string, 0, len(v))
		for name := range v {
			deps = append(deps, name)
		}
		sort.Strings(deps)
		return deps
	}
	return nil
}

// parseEnvironment handles both map and list forms of environment.
func parseEnvironment(raw interface{}) map[string]string {
	if raw == nil {
		return nil
	}

	envs := make(map[string]string)

	switch v := raw.(type) {
	case map[string]interface{}:
		for key, val := range v {
			envs[key] = fmt.Sprintf("%v", val)
		}
	case []interface{}:
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				continue
			}
			parts := strings.SplitN(s, "=", 2)
			if len(parts) == 2 {
				envs[parts[0]] = parts[1]
			}
		}
	}
	return envs
}

// reservedWords are .mozza keywords that cannot be used as section headers.
var reservedWords = map[string]bool{
	"app":       true,
	"namespace": true,
	"images":    true,
}

// capitalize returns the string with its first letter uppercased.
// If the name is a reserved .mozza keyword, it appends "-svc" to avoid conflicts.
func capitalize(s string) string {
	if s == "" {
		return s
	}
	if reservedWords[strings.ToLower(s)] {
		s = s + "-svc"
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

// sortedKeys returns the sorted keys from a map with string keys.
// It works with map[string]T for any T via type assertion through interface{}.
func sortedKeys[T any](m map[string]T) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
