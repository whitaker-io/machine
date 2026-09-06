// Package ast - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package ast

import (
	"reflect"
	"strings"
	"testing"
)

// commentControl is a file that carries no comment at all, written so that every
// position a comment may occupy exists in it: a file-level line, an import, a
// flow declaration, a state block and one of its field types, a var type, a
// statement whose operand is a Go span, a from-list, a continuation clause with
// an operand, and a switch arm target.
//
// IT IS THE ANSWER KEY FOR REQUIREMENT 1. The commented variants below differ
// from it only by added comments, so a tree that differs anywhere but in its
// positions is the change dropping or moving something it was asked to carry.
const commentControl = `import billing "github.com/acme/billing"

flow orders
state {
  processed int
}
var attempt int
source events http.Listen[Order](":8080")
transform charge billing.Charge from events
  over pubsub.Topic("t")
switch route from charge on charge.Kind {
  "card" -> billable
  else -> other
}
sink done audit.Store from billable, other
`

// positionType is the type every masking walk below erases.
var positionType = reflect.TypeOf(Position{})

// maskPositions zeroes every Position in a tree so two trees can be compared for
// the structure they carry rather than for where it was written.
//
// AN INTERFACE ELEMENT IS COPIED RATHER THAN WRITTEN THROUGH. File.Decls holds
// Decl interfaces over struct VALUES, and the value inside an interface is not
// addressable; masking it in place silently does nothing, which would make this
// helper report equality it never checked. The copy is masked and set back.
func maskPositions(v reflect.Value) {
	if v.Type() == positionType {
		if v.CanSet() {
			v.Set(reflect.Zero(positionType))
		}

		return
	}

	switch v.Kind() {
	case reflect.Pointer:
		if !v.IsNil() {
			maskPositions(v.Elem())
		}
	case reflect.Interface:
		if v.IsNil() {
			return
		}
		inner := reflect.New(v.Elem().Type()).Elem()
		inner.Set(v.Elem())
		maskPositions(inner)
		if v.CanSet() {
			v.Set(inner)
		}
	case reflect.Struct:
		for i := range v.NumField() {
			maskPositions(v.Field(i))
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			maskPositions(v.Index(i))
		}
	case reflect.Map:
		for _, key := range v.MapKeys() {
			inner := reflect.New(v.MapIndex(key).Type()).Elem()
			inner.Set(v.MapIndex(key))
			maskPositions(inner)
			v.SetMapIndex(key, inner)
		}
	default:
	}
}

// parseMasked parses src, requires it clean, and returns its tree with every
// position erased and its comments DETACHED into the second return.
//
// REQUIREMENT 1 EXCLUDES EXACTLY TWO THINGS — the positions and the comment
// trivia itself — so both are removed here and the comments are handed back
// rather than dropped, because a caller that never sees them cannot tell a
// carrier holding what was written from one holding nothing.
func parseMasked(t *testing.T, label, src string) (*File, []Comment) {
	t.Helper()
	file, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("%s does not parse clean: %v\nthe source:\n%s", label, err, src)
	}
	comments := file.Comments
	file.Comments = nil
	maskPositions(reflect.ValueOf(file).Elem())

	return file, comments
}

// commentBodies returns the body of each comment written in src, in the order a
// reader meets them, derived from the TEXT rather than from the tree.
func commentBodies(src string) []string {
	var out []string
	for _, line := range strings.Split(src, "\n") {
		if idx := strings.Index(line, "//"); idx >= 0 {
			out = append(out, line[idx+len("//"):])
		}
	}

	return out
}

// assertCarriedComments asserts the carrier holds exactly the comments the
// source writes, in order.
func assertCarriedComments(t *testing.T, src string, got []Comment) {
	t.Helper()
	want := commentBodies(src)
	if len(got) != len(want) {
		t.Fatalf("the carrier holds %d comments for a source writing %d:\n%s", len(got), len(want), src)
	}
	for i := range want {
		if got[i].Text != want[i] {
			t.Errorf("comment %d carries %q, want %q", i+1, got[i].Text, want[i])
		}
	}
}

