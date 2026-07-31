package importer

import (
	"errors"
	"fmt"
	"reflect"
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

// maxReportedValue is how much of a value a message quotes back. A compose file
// can hold a whole certificate in one scalar, and a message that reprints it is
// no more use than one that names no value at all.
const maxReportedValue = 40

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
	Secrets     []composeSecretRef  `yaml:"secrets"`
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
	Driver   string `yaml:"driver"`
	Internal bool   `yaml:"internal"`
}

// UnmarshalYAML reads internal in every shape compose accepts. Compose calls it
// "a boolean or string", so `internal: "true"` is as documented as
// `internal: true`.
//
// The boolean is decoded first, by the same decoder as before, which is what
// keeps yaml's own vocabulary — including bare yes/no/on/off, and TRUE in any
// case — working byte for byte. Only when that fails is the value read as a
// string, and then only compose's own vocabulary is honoured: it accepts these
// twelve spellings and rejects "0", "1" and anything else, so mozza saying so
// out loud is the same answer compose gives.
func (c *composeNetworkConfig) UnmarshalYAML(n *yaml.Node) error {
	var mirror struct {
		Driver   string    `yaml:"driver"`
		Internal yaml.Node `yaml:"internal"`
	}
	if err := n.Decode(&mirror); err != nil {
		return composeMirrorError(n, err, "networks", "a mapping of network settings")
	}

	c.Driver = mirror.Driver

	const field = "networks.internal"

	internal := resolveAlias(&mirror.Internal)
	if internal.IsZero() {
		return nil
	}

	if err := internal.Decode(&c.Internal); err == nil {
		return nil
	}

	if internal.Kind != yaml.ScalarNode {
		return composeUnreadable(internal, field, "true or false, got a list or a mapping")
	}

	raw := internal.Value
	switch strings.ToLower(raw) {
	case "true", "yes", "on":
		c.Internal = true
	case "false", "no", "off":
		c.Internal = false
	default:
		return composeUnreadable(internal, field, fmt.Sprintf("true or false, got %q", raw))
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
	if len(value) > maxReportedValue {
		return "a value beginning " + strconv.Quote(value[:maxReportedValue])
	}

	return strconv.Quote(value)
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
// This is the legacy API used by the scanner; it delegates to ComposeToRecipeAST
// internally and returns the text representation.
func ComposeToRecipe(composeYAML string) (string, error) {
	var cf composeFile
	content := []byte(composeYAML)
	if err := yaml.Unmarshal(content, &cf); err != nil {
		return "", fmt.Errorf("parsing compose YAML: %w", composeReadError(content, err))
	}
	if len(cf.Services) == 0 {
		return "", fmt.Errorf("no services found in compose file")
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
	// already exists — a pvc mount compiles to a claimName taken verbatim from the
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

	// Networks -> NetworkPolicy (basic mapping).
	svcNetworks := parseServiceNetworks(svc.Networks)
	if len(svcNetworks) > 0 {
		slice.NetworkPolicy = &recipe.NetworkPolicySpec{
			AllowFrom: svcNetworks,
		}
	}

	return slice, warnings
}

// convertNetworks produces warnings for compose networks that need attention.
func convertNetworks(networks map[string]composeNetworkConfig) []Warning {
	var warnings []Warning
	for name, net := range networks {
		if net.Internal {
			warnings = append(warnings, Warning{
				Feature:  "networks",
				Message:  fmt.Sprintf("internal network %q mapped to NetworkPolicy deny-all with allowFrom — verify rules", name),
				Severity: "info",
			})
		}
	}
	return warnings
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

// writePort writes a port directive to the builder.
func writePort(b *strings.Builder, portSpec string) {
	// Formats: "8080:80", "8080", "8080:80/udp"
	portSpec = strings.TrimSpace(portSpec)

	// Strip protocol suffix for now.
	portSpec = strings.Split(portSpec, "/")[0]

	if strings.Contains(portSpec, ":") {
		parts := strings.SplitN(portSpec, ":", 2)
		containerPort := parts[1]
		fmt.Fprintf(b, "  open to the public on port %s\n", containerPort)
	} else {
		fmt.Fprintf(b, "  on port %s\n", portSpec)
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
