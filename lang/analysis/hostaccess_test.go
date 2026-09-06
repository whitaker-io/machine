// Package analysis - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package analysis

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/whitaker-io/machine/lang/ast"
)

// hostAccessDir holds this module's own host-access fixtures. The shared
// two-sided contract carries the ONE positive that lang/ast locks; every other
// axis lives here, because a near miss must produce NO diagnostic and the shared
// corpus is defined by every member producing one.
const hostAccessDir = "testdata/hostaccess"

// sharedHostFixture is the positive inside the closed contract.
const sharedHostFixture = sharedContractDir + "/host-accessor-in-func.flow"

// hostAccessDiags runs ONLY this analyzer over one fixture and keeps its own
// diagnostics, so a finding another analyzer reported cannot be read as this
// one's.
func hostAccessDiags(t *testing.T, path string) []Diagnostic {
	t.Helper()

	return withCode(analyze(t, HostAccessAnalyzer, loadSource(t, path)), HostAccessAnalyzer.Name)
}

// occurrencesOf is the test's OWN instrument for where a token sits in a .flow:
// a byte scan of the file, computed without going anywhere near the analyzer's
// mapping arithmetic. An expectation the subject supplies for itself proves
// nothing, so the two must agree by independent routes.
func occurrencesOf(t *testing.T, path, needle string) []ast.Position {
	t.Helper()

	body, err := os.ReadFile(path) //nolint:gosec // a test reading its own corpus
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}

	var out []ast.Position
	for at := 0; ; {
		i := bytes.Index(body[at:], []byte(needle))
		if i < 0 {
			break
		}
		offset := at + i
		out = append(out, ast.Position{
			Offset: offset,
			Line:   1 + bytes.Count(body[:offset], []byte("\n")),
			Col:    offset - bytes.LastIndex(body[:offset], []byte("\n")),
		})
		at = offset + len(needle)
	}
	if len(out) == 0 {
		t.Fatalf("CONTROL FAILED: %s carries no %q, so an expectation derived from it is empty", path, needle)
	}

	return out
}

// TestHostAccessNamesEveryAccessorSiteInTheFlowSource is requirement one: the
// diagnostic lands on the .flow's own line and column, not the synthetic
// prologue's.
func TestHostAccessNamesEveryAccessorSiteInTheFlowSource(t *testing.T) {
	want := occurrencesOf(t, sharedHostFixture, "captured.Host()")
	got := hostAccessDiags(t, sharedHostFixture)

	if len(got) != len(want) {
		t.Fatalf("%d hostaccess diagnostics over %s, want %d: %v",
			len(got), sharedHostFixture, len(want), messages(got))
	}
	for i, site := range want {
		// The report sits on the ACCESSOR, so the expectation is the Host token
		// inside the selector rather than the receiver it hangs off.
		accessor := ast.Position{
			Offset: site.Offset + len("captured."),
			Line:   site.Line,
			Col:    site.Col + len("captured."),
		}
		if got[i].Pos != accessor {
			t.Errorf("diagnostic %d sits at %v, want %v (%s)", i, got[i].Pos, accessor, got[i].Message)
		}
		if wantEnd := (ast.Position{
			Offset: accessor.Offset + len("Host()"),
			Line:   accessor.Line,
			Col:    accessor.Col + len("Host()"),
		}); got[i].End != wantEnd {
			t.Errorf("diagnostic %d ends at %v, want %v", i, got[i].End, wantEnd)
		}
		if got[i].Path != sharedHostFixture {
			t.Errorf("diagnostic %d names %q, want %q", i, got[i].Path, sharedHostFixture)
		}
		if got[i].Severity != SeverityError {
			t.Errorf("diagnostic %d is reported at %s, want error", i, got[i].Severity)
		}
		if !containsAll(got[i].Message, "Settle", "frame") {
			t.Errorf("diagnostic %d does not name the func and the legitimate route: %q", i, got[i].Message)
		}
	}

	// THE IN-FILE NEAR MISS. Tally sits in the same file and reaches storage
	// through the frame; a rule that fired on it would still satisfy every
	// assertion above.
	for _, d := range got {
		if strings.Contains(d.Message, "Tally") {
			t.Errorf("the frame-only func Tally in the same file was flagged: %q", d.Message)
		}
	}
}

