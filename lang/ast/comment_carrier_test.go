// Package ast - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package ast

import (
	"strings"
	"testing"
)

// carrierSource writes four comments in four different positions, so the order
// the carrier reports is checkable against the order they are written in.
const carrierSource = `// one, at the very first byte
flow orders
source events http.Listen[Order](":8080") // two, trailing a Go-span operand
// three, on a line of its own between statements
sink done audit.Store from events
// four, after the last statement
`

// TestEveryCommentIsCarriedWithAByteAccuratePosition is requirement 4: a file
// with N comments yields a carrier of length N in source order, each positioned
// at the byte its marker occupies.
//
// THE EXPECTED POSITIONS ARE COMPUTED FROM THE RAW BYTES, never read back from
// the tree: strings.Index finds each marker in the source and the line and
// column are recomputed from the byte offset by the same independent arithmetic
// the position-fidelity gate uses. A test that asked the parser where it put
// something and then agreed with itself would assert nothing.
func TestEveryCommentIsCarriedWithAByteAccuratePosition(t *testing.T) {
	src := []byte(carrierSource)
	file, err := Parse(src)
	if err != nil {
		t.Fatalf("the carrier source does not parse: %v", err)
	}

	wantBodies := []string{
		" one, at the very first byte",
		" two, trailing a Go-span operand",
		" three, on a line of its own between statements",
		" four, after the last statement",
	}
	if strings.Count(carrierSource, "//") != len(wantBodies) {
		t.Fatalf("CONTROL FAILED: the source writes %d markers against %d expected bodies",
			strings.Count(carrierSource, "//"), len(wantBodies))
	}
	if len(file.Comments) != len(wantBodies) {
		t.Fatalf("the carrier holds %d comments for a source writing %d", len(file.Comments), len(wantBodies))
	}

	starts := lineStarts(src)
	searchFrom := 0
	for i, want := range wantBodies {
		got := file.Comments[i]
		if got.Text != want {
			t.Errorf("comment %d carries %q, want %q", i+1, got.Text, want)
		}

		offset := strings.Index(carrierSource[searchFrom:], "//") + searchFrom
		searchFrom = offset + len("//")
		if got.Start.Offset != offset {
			t.Errorf("comment %d starts at recorded offset %d; its marker's first byte is at %d", i+1, got.Start.Offset, offset)
		}

		wantLine, wantCol := lineColAt(starts, offset)
		if got.Start.Line != wantLine || got.Start.Col != wantCol {
			t.Errorf("comment %d starts at recorded %d:%d; offset %d is at %d:%d",
				i+1, got.Start.Line, got.Start.Col, offset, wantLine, wantCol)
		}

		wantEnd := offset + len("//") + len(want)
		if got.Stop.Offset != wantEnd {
			t.Errorf("comment %d ends at recorded offset %d; the byte past its last is %d", i+1, got.Stop.Offset, wantEnd)
		}
	}
}

// TestACommentIsNotCollapsedTheWayABlankLineIs is requirement 4's contrast leg,
// and it is the requirement's own wording: blank lines are erased by the lexer
// and comments must not be.
//
// THE BLANK-LINE ARM IS THE CONTROL. It asserts the erasure it contrasts against
// is real in this tree rather than remembered from the ticket, so the comment arm
// means something.
func TestACommentIsNotCollapsedTheWayABlankLineIs(t *testing.T) {
	const plain = `flow orders
source events http.Listen[Order](":8080")
sink done audit.Store from events
`
	const withBlanks = `flow orders

source events http.Listen[Order](":8080")


sink done audit.Store from events
`
	const withComments = `flow orders
// one
source events http.Listen[Order](":8080")
// two
// three
sink done audit.Store from events
`

	blankStream, plainStream := tokenKinds(t, withBlanks), tokenKinds(t, plain)
	if len(blankStream) != len(plainStream) {
		t.Fatalf("CONTROL FAILED: blank lines are NOT collapsed in this tree — %d kinds against %d, so the contrast has no subject",
			len(blankStream), len(plainStream))
	}
	blankFile, err := Parse([]byte(withBlanks))
	if err != nil {
		t.Fatalf("the blank-line source does not parse: %v", err)
	}
	if len(blankFile.Comments) != 0 {
		t.Errorf("a source with no comment at all carries %d in its carrier", len(blankFile.Comments))
	}

	commentFile, err := Parse([]byte(withComments))
	if err != nil {
		t.Fatalf("the commented source does not parse: %v", err)
	}
	if len(commentFile.Comments) != 3 {
		t.Errorf("three comment lines yield %d in the carrier; a comment is not vertical whitespace", len(commentFile.Comments))
	}
	if got := tokenKinds(t, withComments); len(got) != len(plainStream) {
		t.Errorf("the commented source lexes to %d token kinds against the plain source's %d; a comment is trivia to the token stream",
			len(got), len(plainStream))
	}
}

