// Package lint - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package lint

import (
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"github.com/whitaker-io/machine/lang/analysis"
)

// limitCensusOpening is where this package's doc states how many of the
// registered analyzers bound what a clean run proves.
const limitCensusOpening = "IT DOES NOT CLAIM MORE THAN THE ANALYZERS DO."

// limitStatements is the AUTHORITATIVE list behind that census: every registered
// analyzer whose own Doc bounds what a clean result proves, mapped to the phrase
// in ITS Doc that does the bounding.
//
// THE PHRASE IS THE POINT, not the name. A census keyed on names alone stays
// green when an analyzer's Doc quietly drops the limit the census credits it
// with, which is the same failure as a numeral that does not move: prose stating
// something no test reads. Each phrase below is asserted present in that
// analyzer's live Doc, so this list cannot outlive what it describes.
//
// FLOWGRAPH IS ABSENT ON PURPOSE. Its Doc carries a required disclosure — that
// its send model is one of two defensible readings — but that is a modeling
// CHOICE rather than a bound on what a clean run proves, so it is not a member
// here. The analysis module's disclosure gate is what keeps the two lists from
// silently diverging.
var limitStatements = map[string]string{
	"typeflow":     "is not type checking",
	"state":        "denylist of two retired spellings",
	"switches":     "cannot prove coverage",
	"resolve":      "ships no unimported-qualifier check",
	"signature":    "resolves to no flow in the run is not reported",
	"errorrouting": "not the enforcement",
	"hostaccess":   ".flow-resident func bodies only",
}

// censusNumerals is the spelled-out vocabulary the census sentence can state its
// two counts in.
var censusNumerals = map[string]int{
	"zero": 0, "one": 1, "two": 2, "three": 3, "four": 4, "five": 5, "six": 6,
	"seven": 7, "eight": 8, "nine": 9, "ten": 10, "eleven": 11, "twelve": 12,
	"thirteen": 13, "fourteen": 14, "fifteen": 15, "sixteen": 16, "seventeen": 17,
	"eighteen": 18, "nineteen": 19, "twenty": 20,
}

var censusRe = regexp.MustCompile(`(?i)\b(\w+) of the (\w+) registered\b`)

// TestPackageDocLimitCensusIsTrue gates the two numerals and the enumeration in
// this package's own doc comment.
//
// WHY IT EXISTS. The sentence claims a clean run is bounded by EVERY limit the
// analyzers state, and then enumerates them. Both halves are prose a compiler
// cannot check, and both went stale together: an analyzer landed carrying the
// largest bound of the set — that its silence is not a proof — and the change
// that added it moved the registry numeral beside the enumeration without moving
// the enumeration.
//
// WHAT IT CANNOT DO, stated rather than implied: it cannot decide that a NEW
// analyzer states a limit. What it does instead is force the question — the
// registry numeral is gated against the live registry, so an analyzer landing
// reds this test until someone reads its Doc and rules, and the analysis module's
// disclosure gate refuses the other escape route.
func TestPackageDocLimitCensusIsTrue(t *testing.T) {
	doc := packageDocParagraph(t, limitCensusOpening)
	registered := map[string]*analysis.Analyzer{}
	for _, a := range analysis.All() {
		registered[a.Name] = a
	}
	if len(registered) == 0 {
		t.Fatal("CONTROL FAILED: the registry is empty, so this census would pass vacuously")
	}

	m := censusRe.FindStringSubmatch(doc)
	if m == nil {
		t.Fatalf("CONTROL FAILED: the census sentence states no counts, so this gate cannot discriminate:\n%s", doc)
	}
	stated, statedOK := censusNumerals[strings.ToLower(m[1])]
	total, totalOK := censusNumerals[strings.ToLower(m[2])]
	if !statedOK || !totalOK {
		t.Fatalf("the census says %q of the %q registered, and this gate can read neither", m[1], m[2])
	}

	if total != len(registered) {
		t.Errorf("the package doc says %d analyzers are registered; the registry holds %d", total, len(registered))
	}
	if stated != len(limitStatements) {
		t.Errorf("the package doc says %d of them state a limit; this census lists %d: %v",
			stated, len(limitStatements), sortedNames(limitStatements))
	}

	// THE ENUMERATION IS THE CLAUSE AFTER THE COLON, so the census's own prose
	// about analyzers stating limits cannot be read as naming one.
	_, enumeration, cut := strings.Cut(doc, "bounded by every\n// one of them:")
	if !cut {
		t.Fatalf("CONTROL FAILED: the census sentence carries no enumeration clause:\n%s", doc)
	}

	for _, name := range sortedNames(limitStatements) {
		a, isRegistered := registered[name]
		if !isRegistered {
			t.Errorf("the census credits %s with a limit, but it is not registered", name)

			continue
		}
		if !strings.Contains(strings.ToLower(collapse(a.Doc)), limitStatements[name]) {
			t.Errorf("%s's own Doc no longer carries the limit this census credits it with (%q)",
				name, limitStatements[name])
		}
		if !regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`).MatchString(enumeration) {
			t.Errorf("%s states a limit and the package doc's enumeration does not name it", name)
		}
	}

	// THE OTHER DIRECTION: an analyzer named in the enumeration that this census
	// does not credit with a limit is the same defect read backwards.
	for name := range registered {
		if _, listed := limitStatements[name]; listed {
			continue
		}
		if regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`).MatchString(enumeration) {
			t.Errorf("the package doc's enumeration names %s, which this census does not list as stating a limit",
				name)
		}
	}
	t.Logf("package doc census: %d of %d registered analyzers state a limit; enumeration names %v",
		stated, total, sortedNames(limitStatements))
}

// packageDocParagraph returns the doc.go comment paragraph opening with a phrase.
func packageDocParagraph(t *testing.T, opening string) string {
	t.Helper()

	raw, err := os.ReadFile("doc.go") //nolint:gosec // a test reading its own package doc
	if err != nil {
		t.Fatalf("reading doc.go: %v", err)
	}
	lines := strings.Split(string(raw), "\n")
	at := -1
	for i, line := range lines {
		if strings.Contains(line, opening) {
			at = i

			break
		}
	}
	if at < 0 {
		t.Fatalf("doc.go carries no paragraph opening with %q; this gate names prose that does not exist", opening)
	}

	end := at
	for end < len(lines) && strings.HasPrefix(lines[end], "//") && strings.TrimSpace(lines[end]) != "//" {
		end++
	}

	return strings.Join(lines[at:end], "\n")
}

// collapse folds a Doc's line breaks so a phrase spanning a wrap still matches.
func collapse(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// sortedNames is a census map's keys in a stable order.
func sortedNames(m map[string]string) []string {
	out := make([]string, 0, len(m))
	for name := range m {
		out = append(out, name)
	}
	sort.Strings(out)

	return out
}