// TestHostAccessMapsAColumnOnTheDeclarationLine covers the one line whose
// columns the reconstruction rewrote.
func TestHostAccessMapsAColumnOnTheDeclarationLine(t *testing.T) {
	path := filepath.Join(hostAccessDir, "one-line-body.flow")
	want := occurrencesOf(t, path, "captured.Host()")
	got := hostAccessDiags(t, path)

	if len(got) != 1 {
		t.Fatalf("%d hostaccess diagnostics over %s, want 1: %v", len(got), path, messages(got))
	}
	accessor := ast.Position{
		Offset: want[0].Offset + len("captured."),
		Line:   want[0].Line,
		Col:    want[0].Col + len("captured."),
	}
	if got[0].Pos != accessor {
		t.Errorf("the accessor is reported at %v, want %v", got[0].Pos, accessor)
	}
	// TWO CONTROLS ON THE FIXTURE, because this test is worth nothing if either
	// property drifts. The accessor must sit on the DECLARATION's own line,
	// which is the only line whose columns the reconstruction rewrote; and the
	// declaration must be INDENTED, because an unindented one puts the two texts
	// at identical columns and the remap this exercises becomes a no-op.
	body, err := os.ReadFile(path) //nolint:gosec // a test reading its own corpus
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}
	decl := bytes.Index(body, []byte("func Seed"))
	if declLine := 1 + bytes.Count(body[:decl], []byte("\n")); declLine != accessor.Line {
		t.Fatalf("CONTROL FAILED: %s declares Seed on line %d and calls the accessor on line %d; this fixture "+
			"exists to exercise the declaration line", path, declLine, accessor.Line)
	}
	if indent := decl - bytes.LastIndex(body[:decl], []byte("\n")) - 1; indent == 0 {
		t.Fatalf("CONTROL FAILED: %s declares Seed at column 1, so the reconstructed Go and the .flow agree "+
			"about this line's columns and the remap under test does nothing", path)
	}
}

// TestHostAccessFollowsTheReachIntoAWrapper is requirement one's wrapper class:
// the reach handed to a `go func()` and the reach handed to a func literal passed
// as a value, neither of which is a statement in the node body itself.
//
// THE GOROUTINE ARM IS THE RULING'S OWN RATIONALE. A runtime stack-walk guard was
// rejected as unsound because stack ancestry does not cross a goroutine boundary,
// so a `go func()` inside a node evades one; a static walk narrowed to the body's
// top-level statements would reintroduce exactly that blindness, and every other
// test in this file would stay green while it did.
func TestHostAccessFollowsTheReachIntoAWrapper(t *testing.T) {
	path := filepath.Join(hostAccessDir, "closure-reach.flow")
	want := occurrencesOf(t, path, "m.Host()")
	if len(want) != 2 {
		t.Fatalf("CONTROL FAILED: %s carries %d accessor reaches, want the two wrapper arms", path, len(want))
	}

	// THE ARMS ARE PINNED FROM THE FIXTURE'S OWN BYTES, so a fixture edit that
	// quietly turned both reaches into ordinary statements fails here rather than
	// leaving this test asserting the same thing twice.
	body, err := os.ReadFile(path) //nolint:gosec // a test reading its own corpus
	if err != nil {
		t.Fatalf("cannot read %s: %v", path, err)
	}
	goStmt := bytes.Index(body, []byte("\tgo func() {"))
	literal := bytes.Index(body, []byte("func(n int) int {"))
	if goStmt < 0 || literal < 0 || !(goStmt < want[0].Offset && want[0].Offset < literal && literal < want[1].Offset) {
		t.Fatalf("CONTROL FAILED: %s no longer carries one reach inside a `go func()` and one inside a func "+
			"literal passed as a value (go at %d, literal at %d, reaches at %d and %d)",
			path, goStmt, literal, want[0].Offset, want[1].Offset)
	}

	got := hostAccessDiags(t, path)
	if len(got) != len(want) {
		t.Fatalf("%d hostaccess diagnostics over %s, want %d — a walk that stopped at the body's top-level "+
			"statements reports neither: %v", len(got), path, len(want), messages(got))
	}
	for i, site := range want {
		// The report sits on the ACCESSOR, so the expectation is the Host token
		// inside the selector rather than the receiver it hangs off.
		accessor := ast.Position{
			Offset: site.Offset + len("m."),
			Line:   site.Line,
			Col:    site.Col + len("m."),
		}
		if got[i].Pos != accessor {
			t.Errorf("diagnostic %d sits at %v, want %v (%s)", i, got[i].Pos, accessor, got[i].Message)
		}
		if got[i].Severity != SeverityError {
			t.Errorf("diagnostic %d is reported at %s, want error", i, got[i].Severity)
		}
		if !containsAll(got[i].Message, "Settle", "frame") {
			t.Errorf("diagnostic %d does not name the func and the legitimate route: %q", i, got[i].Message)
		}
	}
}

