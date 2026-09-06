// Package assembler - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package assembler

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"github.com/whitaker-io/machine/lang/analysis"
	"github.com/whitaker-io/machine/lang/ast"
)

// leakingWiring is hand-written consumer Go beside the .flow: a node function
// that reaches the machine's host-side store accessor.
//
// IT IS WRITTEN THE WAY A CONSUMER WOULD WRITE IT — an inline closure over the
// machine, passed to a real builder method — because the analysis under test
// resolves both the constructor and the accessor through go/types against the
// real runtime. A stub would prove the gate against a fixture rather than against
// the runtime.
const leakingWiring = `package probe

import (
	"context"

	machine "github.com/whitaker-io/machine/v4"
)

var counter = machine.NewCell[int]("counter")

func WireOrders(m *machine.Machine) {
	ingest, _ := m.Source[Order]("ingest")
	charged := ingest.Map("charge", func(f machine.Frame[Order]) Order {
		order := f.Value()
		if held, ok, err := m.Host().Load(context.Background(), counter); err == nil && ok {
			_ = held
		}

		return order
	})
	charged.Drop("charge#drain")
}
`

// cleanWiring is the same file with the reach taken out, and it is the CONTROL:
// without it, a refusal below could be the fixture failing to generate for any
// other reason.
const cleanWiring = `package probe

import (
	machine "github.com/whitaker-io/machine/v4"
)

func WireOrders(m *machine.Machine) {
	ingest, _ := m.Source[Order]("ingest")
	charged := ingest.Map("charge", func(f machine.Frame[Order]) Order {
		return f.Value()
	})
	charged.Drop("charge#drain")
}
`

// realDriver is a driver with nothing injected: the real loader, the real gate,
// and no caller-supplied facts. It is what makes this a seam test rather than a
// unit test with a double on the far side.
func realDriver() *Driver {
	return &Driver{Config: Config{Package: "probe", Qualifier: "acme"}, PackagePath: "probe"}
}

// TestAConsumerGoHostReachRefusesGenerationCarryingItsRealFileAndPosition is the
// seam: a real analysis.Diagnostic on one side, a real assembler.Diagnostic on
// the other, and a real generation run around both.
//
// THE CONVERSION IS THE THING UNDER TEST. gate hands analysis diagnostics to
// partition, which rebuilds each one as this package's own type field by field; a
// field that stops there is a file, a line and a column the author never sees,
// while every other test in both modules stays green. Both directions are
// measured in one run: the violating fixture is refused and names its Go file,
// and the same fixture with the reach removed generates.
func TestAConsumerGoHostReachRefusesGenerationCarryingItsRealFileAndPosition(t *testing.T) {
	in := driverDir(t, map[string]string{"orders.flow": driverFlow, "wire.go": leakingWiring})
	out := t.TempDir()

	err := realDriver().Generate(in, out)
	if err == nil {
		t.Fatal("a run whose consumer Go reaches the host accessor generated successfully")
	}

	var refusal *Error
	if !errors.As(err, &refusal) {
		t.Fatalf("the refusal is %T, want an *Error carrying diagnostics: %v", err, err)
	}

	var found *Diagnostic
	for i := range refusal.Diagnostics {
		if strings.HasSuffix(refusal.Diagnostics[i].Path, "wire.go") {
			found = &refusal.Diagnostics[i]
		}
	}
	if found == nil {
		t.Fatalf("no refusal names the consumer Go file; the run refused with %v", rendered(refusal.Diagnostics))
	}
	if found.Pos.Line == 0 || found.Pos.Col == 0 {
		t.Errorf("the refusal carries no position in %s: %s", found.Path, found.Pos)
	}
	if !found.Foreign {
		t.Errorf("the refusal does not carry the mark that says its file is not one of the run's .flow "+
			"sources, so a caller cannot tell which kind of file it names: %+v", found)
	}
	if found.End.Offset <= found.Pos.Offset {
		t.Errorf("the refusal spans nothing: %s..%s", found.Pos, found.End)
	}

	written, globErr := filepath.Glob(filepath.Join(out, "*.flow.go"))
	if globErr != nil {
		t.Fatalf("reading the output directory: %v", globErr)
	}
	if len(written) != 0 {
		t.Errorf("a refused run wrote %v", written)
	}
	t.Logf("refused at %s:%s: %s", found.Path, found.Pos, found.Message)

	// THE CONTROL, in the same test: the identical fixture without the reach
	// generates, so the refusal above is the reach rather than the fixture.
	clean := driverDir(t, map[string]string{"orders.flow": driverFlow, "wire.go": cleanWiring})
	if err := realDriver().Generate(clean, t.TempDir()); err != nil {
		t.Fatalf("CONTROL FAILED: the same fixture without the host reach did not generate: %v", err)
	}
}