// TestMaskPositionsReachesInsideAnInterfaceElement is the control for the
// comparison instrument itself.
//
// WITHOUT IT EVERY EQUALITY BELOW COULD BE VACUOUS. A masker that silently
// failed to reach through File.Decls would leave both trees carrying their real
// positions, and two files whose text differs would then compare UNEQUAL rather
// than equal — but a masker that reached nothing at all and a masker that
// reached everything are indistinguishable from a passing assertion alone. This
// asserts the erasure happened where it is hardest to happen.
func TestMaskPositionsReachesInsideAnInterfaceElement(t *testing.T) {
	file, err := Parse([]byte(commentControl))
	if err != nil {
		t.Fatalf("CONTROL FAILED: the answer-key source does not parse: %v", err)
	}
	if len(file.Decls) == 0 {
		t.Fatalf("CONTROL FAILED: the answer key parsed to no declarations at all")
	}
	if file.Decls[0].Pos().Offset == 0 && file.Decls[0].Pos().Line == 0 {
		t.Fatalf("CONTROL FAILED: the first declaration's position is already zero before masking")
	}

	maskPositions(reflect.ValueOf(file).Elem())

	if got := file.Decls[0].Pos(); got != (Position{}) {
		t.Errorf("masking did not reach the position inside an interface element: %+v", got)
	}
	if got := file.Pos(); got != (Position{}) {
		t.Errorf("masking did not reach the file's own start: %+v", got)
	}
}

// commentLineClasses are the eleven positions a comment may be written in, each
// as the whole commented file it produces.
//
// EACH IS ITS OWN CLASS rather than one file carrying all eleven, because the
// four trailing-operand positions are the ones where today's lexer ABSORBS the
// marker into a span instead of refusing it, and a single combined file would
// report one failure for whichever of them broke first.
var commentLineClasses = map[string]string{
	"own line at file level": `// a file-level comment
import billing "github.com/acme/billing"

flow orders
source events http.Listen[Order](":8080")
sink done audit.Store from events
`,
	"own line at statement position": `flow orders
source events http.Listen[Order](":8080")
// why this sink exists
sink done audit.Store from events
`,
	"trailing after a flow declaration": `flow orders // the order pipeline
source events http.Listen[Order](":8080")
sink done audit.Store from events
`,
	"trailing after an import": `import billing "github.com/acme/billing" // the billing package
flow orders
source events http.Listen[Order](":8080")
sink done audit.Store from events
`,
	"trailing after a from-list": `flow orders
source events http.Listen[Order](":8080")
transform charge billing.Charge from events // from the source
sink done audit.Store from charge
`,
	"trailing after a switch arm target": `flow orders
source events http.Listen[Order](":8080")
switch route from events on events.Kind {
  "card" -> billable // the card arm
  else -> other
}
sink done audit.Store from billable, other
`,
	"own line inside a state block": `flow orders
state {
  // how many have been processed
  processed int
}
source events http.Listen[Order](":8080")
sink done audit.Store from events
`,
	"trailing on a Go-span operand line": `flow orders
source events http.Listen[Order](":8080") // the listener
sink done audit.Store from events
`,
	"trailing on a clause operand": `flow orders
source events http.Listen[Order](":8080")
transform charge billing.Charge from events
  over pubsub.Topic("t") // why the transport
sink done audit.Store from charge
`,
	"trailing on a state field type": `flow orders
state {
  processed int // how many
}
source events http.Listen[Order](":8080")
sink done audit.Store from events
`,
	"trailing on a var type": `flow orders
var attempt int // how many tries
source events http.Listen[Order](":8080")
sink done audit.Store from events
`,
}

// stripComments removes every ` + "`//`" + ` comment from a source, producing the control
// that source differs from only by its comments.
//
// IT IS TEXT WORK ON PURPOSE and it is safe for these fixtures alone: none of
// them writes a marker inside a string, a note or a func body, which the
// dedicated requirement-2 tests below cover instead.
func stripComments(src string) string {
	var out []string
	for _, line := range strings.Split(src, "\n") {
		idx := strings.Index(line, "//")
		if idx < 0 {
			out = append(out, line)

			continue
		}
		trimmed := strings.TrimRight(line[:idx], " \t")
		if strings.TrimSpace(trimmed) == "" {
			continue
		}
		out = append(out, trimmed)
	}
	stripped := strings.Join(out, "\n")

	// EVERY .flow FILE ENDS WITH A NEWLINE and the parser says so. A source whose
	// last line is a comment written without a trailing newline loses that
	// newline when the comment is removed, so the control would be refused for a
	// reason that has nothing to do with comments.
	if stripped != "" && !strings.HasSuffix(stripped, "\n") {
		stripped += "\n"
	}

	return stripped
}