// TestHostAccessStaysSilentOnItsNearMisses is requirement two: one case per axis
// the rule claims to discriminate on.
func TestHostAccessStaysSilentOnItsNearMisses(t *testing.T) {
	quiet := []string{"frame-path-only.flow", "host-named-otherwise.flow", "no-funcs.flow"}
	for _, name := range quiet {
		t.Run(name, func(t *testing.T) {
			if got := hostAccessDiags(t, filepath.Join(hostAccessDir, name)); len(got) != 0 {
				t.Errorf("%s produced %d hostaccess diagnostics, want none: %v", name, len(got), messages(got))
			}
		})
	}

	// THE KNOWN POSITIVE THROUGH THE SAME INSTRUMENT, in the same run. Without
	// it, silence over the near misses is what an analyzer that matches nothing
	// at all also produces.
	if got := hostAccessDiags(t, sharedHostFixture); len(got) == 0 {
		t.Fatal("CONTROL FAILED: the positive fixture produced nothing through this instrument, so the silences above prove nothing")
	}
}

// TestHostAccessReportsAnUnparseableBodyAtItsDeclaration is requirement three: a
// body that is not Go is a report, never a skip.
func TestHostAccessReportsAnUnparseableBodyAtItsDeclaration(t *testing.T) {
	path := filepath.Join(hostAccessDir, "unparseable-body.flow")
	got := hostAccessDiags(t, path)

	if len(got) != 1 {
		t.Fatalf("%d hostaccess diagnostics over %s, want 1: %v", len(got), path, messages(got))
	}
	decl := occurrencesOf(t, path, "Tally(f machine.Frame[Order])")[0]
	if got[0].Pos != decl {
		t.Errorf("the parse failure is reported at %v, want the declaration at %v", got[0].Pos, decl)
	}
	if got[0].Severity != SeverityError {
		t.Errorf("the parse failure is reported at %s, want error", got[0].Severity)
	}
	if !containsAll(got[0].Message, "Tally", "does not parse as Go") {
		t.Errorf("the diagnostic does not name the func and the failure: %q", got[0].Message)
	}
	// THE POSITION QUOTED IN THE TEXT IS THE .flow's, not the reconstruction's.
	// The COLUMN belongs to whichever token go/parser chose to complain about, so
	// the line is what this pins: the offending text sits well down the fixture
	// and only a few lines into the reconstructed span, so an untranslated
	// position is off by the whole preamble rather than by one line.
	offender := occurrencesOf(t, path, "this is not go")[0]
	_, quoted, cut := strings.Cut(got[0].Message, " at ")
	line, _, _ := strings.Cut(quoted, ":")
	if !cut || line != strconv.Itoa(offender.Line) {
		t.Errorf("the quoted failure position is on line %q, want the .flow's line %d; the message reads %q",
			line, offender.Line, got[0].Message)
	}
}

