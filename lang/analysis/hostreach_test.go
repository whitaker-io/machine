// Package analysis - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package analysis

import (
	"errors"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/whitaker-io/machine/lang/loader"
)

// The consumer-Go fixture module: a real module on disk that imports the real
// runtime, because both halves of the question — is this callee a node
// constructor, is this call the host accessor — are answered by resolving
// objects the runtime declares.
const (
	hostReachSubject = "testdata/hostreach/subject"
	hostReachPkg     = "example.com/hostreach/subject"
	hostReachSecond  = "example.com/hostreach/subject/generated"
)

// wantedReaches is the EXACT set the analyzer must report over the fixture,
// written as "file:line:col func -> constructor".
//
// IT IS A SET RATHER THAN A COUNT. Two sets that lost the same member are the
// same size, so a count would pass an analyzer that dropped the aliased arm and
// gained a false positive on a near miss. The order is the STRING sort of these
// lines rather than the file order, which is why 257 sits above 38.
var wantedReaches = []string{
	"aliased.go:21:25 WireAliased.func1 -> Map",
	"dotimported.go:22:25 WireDotImported.func1 -> Map",
	"generated.go:29:25 WireGenerated.func1 -> Map",
	"helpers.go:29:29 EnrichFromHost -> Map",
	"wiring.go:258:27 box.Wire.func1 -> Map",
	"wiring.go:39:25 WireCapturing.func1 -> Map",
	"wiring.go:60:19 EnrichFromHost -> Map",
	"wiring.go:77:24 wiring.Route.func1 -> If",
	"wiring.go:91:13 WireTeeing.func1 -> Tee",
}

// loadHostReachSubject loads the consumer-Go fixture module.
//
// IT ASSERTS pkgs.Errors() IS EMPTY AS A CONTROL. A fixture module that does not
// type-check resolves nothing, so every silence below would pass for the wrong
// reason: the analyzer would be reporting nothing because it could resolve
// nothing, which is indistinguishable from a clean corpus in the result.
func loadHostReachSubject(t *testing.T) *loader.Packages {
	t.Helper()

	pkgs, err := loader.Load(hostReachSubject, []string{"./..."})
	if err != nil {
		t.Fatalf("the host-reach fixture module did not load: %v", err)
	}
	if problems := pkgs.Errors(); len(problems) != 0 {
		t.Fatalf("CONTROL FAILED: the fixture module does not type-check, so no verdict below means anything: %v",
			problems)
	}

	return pkgs
}

// reachesOf runs the analyzer through the driver and returns its result with the
// diagnostics of the same run.
func reachesOf(t *testing.T) (*HostReaches, []Diagnostic) {
	t.Helper()

	got, diags := resultOf(t, HostReachAnalyzer(loadHostReachSubject(t)))
	reaches, ok := got.(*HostReaches)
	if !ok {
		t.Fatalf("the host-reach analyzer produced %T, want *HostReaches", got)
	}

	return reaches, diags
}

// rendered projects a reach onto the line wantedReaches is written in.
func rendered(site HostReach) string {
	return filepath.Base(site.Path) + ":" + strconv.Itoa(site.Pos.Line) + ":" + strconv.Itoa(site.Pos.Col) +
		" " + site.Func + " -> " + site.Constructor
}

// TestEveryNodeFunctionReachingTheAccessorIsReportedAndEveryNearMissIsSilent is
// the analyzer's whole corpus contract in one run.
//
// THE SIX POSITIVES ARE SIX DIFFERENT WAYS OF WRITING ONE VIOLATION: an inline
// closure capturing the machine, a named func passed by name reaching a
// package-level machine, a Filter reaching it through a struct field, a Tee whose
// Duplicator takes the PAYLOAD rather than a Frame, an aliased import and a dot
// import. The near misses beside them are the sanctioned frame path, a host
// caller outside every constructor, an ErrorHandler body, an EdgeFactory, a
// capability helper that takes a node function it never wires, and a local method
// named Map.
func TestEveryNodeFunctionReachingTheAccessorIsReportedAndEveryNearMissIsSilent(t *testing.T) {
	reaches, diags := reachesOf(t)

	got := make([]string, 0, len(reaches.Sites))
	for _, site := range reaches.Sites {
		got = append(got, rendered(site))
	}
	sort.Strings(got)

	if strings.Join(got, "\n") != strings.Join(wantedReaches, "\n") {
		t.Errorf("the reported set is not the expected one:\n--- got ---\n%s\n--- want ---\n%s",
			strings.Join(got, "\n"), strings.Join(wantedReaches, "\n"))
	}

	reported := withCode(diags, hostReachName)
	if len(reported) != len(wantedReaches) {
		t.Errorf("the run reported %d diagnostics under %s, want %d: %v",
			len(reported), hostReachName, len(wantedReaches), messages(reported))
	}
	t.Logf("reported: %v", messages(reported))
}

