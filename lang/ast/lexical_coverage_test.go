// Package ast - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package ast

import (
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"
)

// lexicalTerminalEntry matches one entry of the grammar header's LEXICAL
// TERMINALS block: a comment line whose name sits at exactly three spaces of
// indent and is followed by its prose.
//
// THE INDENT IS THE DISCRIMINATOR. A terminal's continuation lines are indented
// far further so that the prose wraps under the description rather than under
// the name, which is what lets an entry be told from a continuation without
// pinning how many entries or how many wrapped lines there are.
var lexicalTerminalEntry = regexp.MustCompile(`^//   ([a-zA-Z][A-Za-z0-9]*)  +\S`)

// declaredLexicalTerminals returns the terminal names the grammar's own header
// declares, in the order it declares them.
//
// IT READS THE GRAMMAR rather than restating it, so a terminal added to the
// language widens what the spec must demonstrate without anyone remembering to
// edit a list here — the same derivation discipline TestSpecExamplesCoverEvery
// StatementForm applies to the Statement production.
func declaredLexicalTerminals(t *testing.T) []string {
	t.Helper()
	raw := string(readFixture(t, grammarPath))

	var out []string
	inBlock := false
	for _, line := range strings.Split(raw, "\n") {
		if strings.HasPrefix(line, "// LEXICAL TERMINALS") {
			inBlock = true

			continue
		}
		if !inBlock {
			continue
		}
		if match := lexicalTerminalEntry.FindStringSubmatch(line); match != nil {
			out = append(out, match[1])

			continue
		}
		// The block ends at the first unindented prose line that follows an
		// entry: the header's own paragraphs resume at one space of indent.
		if len(out) > 0 && strings.HasPrefix(line, "// ") && !strings.HasPrefix(line, "//   ") {
			break
		}
	}

	return out
}

// observeTerminals returns the lexical terminals one example exercises.
//
// IT TAKES THREE INSTRUMENTS BECAUSE THE TERMINALS ARE NOT ALL OBSERVABLE THE
// SAME WAY, and a census that used one of them alone would report a confident
// zero for the terminals the other two see:
//
//   - THE RAW LEXER WALK observes every terminal the scanner produces unprompted:
//     ident, string, number, newline and noteText. This is the instrument the
//     keyword census uses.
//   - THE PARSED TREE observes goSpan and goFuncSpan, which a raw walk can NEVER
//     reach. A span scan is parser-driven: the lexer enters one only when the
//     parser asks it to at a grammar position, so walking next() to EOF over a
//     file full of Go operands produces not one span token.
//   - THE CARRIER observes comment, which is trivia consumed before any token is
//     returned and so is likewise invisible to a token walk.
func observeTerminals(t *testing.T, src string) map[string]bool {
	t.Helper()
	seen := map[string]bool{}

	lex := newLexer([]byte(src))
	kinds := map[tokenKind]bool{}
	for tok := lex.next(); tok.kind != tokEOF; tok = lex.next() {
		kinds[tok.kind] = true
	}
	for name, kind := range lexicalKinds {
		if kinds[kind] {
			seen[name] = true
		}
	}
	if len(lex.comments) > 0 {
		seen["comment"] = true
	}

	file, err := Parse([]byte(src))
	if err != nil {
		t.Fatalf("an example does not parse, so its spans could not be observed: %v\n%s", err, src)
	}
	observeSpans(reflect.ValueOf(file), seen)

	return seen
}

// observeSpans records the two span terminals from a parsed tree.
//
// A FUNC DECLARATION'S BODY IS THE ONLY goFuncSpan in the language — parser.go
// FuncSpan has exactly one caller, the func declaration parser — so the owner
// tells the two apart and no separate marker is needed on GoSpan itself.
func observeSpans(v reflect.Value, seen map[string]bool) {
	switch v.Kind() {
	case reflect.Pointer, reflect.Interface:
		if !v.IsNil() {
			observeSpans(v.Elem(), seen)
		}
	case reflect.Slice, reflect.Array:
		for i := range v.Len() {
			observeSpans(v.Index(i), seen)
		}
	case reflect.Struct:
		isFunc := v.Type() == reflect.TypeOf(FuncDecl{})
		for i := range v.NumField() {
			field := v.Field(i)
			if field.Type() == reflect.TypeOf(GoSpan{}) {
				if span, ok := field.Interface().(GoSpan); ok && span.Text != "" {
					seen[spanTerminal(isFunc, v.Type().Field(i).Name)] = true
				}

				continue
			}
			observeSpans(field, seen)
		}
	default:
	}
}

// spanTerminal names which of the two span terminals a GoSpan field carries.
func spanTerminal(ownerIsFunc bool, field string) string {
	if ownerIsFunc && field == "Body" {
		return "goFuncSpan"
	}

	return "goSpan"
}

// TestSpecExamplesCoverEveryLexicalTerminal asserts the spec demonstrates every
// lexical form the grammar declares.
//
// WHY IT EXISTS. The landed coverage gate beside it derives its wanted set from
// the grammar's Statement production, and a comment is not a statement — so
// before this gate, nothing required the spec to show a comment at all. The
// document is handed to a model as the language's whole in-context definition,
// and a form the spec never demonstrates is a form the model never writes.
//
// IT EXEMPTS NOTHING. Every terminal the grammar declares is demonstrated by
// some example, so the wanted set is the declared set and there is no by-name
// escape for a reader to widen later.
func TestSpecExamplesCoverEveryLexicalTerminal(t *testing.T) {
	wanted := declaredLexicalTerminals(t)
	if len(wanted) == 0 {
		t.Fatalf("CONTROL FAILED: no lexical terminal was derived from %s, so this gate would require nothing", grammarPath)
	}
	for _, name := range wanted {
		if _, known := lexicalKinds[name]; !known && name != "comment" {
			t.Errorf("%s declares the lexical terminal %q that no instrument here can observe", grammarPath, name)
		}
	}

	examples := specExamples(t)
	if len(examples) == 0 {
		t.Fatalf("CONTROL FAILED: %s quotes no flow example at all", specPath)
	}

	seen := map[string]bool{}
	for _, src := range examples {
		for name := range observeTerminals(t, src) {
			seen[name] = true
		}
	}
	if len(seen) == 0 {
		t.Fatalf("CONTROL FAILED: the walk observed no terminal at all across %d examples", len(examples))
	}

	var missing []string
	for _, name := range wanted {
		if !seen[name] {
			missing = append(missing, name)
		}
	}
	if len(missing) > 0 {
		slices.Sort(missing)
		t.Errorf("no example in %s exercises these lexical terminals the grammar declares: %v", specPath, missing)
	}

	t.Logf("%d examples exercise %d of the %d lexical terminals %s declares", len(examples), len(seen), len(wanted), grammarPath)
}
