package importer

import (
	"fmt"
	"strconv"
	"strings"
)

// A value read from an imported artifact is written into recipe text at one of
// three kinds of position (as a bare word, inside a quoted string, or inside a
// comment), and what can end the value differs at each. The recipe grammar
// offers no escape sequence in any of them: recipe.Lexer.scanString reads to the
// next quote with no way to write one, and a newline ends a line everywhere. So
// a value carrying a byte that is structural where it lands cannot be encoded,
// only refused, and the predicates below refuse it while naming the field it
// came from.
//
// Rejecting is per-position rather than per-character, and the parser's rule
// that a directive begins a line does not make it redundant. That rule stops a
// value from starting a second directive; it cannot see one widening the
// directive it is already inside, because the extra tokens land in that same
// directive's argument list on the same line: `depends_on: "db and cache"`
// writes `needs db and cache`, which parses clean as two dependencies. A plain
// space in a bare word is therefore still as structural as a newline. A newline
// in a comment still ends the comment and leaves what follows on a live line,
// while the same space inside a quoted string is inert and must stay accepted.
const (
	// structuralInWord is every byte recipe's isDelimiter treats as a token
	// boundary, plus the two line terminators. A bare word has to lex to exactly
	// one token, and any of these splits it into two.
	structuralInWord = " \t,\"\n\r"

	// structuralInString is what ends a quoted string. scanString searches for
	// its closing quote within one line only, so a string cannot be ended by
	// anything else. Spaces, tabs and commas keep their literal selves inside
	// one, which is what lets an env value hold a sentence.
	structuralInString = "\"\n\r"

	// structuralInComment is what ends a comment. A comment is the whole of its
	// line, so nothing short of a line terminator escapes one; a mount path may
	// hold spaces and quotes and still be reported verbatim.
	structuralInComment = "\n\r"
)

// structuralByteNames names each byte the sets above reject, so the set a
// position rejects and the words a message uses for it cannot drift apart.
// TestEveryStructuralByteIsNamed holds the two together.
var structuralByteNames = map[byte]string{ //nolint:gochecknoglobals // immutable lookup table
	' ':  "a space",
	'\t': "a tab",
	',':  "a comma",
	'"':  "a quote",
	'\n': "a newline",
	'\r': "a carriage return",
}

// A carriage return is inert in today's lexer (it splits on "\n" alone and
// isDelimiter omits '\r'), so every set above rejects one byte more than the
// grammar currently requires. It is refused anyway because nothing is holding it
// inert: a CRLF-authored recipe does not parse today, so normalising line
// endings, or adding '\r' to isDelimiter, is an ordinary future fix and either
// one makes '\r' structural everywhere at once. The rest of the repo already
// treats it as a terminator (plan.Validate rejects it in an image), and a value
// that needs a bare CR in it is not a value any of these fields holds.

// recipeSafeWord reports whether value can be written into recipe text unquoted
// and outside a comment, as `restart <value>` does.
func recipeSafeWord(field, value string) error {
	return recipeSafe(field, value, structuralInWord, "the word")
}

// recipeSafeString reports whether value can be written between quotes, as
// `set KEY to "<value>"` does.
func recipeSafeString(field, value string) error {
	return recipeSafe(field, value, structuralInString, "the quoted value")
}

// recipeSafeComment reports whether value can be written inside a comment, as
// the `# WARNING:` lines do.
func recipeSafeComment(field, value string) error {
	return recipeSafe(field, value, structuralInComment, "the comment")
}

// recipeSafe refuses a value that would not survive the position it is about to
// be written into, naming the field so the author can find it. ends is what the
// offending byte would close, in the reader's terms.
func recipeSafe(field, value, structural, ends string) error {
	i := strings.IndexAny(value, structural)
	if i < 0 {
		return nil
	}

	name, known := structuralByteNames[value[i]]
	if !known {
		name = strconv.Quote(string(value[i]))
	}

	return fmt.Errorf(
		"%s: %s cannot be written to a recipe — it holds %s, which would end %s and let the rest parse as recipe directives",
		field, reportedRecipeValue(value), name, ends)
}

// maxReportedValue is how much of a value a message quotes back. An imported
// file can hold a whole certificate in one scalar, and a message that reprints
// it is no more use than one that names no value at all.
const maxReportedValue = 40

// reportedRecipeValue quotes a value back for a message. composeGot defers to it
// so the read errors and the errors above cut a long value the same way.
func reportedRecipeValue(value string) string {
	if len(value) > maxReportedValue {
		return "a value beginning " + strconv.Quote(value[:maxReportedValue])
	}

	return strconv.Quote(value)
}