// TestACommentedFileYieldsItsControlTree is requirement 1: a file differing from
// a clean one only by comments parses with a nil error and yields the same tree
// modulo positions.
func TestACommentedFileYieldsItsControlTree(t *testing.T) {
	if len(commentLineClasses) != 11 {
		t.Fatalf("CONTROL FAILED: the eleven line classes the ticket names are %d here", len(commentLineClasses))
	}

	for name, commented := range commentLineClasses {
		t.Run(name, func(t *testing.T) {
			control := stripComments(commented)
			if !strings.Contains(commented, "//") {
				t.Fatalf("CONTROL FAILED: the %q class carries no comment marker at all", name)
			}
			if strings.Contains(control, "//") {
				t.Fatalf("CONTROL FAILED: the control still carries a marker:\n%s", control)
			}

			want, controlComments := parseMasked(t, "the control", control)
			got, carried := parseMasked(t, "the commented source", commented)

			if len(controlComments) != 0 {
				t.Fatalf("CONTROL FAILED: the comment-free control carries %d comments", len(controlComments))
			}
			if !reflect.DeepEqual(want, got) {
				t.Errorf("the commented source yields a different tree than its control\ncontrol:\n%s\ncommented:\n%s\nwant %+v\ngot  %+v",
					control, commented, want, got)
			}
			assertCarriedComments(t, commented, carried)
		})
	}
}

// commentBoundaryForms are the edges of the comment form itself.
var commentBoundaryForms = map[string]string{
	"the file's first byte": `// the very first byte of the file
flow orders
source events http.Listen[Order](":8080")
sink done audit.Store from events
`,
	"the file's last line with a trailing newline": `flow orders
source events http.Listen[Order](":8080")
sink done audit.Store from events
// the last line
`,
	"the file's last line without a trailing newline": `flow orders
source events http.Listen[Order](":8080")
sink done audit.Store from events
// the last line, unterminated`,
	"an empty comment": `flow orders
//
source events http.Listen[Order](":8080")
sink done audit.Store from events
`,
	"a comment holding only whitespace": `flow orders
//   ` + `
source events http.Listen[Order](":8080")
sink done audit.Store from events
`,
	"a body holding a note delimiter": `flow orders
// a body holding """ a note delimiter
source events http.Listen[Order](":8080")
sink done audit.Store from events
`,
	"a body holding an arrow": `flow orders
// a body holding -> an arrow
source events http.Listen[Order](":8080")
sink done audit.Store from events
`,
	"a body holding a brace": `flow orders
// a body holding { a brace
source events http.Listen[Order](":8080")
sink done audit.Store from events
`,
	"a body holding a second marker": `flow orders
// a body holding // a second marker
source events http.Listen[Order](":8080")
sink done audit.Store from events
`,
}

// TestCommentBoundaryFormsParseClean asserts the comment form's own edges are
// accepted, and that each yields the tree its comment-free control yields.
func TestCommentBoundaryFormsParseClean(t *testing.T) {
	if len(commentBoundaryForms) == 0 {
		t.Fatalf("CONTROL FAILED: no boundary form is listed, so this test checked nothing")
	}

	for name, src := range commentBoundaryForms {
		t.Run(name, func(t *testing.T) {
			got, carried := parseMasked(t, "the boundary form", src)
			want, _ := parseMasked(t, "its control", stripComments(src))
			if !reflect.DeepEqual(want, got) {
				t.Errorf("the boundary form yields a different tree than its control\nsource:\n%s\nwant %+v\ngot  %+v", src, want, got)
			}
			assertCarriedComments(t, src, carried)
		})
	}
}

// noteBodyWithMarker holds the marker inside a note body, which is the prose
// region and stays verbatim.
const noteBodyWithMarker = `flow orders
note """a // b"""
source events http.Listen[Order](":8080")
sink done audit.Store from events
`

// TestMarkerInsideANoteBodyIsVerbatim is requirement 2's note-body arm.
func TestMarkerInsideANoteBodyIsVerbatim(t *testing.T) {
	file, err := Parse([]byte(noteBodyWithMarker))
	if err != nil {
		t.Fatalf("a note body holding the marker does not parse: %v", err)
	}

	var seen []string
	for _, decl := range file.Decls {
		flow, ok := decl.(FlowDecl)
		if !ok {
			continue
		}
		if flow.Note != nil {
			seen = append(seen, flow.Note.Text)
		}
	}
	if len(seen) != 1 {
		t.Fatalf("CONTROL FAILED: the source declares one flow-level note and the tree carries %d", len(seen))
	}
	if seen[0] != "a // b" {
		t.Errorf("the note body is %q; the marker inside a note must stay verbatim", seen[0])
	}
}

