package importer

import (
	"errors"
	"fmt"
	"reflect"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// yaml reports a shape it could not read in Go's own terms — "cannot unmarshal
// !!str `web` into map[string]importer.composeService" — naming a type and a tag
// that appear nowhere in a compose file. Compose's own validator reports the
// same fault as "services must be a mapping", so that is the shape used below:
// the field named by the dotted path compose itself prints, and the value named
// by what the file was expected to hold.
//
// The unmarshalers in compose_decode.go never come through here. yaml hands an unmarshaler's
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
