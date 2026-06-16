package recipe

import "fmt"

// ParseError represents a lexing or parsing error with source location.
type ParseError struct {
	Line    int    // 1-based line number where the error occurred.
	Col     int    // 1-based column number where the error occurred.
	Message string // Human-readable description of the error.
}

// Error returns the formatted error string including source location.
func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d, col %d: %s", e.Line, e.Col, e.Message)
}

// newParseError creates a ParseError at the given position.
func newParseError(line, col int, msg string) *ParseError {
	return &ParseError{
		Line:    line,
		Col:     col,
		Message: msg,
	}
}

// newParseErrorf creates a ParseError at the given position with a formatted message.
func newParseErrorf(line, col int, format string, args ...any) *ParseError {
	return &ParseError{
		Line:    line,
		Col:     col,
		Message: fmt.Sprintf(format, args...),
	}
}