// multiLineSpanWithMarker puts the marker inside a Go span at bracket depth one,
// where a newline does not end the span either.
const multiLineSpanWithMarker = `flow orders
source events pkg.A(
  1, // one
  2,
)
sink done audit.Store from events
`

// TestMarkerInsideAMultiLineGoSpanStaysSpanText is requirement 2's depth-above-
// zero arm: the span stop is a DEPTH-ZERO rule, so a marker nested inside
// brackets is Go's own comment and belongs to the span text.
func TestMarkerInsideAMultiLineGoSpanStaysSpanText(t *testing.T) {
	spans := sourceSpans(t, multiLineSpanWithMarker)
	if len(spans) != 1 {
		t.Fatalf("CONTROL FAILED: the source declares one source statement and the tree carries %d", len(spans))
	}
	if !strings.Contains(spans[0], "// one") {
		t.Errorf("the span text is %q; a marker at bracket depth one is Go's own comment and stays in the span", spans[0])
	}
}

// sourceSpans returns the Go-span text of every source statement in src.
func sourceSpans(t *testing.T, src string) []string {
	t.Helper()
	file, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("the source does not parse: %v\n%s", err, src)
	}

	var out []string
	for _, decl := range file.Decls {
		flow, ok := decl.(FlowDecl)
		if !ok {
			continue
		}
		for _, stmt := range flow.Body {
			if source, isSource := stmt.(SourceStmt); isSource {
				out = append(out, source.Ref.Text)
			}
		}
	}

	return out
}

// TestMarkerInsideAFuncBodyStaysFuncSpanText is requirement 2's func-body arm,
// asserted against the corpus fixture that already carries the shape.
func TestMarkerInsideAFuncBodyStaysFuncSpanText(t *testing.T) {
	const path = validCorpusDir + "/func-go-aware-spans.flow"
	src := readFixture(t, path)
	if !strings.Contains(string(src), "// a closing brace") {
		t.Fatalf("CONTROL FAILED: %s no longer carries a marker inside a func body", path)
	}

	file, err := Parse(src)
	if err != nil {
		t.Fatalf("%s does not parse: %v", path, err)
	}

	bodies := 0
	for _, decl := range file.Decls {
		fn, ok := decl.(FuncDecl)
		if !ok {
			continue
		}
		if strings.Contains(fn.Body.Text, "// a closing brace } lives inside this comment") {
			bodies++
		}
	}
	if bodies != 1 {
		t.Errorf("%d func bodies carry the marker verbatim; the Go-aware func span owns its own comment forms", bodies)
	}
}

// TestAQuotedURLInASpanIsNotAComment asserts the quoted-literal skip runs before
// the marker rule, so a `//` inside a string never ends a span.
func TestAQuotedURLInASpanIsNotAComment(t *testing.T) {
	const src = `flow orders
source events pkg.Topic("https://example.com/x")
sink done audit.Store from events
`
	spans := sourceSpans(t, src)
	if len(spans) != 1 {
		t.Fatalf("CONTROL FAILED: the source declares one source statement and the tree carries %d", len(spans))
	}
	if spans[0] != `pkg.Topic("https://example.com/x")` {
		t.Errorf("the span text is %q; a marker inside a quoted literal is text", spans[0])
	}
}

// TestASingleSlashIsNotAComment asserts the rule requires the PAIR, so Go's
// division operator keeps working inside a span.
func TestASingleSlashIsNotAComment(t *testing.T) {
	const src = `flow orders
source events pkg.A(x/y)
sink done audit.Store from events
`
	spans := sourceSpans(t, src)
	if len(spans) != 1 {
		t.Fatalf("CONTROL FAILED: the source declares one source statement and the tree carries %d", len(spans))
	}
	if spans[0] != "pkg.A(x/y)" {
		t.Errorf("the span text is %q; a single slash is division, not a marker", spans[0])
	}
}