// tokenKinds lexes src and returns every token kind it produced, EOF excluded.
func tokenKinds(t *testing.T, src string) []tokenKind {
	t.Helper()
	lex := newLexer([]byte(src))
	var out []tokenKind
	for tok := lex.next(); tok.kind != tokEOF; tok = lex.next() {
		out = append(out, tok.kind)
	}
	if len(out) == 0 {
		t.Fatalf("CONTROL FAILED: the lexer produced no token at all for:\n%s", src)
	}

	return out
}

// TestACommentAfterAStatementLineIsRecordedOnce is the speculative-read seam,
// and it is the one seam with no landed gate on it.
//
// THE MECHANISM. A clause-bearing statement's parse ends on a newline, and
// atClause then SAVES, advances one token past that newline to see whether the
// next line opens with a clause keyword, and RESTORES when it does not. A comment
// on the following line is scanned by that speculative advance and scanned again
// after the restore, so a carrier that does not ride the cursor the way the
// lexer's diagnostics already do records every such comment twice.
func TestACommentAfterAStatementLineIsRecordedOnce(t *testing.T) {
	const src = `flow orders
source events http.Listen[Order](":8080")
transform charge billing.Charge from events
  reads attempt
// this comment follows a clause-bearing statement
sink done audit.Store from charge
`
	if strings.Count(src, "//") != 1 {
		t.Fatalf("CONTROL FAILED: the source writes %d markers, not the single one this seam needs", strings.Count(src, "//"))
	}

	file, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("the source does not parse: %v", err)
	}
	if len(file.Comments) != 1 {
		t.Fatalf("the carrier holds %d comments for a source writing one; the speculative clause lookahead re-scanned it", len(file.Comments))
	}
	if got := file.Comments[0].Text; got != " this comment follows a clause-bearing statement" {
		t.Errorf("the carried comment is %q", got)
	}
}

// TestTheCommentCarrierIsASealedPositionedNode asserts the carrier satisfies the
// package's own node contract, which is what makes the position-fidelity walk
// reach it and what requirement 4 means by trivia the sealed-node gate admits.
func TestTheCommentCarrierIsASealedPositionedNode(t *testing.T) {
	var node Node = Comment{}
	if node.Pos() != (Position{}) || node.End() != (Position{}) {
		t.Errorf("a zero Comment reports a non-zero span: %+v..%+v", node.Pos(), node.End())
	}

	file, err := Parse([]byte(carrierSource))
	if err != nil {
		t.Fatalf("the carrier source does not parse: %v", err)
	}
	if len(file.Comments) == 0 {
		t.Fatalf("CONTROL FAILED: the carrier is empty, so the containment below checks nothing")
	}
	for i, comment := range file.Comments {
		if comment.Pos().Offset < file.Pos().Offset || comment.End().Offset > file.End().Offset {
			t.Errorf("comment %d spans %d..%d, outside the file's %d..%d",
				i+1, comment.Pos().Offset, comment.End().Offset, file.Pos().Offset, file.End().Offset)
		}
	}
}
