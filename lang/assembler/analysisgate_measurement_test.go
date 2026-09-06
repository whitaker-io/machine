// Package assembler - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package assembler

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/whitaker-io/machine/lang/analysis"
	"github.com/whitaker-io/machine/lang/ast"
	"github.com/whitaker-io/machine/lang/loader"
)

// legalAndDeliberate is the phrase a finding uses to say, in its own text, that
// the condition it reports is permitted. partition's doc counts them.
const legalAndDeliberate = "legal and may be deliberate"

// measuredNumerals is the spelled-out vocabulary partition's doc can state a
// measurement in.
var measuredNumerals = map[string]int{
	"zero": 0, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6,
	"seven": 7, "eight": 8, "nine": 9, "ten": 10, "eleven": 11, "twelve": 12,
	"thirteen": 13, "fourteen": 14, "fifteen": 15, "sixteen": 16, "seventeen": 17,
	"eighteen": 18, "nineteen": 19, "twenty": 20,
}

// numeralAlternation is the vocabulary as a regex group. The counts are matched
// against THIS rather than against a bare word, because "the gate's findings"
// also reads as a word standing before the phrase and would be taken for a
// number that could not be read.
var numeralAlternation = `(zero|one|two|three|four|five|six|seven|eight|nine|ten|eleven|twelve|thirteen|` +
	`fourteen|fifteen|sixteen|seventeen|eighteen|nineteen|twenty)`

var (
	findingsCountRe = regexp.MustCompile(`(?i)\b` + numeralAlternation + ` findings\b`)
	ofWhichCountRe  = regexp.MustCompile(`(?i)\b` + numeralAlternation + ` of which\b`)
)

// TestPartitionsMeasurementMatchesTheEndToEndFixture makes partition's doc
// DERIVE FROM THE FIXTURE instead of remembering a number.
//
// WHY IT EXISTS. partition's doc justifies a fixed refusal line with a
// measurement over this repository's own end-to-end fixture — how many findings
// the gate reports and how many of them say in their own text that the condition
// is legal. A measurement in prose does not move when the corpus or the registry
// moves, and this one did not: it stated eleven findings while the fixture
// produced seven, and stayed green through the analyzer change that edited the
// sentence beside it.
//
// THE INSTRUMENT IS THE ONE THE SENTENCE IS ABOUT: the module's own e2e fixture,
// staged the way the e2e tests stage it, run through analysis.Gate — which is
// the whole constructed-inclusive walk, not the registered subset.
func TestPartitionsMeasurementMatchesTheEndToEndFixture(t *testing.T) {
	dir := t.TempDir()
	stageModule(t, dir, "flowe2e", map[string]string{
		"feed.go":       readFixtureFile(t, filepath.Join("testdata", "e2e", "feed.go.txt")),
		"pipeline.flow": readFixtureFile(t, filepath.Join("testdata", "e2e", "pipeline.flow")),
		"main.go":       e2eMain,
	})

	pkgs, err := loader.Load(dir, []string{"./..."})
	if err != nil {
		t.Fatalf("the end-to-end fixture module did not load: %v", err)
	}

	flowPath := filepath.Join(dir, "pipeline.flow")
	body, err := os.ReadFile(flowPath) //nolint:gosec // a test reading the fixture it just staged
	if err != nil {
		t.Fatalf("cannot read %s: %v", flowPath, err)
	}
	file, err := ast.Parse(body)
	if err != nil {
		t.Fatalf("the end-to-end fixture does not parse: %v", err)
	}

	result, err := analysis.Gate([]analysis.Source{{Path: flowPath, Src: body, File: file}}, pkgs, "flowe2e")
	if err != nil {
		t.Fatalf("the gate refused the end-to-end fixture: %v", err)
	}

	total := len(result.Diagnostics)
	legal := 0
	for _, d := range result.Diagnostics {
		if strings.Contains(d.Message, legalAndDeliberate) {
			legal++
		}
	}

	// THE CONTROL. A run that produced nothing, or one whose phrase match never
	// fired, would satisfy any doc that happened to spell a zero.
	if total == 0 || legal == 0 {
		t.Fatalf("CONTROL FAILED: the gate reported %d findings, %d of them saying %q; a measurement over "+
			"nothing cannot gate a sentence", total, legal, legalAndDeliberate)
	}

	doc := docCommentAbove(t, "analysisgate.go", "func partition(")
	gotTotal := numeralBefore(t, doc, findingsCountRe, "findings")
	gotLegal := numeralBefore(t, doc, ofWhichCountRe, "of which")

	if gotTotal != total || gotLegal != legal {
		t.Errorf("partition's doc says the fixture produces %d findings, %d of which say %q; the fixture "+
			"produces %d and %d", gotTotal, gotLegal, legalAndDeliberate, total, legal)
		for _, d := range result.Diagnostics {
			t.Logf("  measured: %s", d.Message)
		}
	}
	t.Logf("analysis.Gate over the end-to-end fixture: %d findings, %d saying %q; partition's doc says %d and %d",
		total, legal, legalAndDeliberate, gotTotal, gotLegal)
}

// docCommentAbove returns the contiguous // block immediately above a
// declaration, in source order.
func docCommentAbove(t *testing.T, file, decl string) string {
	t.Helper()

	raw, err := os.ReadFile(file) //nolint:gosec // a test reading its own package source
	if err != nil {
		t.Fatalf("reading %s: %v", file, err)
	}
	lines := strings.Split(string(raw), "\n")
	at := -1
	for i, line := range lines {
		if strings.HasPrefix(line, decl) {
			at = i

			break
		}
	}
	if at < 0 {
		t.Fatalf("%s declares no %q; this gate names a declaration that does not exist", file, decl)
	}

	first := at
	for first > 0 && strings.HasPrefix(lines[first-1], "//") {
		first--
	}
	if first == at {
		t.Fatalf("CONTROL FAILED: %s carries no doc comment above %q, so this gate has nothing to read",
			file, decl)
	}

	return strings.Join(lines[first:at], "\n")
}

// numeralBefore reads the spelled-out number standing immediately before a
// phrase in a doc comment.
func numeralBefore(t *testing.T, doc string, re *regexp.Regexp, phrase string) int {
	t.Helper()

	m := re.FindStringSubmatch(doc)
	if m == nil {
		t.Fatalf("CONTROL FAILED: the doc states no count before %q, so this gate cannot discriminate:\n%s",
			phrase, doc)
	}
	n, known := measuredNumerals[strings.ToLower(m[1])]
	if !known {
		t.Fatalf("the doc says %q before %q, which is not a number this gate can read", m[1], phrase)
	}

	return n
}