// TestATrailingMarkerIsNoLongerAbsorbedIntoTheGoSpan is the absorption-reversed
// row: today a trailing marker on a Go-span line parses clean and is silently
// swallowed into the span's text, so the observable is not a refusal becoming an
// acceptance but a span becoming byte-identical to its control's.
func TestATrailingMarkerIsNoLongerAbsorbedIntoTheGoSpan(t *testing.T) {
	const commented = `flow orders
source events pkg.Listen[E](":1") // trailing
sink done audit.Store from events
`
	const control = `flow orders
source events pkg.Listen[E](":1")
sink done audit.Store from events
`
	want, got := sourceSpans(t, control), sourceSpans(t, commented)
	if len(want) != 1 || len(got) != 1 {
		t.Fatalf("CONTROL FAILED: one source statement each, and the trees carry %d and %d", len(want), len(got))
	}
	if got[0] != want[0] {
		t.Errorf("the commented span text is %q against the control's %q; a trailing marker is a comment, not span text", got[0], want[0])
	}
}

// TestASwitchArmCommentLineParsesClean is the switch-arm row: an arm begins with
// a Go span, so a line-leading marker inside a switch block is the one position
// where it is already legal Go-span text today and fails downstream instead of
// at the marker.
func TestASwitchArmCommentLineParsesClean(t *testing.T) {
	const commented = `flow orders
source events http.Listen[Order](":8080")
switch route from events on events.Kind {
  // the card arm
  "card" -> billable
  else -> other
}
sink done audit.Store from billable, other
`
	got, carried := parseMasked(t, "the commented switch", commented)
	want, _ := parseMasked(t, "its control", stripComments(commented))
	if !reflect.DeepEqual(want, got) {
		t.Errorf("a comment line inside a switch block changes the tree\nwant %+v\ngot  %+v", want, got)
	}
	assertCarriedComments(t, commented, carried)
}

// markerAsWholeOperand pairs each guarded clause with the diagnostic its bare
// form already produces, so the marker-as-operand form is held to the SAME
// refusal rather than to one invented for it.
var markerAsWholeOperand = map[string]struct{ commented, bare, message string }{
	"over": {
		commented: `flow orders
source events http.Listen[Order](":8080")
transform charge billing.Charge from events
  over //
sink done audit.Store from charge
`,
		bare: `flow orders
source events http.Listen[Order](":8080")
transform charge billing.Charge from events
  over
sink done audit.Store from charge
`,
		message: `"over" needs an operand: a transport factory expression`,
	},
	"checkpoint": {
		commented: `flow orders
source events http.Listen[Order](":8080")
transform charge billing.Charge from events
  checkpoint //
sink done audit.Store from charge
`,
		bare: `flow orders
source events http.Listen[Order](":8080")
transform charge billing.Charge from events
  checkpoint
sink done audit.Store from charge
`,
		message: `"checkpoint" needs an operand: a codec expression`,
	},
}

// TestTheMarkerAsAWholeClauseOperandIsRefused is the row requirement 1's
// equality cannot reach, because a file that REPLACES an operand with a comment
// is not a file differing only by added comments.
//
// A DEPTH-ZERO MARKER STOP ADDS A THIRD ROUTE to the empty span that
// requireClauseOperand refuses, at exactly the positions where the marker used
// to be absorbed into a non-empty one. Without this, a parser that returned
// early on a marker would accept an operandless clause and every other test
// written from the ticket would still pass.
func TestTheMarkerAsAWholeClauseOperandIsRefused(t *testing.T) {
	for keyword, form := range markerAsWholeOperand {
		t.Run(keyword, func(t *testing.T) {
			if _, err := Parse([]byte(form.bare)); err == nil {
				t.Fatalf("CONTROL FAILED: the bare %q form parses clean, so the refusal this row rides does not exist", keyword)
			} else if !strings.Contains(err.Error(), form.message) {
				t.Fatalf("CONTROL FAILED: the bare %q form is refused for another reason: %v", keyword, err)
			}

			err := parseError(t, form.commented)
			if !strings.Contains(err.Error(), form.message) {
				t.Errorf("a bare marker after %q is refused as %v; it must produce the same diagnostic the bare form does", keyword, err)
			}
		})
	}
}

// parseError parses src and requires a diagnostic.
func parseError(t *testing.T, src string) error {
	t.Helper()
	_, err := Parse([]byte(src))
	if err == nil {
		t.Fatalf("the source parses clean and was required to be refused:\n%s", src)
	}

	return err
}