// TestAHandBuiltMachineWithNoFlowIsNotGatedAtAll is the limit the check's Doc
// discloses, measured rather than only stated.
//
// THE CORPUS IS THE PACKAGE SET A GENERATION RUN LOADS, and a directory with no
// .flow in it is not a generation run: nothing loads the packages, so nothing
// reads the Go. A consumer who builds their machine by hand is therefore ungated
// by this check — which is a real gap, disclosed in the analyzer's own Doc, and
// this is the arm that keeps the disclosure true. The violating file is byte for
// byte the one the seam test above refuses.
func TestAHandBuiltMachineWithNoFlowIsNotGatedAtAll(t *testing.T) {
	in := driverDir(t, map[string]string{"wire.go": leakingWiring})
	out := t.TempDir()

	err := realDriver().Generate(in, out)

	var refusal *Error
	if errors.As(err, &refusal) {
		t.Fatalf("a directory with no .flow was refused by the analysis gate: %v", rendered(refusal.Diagnostics))
	}
	// Whatever the driver does with a directory that declares no flows, it is not
	// a host-reach refusal — that is the whole claim, and the message is logged so
	// a later reader sees what it actually is.
	t.Logf("a directory holding only the violating Go generated with: %v", err)
}

// TestTheAssemblersOwnErrorRendersTheFileOfAForeignDiagnostic pins the one
// rendering this package owns.
//
// (*Error).Error renders the FIRST diagnostic's position and message. For this
// package's own refusals the caller knows the file — it handed the sources in —
// and the position alone is the right answer, which is why Path is documented as
// empty there. A finding about a file the run never parsed is the opposite case:
// a line and a column with no file name send a reader nowhere.
func TestTheAssemblersOwnErrorRendersTheFileOfAForeignDiagnostic(t *testing.T) {
	own := &Error{Diagnostics: []Diagnostic{{
		Pos: ast.Position{Offset: 10, Line: 3, Col: 5}, Message: "a from-name nothing declares",
	}}}
	if got := own.Error(); got != "3:5: a from-name nothing declares" {
		t.Errorf("this package's own refusal renders %q; the caller's file name is the right answer there", got)
	}

	foreign := &Error{Diagnostics: []Diagnostic{{
		Pos:     ast.Position{Offset: 412, Line: 24, Col: 14},
		Message: "the node function WireOrders.func1 reaches the accessor",
		Path:    "/work/orders/cmd/wire.go",
		Foreign: true,
	}}}
	got := foreign.Error()
	if !strings.Contains(got, "/work/orders/cmd/wire.go:24:14") {
		t.Errorf("a refusal about a file the run did not parse renders %q, naming no file", got)
	}
	t.Logf("rendered: %s", got)
}

// TestPartitionCarriesEveryFieldOfTheAnalysisDiagnostic is the conversion in
// isolation, beside the seam run above.
//
// IT LISTS EACH CARRIED FIELD BY HAND; it is not a reflective census. partition
// rebuilds the value field by field, so it catches a field the conversion drops
// among those named here. A field the framework GROWS and this conversion never
// mentions is invisible to this test and is caught instead by the linter's
// wire-projection test, which walks analysis.Diagnostic reflectively; the two
// are extended together when the framework's type grows.
func TestPartitionCarriesEveryFieldOfTheAnalysisDiagnostic(t *testing.T) {
	source := analysis.Diagnostic{
		Pos:      ast.Position{Offset: 412, Line: 24, Col: 14},
		End:      ast.Position{Offset: 418, Line: 24, Col: 20},
		Message:  "reaches the accessor",
		Severity: analysis.SeverityError,
		Code:     "hostreach",
		Path:     "/work/orders/cmd/wire.go",
		Foreign:  true,
	}

	refused, disclosed := partition([]analysis.Diagnostic{source})
	if len(refused) != 1 || len(disclosed) != 0 {
		t.Fatalf("partition split %d refused and %d disclosed, want 1 and 0", len(refused), len(disclosed))
	}

	got := refused[0]
	if got.Pos != source.Pos || got.End != source.End || got.Message != source.Message {
		t.Errorf("the conversion lost a position or the message: %+v", got)
	}
	if got.Path != source.Path {
		t.Errorf("the conversion lost the file: %q", got.Path)
	}
	if !got.Foreign {
		t.Error("the conversion lost the mark that says the file is not one of the run's own sources")
	}
}

// rendered projects diagnostics for a failure message.
func rendered(diags []Diagnostic) []string {
	out := make([]string, 0, len(diags))
	for _, d := range diags {
		out = append(out, d.Path+":"+d.Pos.String()+": "+d.Message)
	}

	return out
}
