// Package analysis - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package analysis

import (
	"strings"
	"testing"

	"github.com/whitaker-io/machine/lang/ast"
)

// consumerGoFile is a path of the kind an analyzer reading hand-written Go
// reports about: not a .flow source, and not one of the run's Sources.
const consumerGoFile = "cmd/wire/wire.go"

// reporter builds an analyzer that reports one diagnostic through Report and one
// through ReportForeign, so both stamping paths are exercised by ONE run.
func reporter(name string, foreign Diagnostic) *Analyzer {
	return &Analyzer{
		Name: name,
		Doc:  name + " reports one finding about the source it was given and one about a file it read itself",
		Run: func(p *Pass) (any, error) {
			for _, src := range p.Sources {
				p.Report(src, Diagnostic{Pos: position(1), End: position(2), Message: "about the flow"})
			}
			p.ReportForeign(foreign)

			return nil, nil
		},
	}
}

// TestAForeignFindingKeepsTheFileTheAnalyzerNamedAndAFlowFindingDoesNot is the
// widening's whole contract at the driver.
//
// BOTH ARMS RUN IN ONE PASS deliberately. The existing behavior — every finding
// attributed to the Source that reported it — is what every registered analyzer
// depends on, and a change that only added the new arm could silently drop it.
func TestAForeignFindingKeepsTheFileTheAnalyzerNamedAndAFlowFindingDoesNot(t *testing.T) {
	src := parseSource(t, "run.flow", "flow Run\nsource ingest Poll\nsink done Store from ingest\n")
	foreign := Diagnostic{
		Pos:      ast.Position{Offset: 400, Line: 12, Col: 14},
		End:      ast.Position{Offset: 406, Line: 12, Col: 20},
		Message:  "a node function reaches the host accessor",
		Severity: SeverityError,
		Path:     consumerGoFile,
	}

	diags, err := Run([]Source{src}, []*Analyzer{reporter("widening", foreign)})
	if err != nil {
		t.Fatalf("the run failed: %v", err)
	}
	if len(diags) != 2 {
		t.Fatalf("the run reported %d diagnostics, want 2: %v", len(diags), messages(diags))
	}

	var flowFinding, goFinding Diagnostic
	for _, d := range diags {
		if d.Foreign {
			goFinding = d

			continue
		}
		flowFinding = d
	}

	if goFinding.Path != consumerGoFile {
		t.Errorf("the foreign finding is attributed to %q, want the file the analyzer read, %q",
			goFinding.Path, consumerGoFile)
	}
	if goFinding.Pos.Line != 12 || goFinding.Pos.Col != 14 {
		t.Errorf("the foreign finding is positioned at %s, want 12:14 — the position the analyzer supplied",
			goFinding.Pos)
	}
	if goFinding.Code != "widening" {
		t.Errorf("the foreign finding carries code %q, want the reporting analyzer's name", goFinding.Code)
	}

	if flowFinding.Path != src.Path {
		t.Errorf("the flow finding is attributed to %q, want the reporting source %q", flowFinding.Path, src.Path)
	}
	if flowFinding.Foreign {
		t.Error("a finding reported through Report is marked foreign, so the two paths are not distinguishable")
	}
	t.Logf("one run, two attributions: %v", messages(diags))
}

// TestAnAnalyzerCannotForgeTheForeignMarkOnASourceFinding pins that the DRIVER
// owns the mark, exactly as it owns the Code.
//
// An analyzer that sets Foreign on a diagnostic it reports through Report would
// otherwise keep the mark while the driver overwrote its path, producing a
// finding that claims to be about a file it is not about.
func TestAnAnalyzerCannotForgeTheForeignMarkOnASourceFinding(t *testing.T) {
	src := parseSource(t, "run.flow", "flow Run\nsource ingest Poll\nsink done Store from ingest\n")
	forging := &Analyzer{
		Name: "forging",
		Doc:  "forging claims a finding is about a file it did not read",
		Run: func(p *Pass) (any, error) {
			p.Report(p.Sources[0], Diagnostic{
				Pos: position(1), End: position(2), Message: "forged", Path: consumerGoFile, Foreign: true,
			})

			return nil, nil
		},
	}

	diags, err := Run([]Source{src}, []*Analyzer{forging})
	if err != nil {
		t.Fatalf("the run failed: %v", err)
	}
	if len(diags) != 1 {
		t.Fatalf("the run reported %d diagnostics, want 1", len(diags))
	}
	if diags[0].Foreign || diags[0].Path != src.Path {
		t.Errorf("a forged foreign mark survived Report: %+v", diags[0])
	}
}

// TestAForeignFindingWithNoFileIsRefusedRatherThanAttributedToNothing pins the
// one invalid shape the widening makes expressible.
//
// A foreign finding names its own file BECAUSE NOTHING ELSE CAN. An empty path
// there is not a degraded answer, it is a finding about nowhere: it sorts to the
// front of the run, renders as ":12:14:" and points a reader at no file at all.
func TestAForeignFindingWithNoFileIsRefusedRatherThanAttributedToNothing(t *testing.T) {
	src := parseSource(t, "run.flow", "flow Run\nsource ingest Poll\nsink done Store from ingest\n")
	unnamed := &Analyzer{
		Name: "unnamed",
		Doc:  "unnamed reports a foreign finding without naming the file it read",
		Run: func(p *Pass) (any, error) {
			p.ReportForeign(Diagnostic{Pos: position(1), End: position(2), Message: "about nowhere"})

			return nil, nil
		},
	}

	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatal("a foreign finding naming no file was accepted, so a run can report a finding about nowhere")
		}
		text, isString := recovered.(string)
		if !isString || !strings.Contains(text, "unnamed") {
			t.Errorf("the refusal does not name the analyzer that reported it: %v", recovered)
		}
		t.Logf("refused: %v", recovered)
	}()

	_, _ = Run([]Source{src}, []*Analyzer{unnamed})
}

// TestTheOrderIsStableWhenAFlowFindingAndAGoFindingMeet pins the sort key over
// the mixed run the widening makes possible.
//
// THE KEY LEADS WITH PATH so the order is a function of the content rather than
// of the order the caller listed its sources in, and a foreign path is just
// another path to it. Reversing the sources must not reverse the output.
func TestTheOrderIsStableWhenAFlowFindingAndAGoFindingMeet(t *testing.T) {
	first := parseSource(t, "a.flow", "flow A\nsource ingest Poll\nsink done Store from ingest\n")
	second := parseSource(t, "b.flow", "flow B\nsource ingest Poll\nsink done Store from ingest\n")
	foreign := Diagnostic{
		Pos: ast.Position{Offset: 0, Line: 1, Col: 1}, End: ast.Position{Offset: 1, Line: 1, Col: 2},
		Message: "go finding", Path: "a_go_file.go",
	}

	forward, err := Run([]Source{first, second}, []*Analyzer{reporter("ordering", foreign)})
	if err != nil {
		t.Fatalf("the forward run failed: %v", err)
	}
	reversed, err := Run([]Source{second, first}, []*Analyzer{reporter("ordering", foreign)})
	if err != nil {
		t.Fatalf("the reversed run failed: %v", err)
	}

	if strings.Join(messages(forward), "\n") != strings.Join(messages(reversed), "\n") {
		t.Errorf("reversing the sources changed the order:\n--- forward ---\n%s\n--- reversed ---\n%s",
			strings.Join(messages(forward), "\n"), strings.Join(messages(reversed), "\n"))
	}
	t.Logf("stable across both orders: %v", messages(forward))
}