// TestTheWalkReachedTheWholeCorpusBeforeAnySilenceIsBelieved is the control every
// silence above rests on.
//
// A WALK THAT READ NOTHING REPORTS NOTHING, and that result is byte-identical to
// a clean corpus. So the analyzer records what it read: the packages it walked,
// the files it parsed and every node function it identified, by name. The floors
// are asserted, and the two near-miss node functions are asserted to have been
// INSPECTED — a silence about a function the walk never reached is not evidence.
func TestTheWalkReachedTheWholeCorpusBeforeAnySilenceIsBelieved(t *testing.T) {
	reaches, _ := reachesOf(t)

	if len(reaches.Packages) < 2 || !slices.Contains(reaches.Packages, hostReachPkg) {
		t.Errorf("the walk read %v, want at least the fixture package %s and its generated sibling",
			reaches.Packages, hostReachPkg)
	}
	if reaches.Files < 4 {
		t.Errorf("the walk read %d files, and the fixture module holds four", reaches.Files)
	}

	inspected := map[string]bool{}
	for _, fn := range reaches.Inspected {
		inspected[fn.Func] = true
	}
	// The near misses that ARE node functions: a clean verdict about them is only
	// evidence if the walk identified them as node functions in the first place.
	for _, name := range []string{"WireFramePath.func1", "WireHandled.func1"} {
		if !inspected[name] {
			t.Errorf("the walk never identified %s as a node function, so its silence proves nothing; "+
				"it identified %v", name, sortedKeys(inspected))
		}
	}
	t.Logf("walked %d packages and %d files, inspecting %d node functions: %v",
		len(reaches.Packages), reaches.Files, len(reaches.Inspected), sortedKeys(inspected))
}

// TestTheReportNamesTheFileTheLineAndTheColumnStructurally is requirement 1.
//
// THE ASSERTION IS ON THE FIELDS, not on the message text. A diagnostic that
// carried the file only inside its Message would satisfy a reader and fail every
// consumer that positions a finding: the linter's text and JSON renderings, the
// assembler's conversion, an editor's range.
func TestTheReportNamesTheFileTheLineAndTheColumnStructurally(t *testing.T) {
	_, diags := reachesOf(t)

	reported := withCode(diags, hostReachName)
	if len(reported) == 0 {
		t.Fatal("the analyzer reported nothing, so this requirement cannot be measured")
	}

	for _, d := range reported {
		if !strings.HasSuffix(d.Path, ".go") || strings.Contains(d.Path, ".flow") {
			t.Errorf("a finding about consumer Go is attributed to %q", d.Path)
		}
		if !d.Foreign {
			t.Errorf("a finding about a file the run did not parse is not marked foreign: %+v", d)
		}
		if d.Pos.Line == 0 || d.Pos.Col == 0 {
			t.Errorf("a finding carries no position: %s at %s", d.Path, d.Pos)
		}
		if d.End.Offset <= d.Pos.Offset {
			t.Errorf("a finding spans nothing: %s..%s", d.Pos, d.End)
		}
		if d.Severity != SeverityError {
			t.Errorf("a finding is reported at %s; the assembler refuses on error and this violation "+
				"must stop generation", d.Severity)
		}
		if d.Code != hostReachName {
			t.Errorf("a finding carries code %q, want %q", d.Code, hostReachName)
		}
		// The constructor call is a SECOND position in a possibly different file,
		// which one diagnostic cannot carry structurally, so it is named in the
		// message — the file this finding is ABOUT is not.
		if !strings.Contains(d.Message, "passed to ") {
			t.Errorf("a finding does not name the constructor call that made the function a node function: %s",
				d.Message)
		}
		if strings.Contains(d.Message, d.Path+":"+d.Pos.String()) {
			t.Errorf("a finding composes its OWN file and line into its message instead of carrying them: %s",
				d.Message)
		}
	}
}

