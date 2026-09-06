// Package lint - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package lint

import (
	"encoding/json"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/whitaker-io/machine/lang/analysis"
	"github.com/whitaker-io/machine/lang/ast"
)

// projection maps every exported field of analysis.Diagnostic to the JSON key or
// keys that carry it.
//
// IT IS THE LOSSLESS CLAIM, WRITTEN DOWN. WriteJSON's own doc says the document
// is "a lossless projection of the Diagnostic vocabulary, so a consumer of this
// format sees exactly what a consumer of the pass framework sees". A hand-written
// per-field assertion cannot keep that true, because a field ADDED to the
// framework's type is invisible to a test that lists the fields it knows about.
// This table is walked against the type by reflection in both directions.
var projection = map[string][]string{
	"Pos":      {"line", "col", "offset"},
	"End":      {"end_line", "end_col", "end_offset"},
	"Message":  {"message"},
	"Severity": {"severity"},
	"Code":     {"code"},
	"Path":     {"path"},
	"Foreign":  {"foreign"},
}

// goFinding is a diagnostic of the shape the consumer-Go host-reach check
// reports: a real Go file, a real position in it, and the foreign mark that says
// the run did not parse that file.
func goFinding() analysis.Diagnostic {
	return analysis.Diagnostic{
		Pos:      ast.Position{Offset: 412, Line: 24, Col: 14},
		End:      ast.Position{Offset: 418, Line: 24, Col: 20},
		Message:  "the node function WireOrders.func1, passed to Map at wire.go:22:19, reaches the accessor",
		Severity: analysis.SeverityError,
		Code:     "hostreach",
		Path:     "cmd/orders/wire.go",
		Foreign:  true,
	}
}

