// Package analysis - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package analysis

import (
	"errors"
	"strconv"

	"github.com/whitaker-io/machine/lang/ast"
)

// parseCode is the Code carried by a diagnostic converted out of a parse error.
//
// It is not an analyzer name: no analyzer produced it, the parser did.
const parseCode = "parse"

// Severity is how loudly a Diagnostic asks to be heard.
//
// The vocabulary is the framework's own addition. ast.Diagnostic carries a
// position and a message and deliberately nothing else, because the parser has
// no opinion about how bad a thing is; a linter needs one and an editor needs
// one, so the pass framework supplies it.
type Severity int

const (
	// SeverityError marks a program that is wrong: an undefined reference, a
	// node nothing can reach, a state entry spelled in a retired form.
	SeverityError Severity = iota
	// SeverityWarning marks a program that is suspicious but not provably wrong.
	SeverityWarning
	// SeverityHint marks an observation an author may reasonably ignore, such as
	// a producer whose output nothing consumes.
	SeverityHint
)

// String renders the severity as the lowercase word a command line and an editor
// both display without further formatting.
func (s Severity) String() string {
	switch s {
	case SeverityError:
		return "error"
	case SeverityWarning:
		return "warning"
	case SeverityHint:
		return "hint"
	default:
		return "severity(" + strconv.Itoa(int(s)) + ")"
	}
}

// Diagnostic is one positioned finding, with the severity and the rule identity
// ast.Diagnostic does not carry.
//
// Code is the emitting analyzer's Name, so a consumer can suppress or route one
// rule without matching on message text.
//
// Path is the file the diagnostic is about. A run covers many sources, and a
// position alone cannot name one: ast.Position.Offset is per-file and every
// parsed tree starts at offset zero, so two diagnostics in two files routinely
// carry identical positions. The driver stamps Path from the Source the
// reporting analyzer named, exactly as it stamps Code.
//
// FOREIGN IS THE ONE EXCEPTION, AND IT IS THE DRIVER'S TO SET. An analysis that
// reads Go the run never parsed — a hand-written consumer package reached
// through a loaded package set — has a file, a line and a column to report, and
// no Source to hang them on. Such a finding is reported through
// Pass.ReportForeign, which keeps the analyzer's own Path and marks it here.
// THE POSITION IS CARRIED STRUCTURALLY RATHER THAN COMPOSED INTO Message: a
// consumer that renders `path:line:col` renders the real file, a JSON consumer
// projects it, and an editor is not handed a position in a document it does not
// have. Composing it into the message instead is a workaround this type exists
// to make unnecessary.
type Diagnostic struct {
	Pos      ast.Position
	End      ast.Position
	Message  string
	Severity Severity
	Code     string
	Path     string
	// Foreign reports that Path names a file the run did not parse, so a
	// consumer keyed on the run's own sources — an editor publishing per open
	// document, a suppression keyed on a damaged parse — can tell the two apart
	// instead of positioning a Go file inside a .flow buffer.
	Foreign bool
}

// Source is one file under analysis: its path, its bytes and its parsed tree.
//
// The bytes are retained alongside the tree because two consumers need them. The
// Mermaid renderer reads them, and an LSP mapping byte columns onto UTF-16 code
// units will need them too — ast.Position.Col counts BYTES rather than runes,
// which position.go states explicitly and which no re-scan of the tree recovers.
type Source struct {
	Path string
	Src  []byte
	File *ast.File
}

// ParseDiagnostics converts the diagnostics carried by an ast.Parse error into
// framework diagnostics at SeverityError under the "parse" code.
//
// A parse that produced diagnostics still produced a tree, so a caller feeds the
// tree to the analyzers AND renders these alongside whatever the analyzers find.
// A nil error, or an error that is not an *ast.Error, yields no diagnostics.
//
// The path is supplied by the caller rather than read off anything, because a
// parse error carries a tree and positions and never a file name.
func ParseDiagnostics(path string, err error) []Diagnostic {
	var perr *ast.Error
	if !errors.As(err, &perr) {
		return nil
	}
	out := make([]Diagnostic, 0, len(perr.Diagnostics))
	for _, d := range perr.Diagnostics {
		out = append(out, Diagnostic{
			Pos:      d.Pos,
			End:      d.End,
			Message:  d.Message,
			Severity: SeverityError,
			Code:     parseCode,
			Path:     path,
		})
	}
	return out
}