// TestANodeFunctionTheWalkCannotOpenIsRecordedRatherThanCalledClean is the
// disclosure, measured.
//
// The Doc says the walk does not see a node function reached through a variable,
// a call or a struct field. That sentence is prose, and prose about a silence is
// exactly what a consumer over-reads: the result therefore RECORDS each one, so
// "clean" and "not read" are different answers rather than the same empty set.
func TestANodeFunctionTheWalkCannotOpenIsRecordedRatherThanCalledClean(t *testing.T) {
	reaches, _ := reachesOf(t)

	if len(reaches.Unread) < 2 {
		t.Errorf("the walk recorded %d unreadable node functions, and the fixture wires two — one held in "+
			"a variable and one returned by a call: %+v", len(reaches.Unread), reaches.Unread)
	}
	for _, unread := range reaches.Unread {
		if unread.Pos.Line == 0 || unread.Path == "" {
			t.Errorf("an unreadable node function is recorded without a position: %+v", unread)
		}
	}

	// AND IT IS NOT REPORTED. A body the walk never opened is not a violation, and
	// reporting one would be a finding about a function nobody read.
	for _, site := range reaches.Sites {
		if site.Func == "" {
			t.Errorf("a reach was reported for a node function with no resolved body: %+v", site)
		}
	}
	t.Logf("%d node functions were identified and not readable: %+v", len(reaches.Unread), reaches.Unread)
}

// TestEveryRootPackageIsWalkedRatherThanOnlyTheFirst pins the scope decision.
//
// THE CORPUS IS EVERY PACKAGE THE PATTERNS NAMED, and the tempting narrowing is
// to skip the package being generated — which is wrong, because a consumer
// routinely generates into the package their own wiring lives in, and skipping it
// would skip the hand-written code this check exists to read. The fixture module
// holds two packages and the violation in the SECOND one must be reported.
func TestEveryRootPackageIsWalkedRatherThanOnlyTheFirst(t *testing.T) {
	reaches, _ := reachesOf(t)

	if !slices.Contains(reaches.Packages, hostReachSecond) {
		t.Fatalf("the walk read %v, and the fixture's second package %s is not among them",
			reaches.Packages, hostReachSecond)
	}

	var found bool
	for _, site := range reaches.Sites {
		if strings.Contains(site.Path, "generated") {
			found = true
		}
	}
	if !found {
		t.Errorf("the violation in the second package was not reported; the walk reported %d sites",
			len(reaches.Sites))
	}
}

// TestTheHostReachAnalyzerRefusesANilPackageSet pins the refusal on both
// boundaries.
//
// THE WRAP IS THE POINT. errors.Is must answer the same directly and through the
// driver, which is where the analysis is actually consumed; joining the name onto
// the error's text instead would read identically and end the chain.
func TestTheHostReachAnalyzerRefusesANilPackageSet(t *testing.T) {
	a := HostReachAnalyzer(nil)

	_, direct := a.Run(&Pass{Analyzer: a})
	if !errors.Is(direct, errNoPackages) {
		t.Errorf("called directly, the analyzer refused with %v, want the shared no-packages sentinel", direct)
	}

	_, through := Run(nil, []*Analyzer{a})
	if !errors.Is(through, errNoPackages) {
		t.Errorf("through the driver, the analyzer refused with %v, want the shared no-packages sentinel", through)
	}
	t.Logf("refused directly: %v; through the driver: %v", direct, through)
}

// TestTheHostReachAnalyzerStaysOutOfTheRegisteredSet pins that it is constructed
// rather than registered, the same way the two analyzers before it are.
//
// IT ASSERTS ABSENCE BY NAME RATHER THAN A COUNT. The registered set's size moves
// whenever any analyzer lands; this analyzer's absence from it does not.
func TestTheHostReachAnalyzerStaysOutOfTheRegisteredSet(t *testing.T) {
	for _, a := range All() {
		if a.Name == hostReachName {
			t.Fatalf("%s is registered; a package-set analyzer that joined All() would refuse every run "+
				"the linter and the language server make, neither of which loads packages", hostReachName)
		}
	}
	t.Logf("All() reports %d analyzers and none of them %s", len(All()), hostReachName)
}