// TestEveryDiagnosticFieldHasAPlaceOnTheWire is the lossless claim as a gate.
//
// BOTH DIRECTIONS ARE CHECKED. A field on the framework's type with no key here
// is a field the JSON consumer silently loses; a key here that the document does
// not carry is a promise this test makes and the writer does not keep.
func TestEveryDiagnosticFieldHasAPlaceOnTheWire(t *testing.T) {
	typ := reflect.TypeOf(analysis.Diagnostic{})
	if typ.NumField() == 0 {
		t.Fatal("CONTROL FAILED: the diagnostic type has no fields, so this gate would pass vacuously")
	}

	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		if !field.IsExported() {
			continue
		}
		if _, carried := projection[field.Name]; !carried {
			t.Errorf("analysis.Diagnostic.%s has no key in the JSON projection, so a consumer of this "+
				"format no longer sees what a consumer of the pass framework sees", field.Name)
		}
	}

	var out strings.Builder
	if err := WriteJSON(&out, Result{Diagnostics: []analysis.Diagnostic{goFinding()}}); err != nil {
		t.Fatalf("write json: %v", err)
	}

	var document struct {
		Diagnostics []map[string]any `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(out.String()), &document); err != nil {
		t.Fatalf("the document does not decode: %v\n%s", err, out.String())
	}
	if len(document.Diagnostics) != 1 {
		t.Fatalf("the document carries %d diagnostics, want 1", len(document.Diagnostics))
	}

	onTheWire := document.Diagnostics[0]
	var promised []string
	for _, keys := range projection {
		for _, key := range keys {
			if _, present := onTheWire[key]; !present {
				t.Errorf("the projection promises the key %q and the document does not carry it", key)
			}
			promised = append(promised, key)
		}
	}
	sort.Strings(promised)
	if len(onTheWire) != len(promised) {
		t.Errorf("the document carries %d keys and the projection accounts for %d: %v vs %v",
			len(onTheWire), len(promised), sortedFieldNames(onTheWire), promised)
	}
}

// sortedFieldNames lists a decoded object's keys, for a failure message.
func sortedFieldNames(object map[string]any) []string {
	out := make([]string, 0, len(object))
	for key := range object {
		out = append(out, key)
	}
	sort.Strings(out)

	return out
}

// TestAConsumerGoFindingRendersItsRealFileInBothFormats is the rendering arm for
// a finding about a file the run did not parse.
//
// THE FILE, THE LINE AND THE COLUMN ARE READ OUT OF THE POSITION FIELDS, which is
// what the widened Diagnostic exists for. A finding that carried its file only
// inside the message would render here as a path of ":24:14", which is a position
// in nothing.
func TestAConsumerGoFindingRendersItsRealFileInBothFormats(t *testing.T) {
	finding := goFinding()
	result := Result{Diagnostics: []analysis.Diagnostic{finding}, Threshold: analysis.SeverityError, Failing: 1}

	var text strings.Builder
	if err := WriteText(&text, result); err != nil {
		t.Fatalf("write text: %v", err)
	}
	wantLine := "cmd/orders/wire.go:24:14: error: " + finding.Message + " [hostreach]"
	if !strings.Contains(text.String(), wantLine) {
		t.Errorf("the text rendering does not carry the Go file and its position:\n--- want line ---\n%s\n"+
			"--- got ---\n%s", wantLine, text.String())
	}

	var wire strings.Builder
	if err := WriteJSON(&wire, result); err != nil {
		t.Fatalf("write json: %v", err)
	}
	var document struct {
		Diagnostics []struct {
			Path      string `json:"path"`
			Line      int    `json:"line"`
			Col       int    `json:"col"`
			Offset    int    `json:"offset"`
			EndLine   int    `json:"end_line"`
			EndCol    int    `json:"end_col"`
			EndOffset int    `json:"end_offset"`
			Foreign   bool   `json:"foreign"`
		} `json:"diagnostics"`
	}
	if err := json.Unmarshal([]byte(wire.String()), &document); err != nil {
		t.Fatalf("the document does not decode: %v\n%s", err, wire.String())
	}

	got := document.Diagnostics[0]
	if got.Path != finding.Path || got.Line != 24 || got.Col != 14 || got.Offset != 412 {
		t.Errorf("the JSON rendering lost the position: %+v", got)
	}
	if got.EndLine != 24 || got.EndCol != 20 || got.EndOffset != 418 {
		t.Errorf("the JSON rendering lost the end position: %+v", got)
	}
	if !got.Foreign {
		t.Error("the JSON rendering does not say the file is one the run did not parse")
	}
}

// TestTheTwoSortsStayAgreedAcrossFlowAndGoPaths pins the second copy of the sort
// key against the first.
//
// THE COPY IS DELIBERATE AND UNJOINED. analysis.Run orders what IT returns, and
// this package merges parse diagnostics in afterwards, so it sorts again with its
// own copy of the key. Nothing but a test holds the two together, and a mixed run
// — .flow findings beside a consumer-Go one — is exactly where an order that
// disagreed would show up as flowlint printing a different sequence from the gate.
//
// BOTH SIDES ARE REAL. The expected order is the one analysis.Run itself produced
// over real parsed sources, not a second statement of the key here; this package's
// sort is then handed that same set, reversed, and must put it back.
func TestTheTwoSortsStayAgreedAcrossFlowAndGoPaths(t *testing.T) {
	batch, err := Load([]string{filepath.Join(astTestdata, "strawman")})
	if err != nil {
		t.Fatalf("load the strawman corpus: %v", err)
	}
	if len(batch.Sources) < 2 {
		t.Fatalf("CONTROL FAILED: the corpus loaded %d sources, and an order needs at least two",
			len(batch.Sources))
	}

	mixing := &analysis.Analyzer{
		Name: "mixing",
		Doc:  "mixing reports one finding per source and one about a Go file the run did not parse",
		Run: func(p *analysis.Pass) (any, error) {
			for _, src := range p.Sources {
				p.Report(src, analysis.Diagnostic{
					Pos:     ast.Position{Offset: 3, Line: 1, Col: 4},
					End:     ast.Position{Offset: 9, Line: 1, Col: 10},
					Message: "about the flow",
				})
			}
			p.ReportForeign(goFinding())

			return nil, nil
		},
	}

	ordered, err := analysis.Run(batch.Sources, []*analysis.Analyzer{mixing})
	if err != nil {
		t.Fatalf("the analysis run failed: %v", err)
	}
	if len(ordered) < 3 {
		t.Fatalf("CONTROL FAILED: the run reported %d diagnostics, too few to order", len(ordered))
	}

	reversed := make([]analysis.Diagnostic, 0, len(ordered))
	for i := len(ordered) - 1; i >= 0; i-- {
		reversed = append(reversed, ordered[i])
	}
	sortDiagnostics(reversed)

	if !reflect.DeepEqual(reversed, ordered) {
		t.Errorf("this package's sort disagrees with the analysis driver's over a mixed run:\n"+
			"--- lint ---\n%v\n--- analysis ---\n%v", rendered(reversed), rendered(ordered))
	}
	t.Logf("both orders: %v", rendered(ordered))
}

// rendered projects diagnostics onto their sort key, for a failure message.
func rendered(diags []analysis.Diagnostic) []string {
	out := make([]string, 0, len(diags))
	for _, d := range diags {
		out = append(out, d.Path+":"+d.Pos.String()+" ["+d.Code+"]")
	}

	return out
}