// TestHostAccessIsSilentAcrossTheSharedCorpus sweeps every .flow lang/ast owns.
//
// The floors are the point: a walk that stopped reading, or one whose matcher
// never fired, reports a clean corpus and reads exactly like a clean corpus.
func TestHostAccessIsSilentAcrossTheSharedCorpus(t *testing.T) {
	var walked, parsed, funcs int
	unparsed := []string{}
	flagged := map[string][]string{}

	err := filepath.WalkDir(astTestdata, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".flow" {
			return err
		}
		walked++

		body, readErr := os.ReadFile(path) //nolint:gosec // a test reading its own corpus
		if readErr != nil {
			t.Fatalf("cannot read %s: %v", path, readErr)
		}
		file, parseErr := ast.Parse(body)
		if parseErr != nil {
			// lang/ast's corpus deliberately holds unparseable .flow sources.
			// Recorded rather than skipped silently; the floors below are what
			// catch a sweep that parsed too little to mean anything.
			unparsed = append(unparsed, path)

			return nil
		}
		parsed++
		for _, decl := range file.Decls {
			if _, isFunc := decl.(ast.FuncDecl); isFunc {
				funcs++
			}
		}
		src := Source{Path: path, Src: body, File: file}
		for _, d := range withCode(analyze(t, HostAccessAnalyzer, src), HostAccessAnalyzer.Name) {
			flagged[path] = append(flagged[path], d.Message)
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", astTestdata, err)
	}

	if parsed < 20 || funcs < 10 {
		t.Fatalf("CONTROL FAILED: the sweep parsed %d of %d .flow sources and read %d func declarations; "+
			"a silence over that little is not evidence", parsed, walked, funcs)
	}
	if len(flagged[sharedHostFixture]) == 0 {
		t.Fatalf("CONTROL FAILED: the positive fixture was not flagged by the sweep, so every other silence is unproven")
	}
	for path, msgs := range flagged {
		if path != sharedHostFixture {
			t.Errorf("%s was flagged and is not the positive fixture: %v", path, msgs)
		}
	}
	t.Logf("swept %s: %d .flow sources, %d parsed, %d func declarations, %d unparsed %v; flagged only %s",
		astTestdata, walked, parsed, funcs, len(unparsed), unparsed, sharedHostFixture)
}

// TestHostAccessIsRegisteredWithADoc is requirement four's registry half. The
// disclosure phrases themselves are gated by corpus_test.go's required map, and
// the linter and the LSP publish out of this same registry.
func TestHostAccessIsRegisteredWithADoc(t *testing.T) {
	registered := All()
	if len(registered) == 0 {
		t.Fatal("CONTROL FAILED: the registry is empty, so a membership assertion would be vacuous")
	}
	if !slices.Contains(registered, HostAccessAnalyzer) {
		names := make([]string, 0, len(registered))
		for _, a := range registered {
			names = append(names, a.Name)
		}
		t.Fatalf("hostaccess is not registered, so neither flowlint nor the LSP can surface it; the registry holds %v", names)
	}
	if HostAccessAnalyzer.Doc == "" {
		t.Error("hostaccess carries an empty Doc, which the LSP roster refuses")
	}
}

// TestHostAccessReadsThroughTheSymbolTable is requirement six. The refusal is
// what makes it a real discriminator: an analyzer re-walking Source.File would
// not need the table at all and would happily report without one.
func TestHostAccessReadsThroughTheSymbolTable(t *testing.T) {
	if !slices.Contains(HostAccessAnalyzer.Requires, SymbolsAnalyzer) {
		t.Errorf("hostaccess requires %v, want the symbols analyzer among them", HostAccessAnalyzer.Requires)
	}

	src := loadSource(t, sharedHostFixture)
	_, err := HostAccessAnalyzer.Run(&Pass{
		Analyzer: HostAccessAnalyzer,
		Sources:  []Source{src},
		Report:   func(Source, Diagnostic) { t.Error("the analyzer reported without a symbol table") },
		ResultOf: map[*Analyzer]any{},
	})
	if !errors.Is(err, errNoSymbols) {
		t.Errorf("without the symbol table the analyzer answered %v, want errNoSymbols", err)
	}

	// AND THE RESULT IS THE SITES, keyed to the funcs the table carried.
	got, _ := resultOf(t, HostAccessAnalyzer, src)
	sites, ok := got.(*HostAccesses)
	if !ok {
		t.Fatalf("the analyzer produced %T, want *HostAccesses", got)
	}
	if len(sites.Sites) == 0 {
		t.Fatal("the result carries no sites over a fixture that reaches the accessor twice")
	}
	for _, site := range sites.Sites {
		if site.Func != "Settle" || site.Path != sharedHostFixture {
			t.Errorf("a site is attributed to %s in %s, want Settle in %s", site.Func, site.Path, sharedHostFixture)
		}
	}
}
