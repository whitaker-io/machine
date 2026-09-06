// Package analysis - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package analysis

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

// lintDocPath is lang/lint's package doc, read ACROSS THE MODULE BOUNDARY the way
// astTestdata is. It is the shipped sentence that tells a consumer which
// analyzers bound what a clean run proves.
const lintDocPath = "../lint/doc.go"

// notALimit records the registered analyzers this module mandates a disclosure
// for whose disclosure is NOT a bound on what a clean run proves, and is
// therefore correctly absent from lang/lint's census.
//
// ONE ENTRY, AND THE REASON IS THE ENTRY. flowgraph discloses that its send model
// is one of two defensible readings — a modeling CHOICE. A reader comparing the
// two lists finds one name in this module's map and not in lang/lint's, and
// without this record the only two available conclusions are that lang/lint is
// stale or that this map is wrong.
var notALimit = map[string]string{
	"flowgraph": "its disclosure is a modeling choice — one of two defensible send readings — " +
		"rather than a bound on what a clean run proves",
}

// TestMandatedDisclosuresAreAccountedForInTheShippedCensus couples this module's
// disclosure gate to the sentence lang/lint ships about it.
//
// WHY IT EXISTS. A disclosure with no row in the map beside it is ungated prose;
// that half is enforced next door. The other half is that lang/lint's package doc
// enumerates the analyzers whose limits bound a clean run, and NOTHING held the
// two together — an analyzer landed with a mandated disclosure, the enumeration
// was not extended, and every suite in every module stayed green. This test is
// the join: a registered analyzer this module mandates a disclosure for is either
// NAMED in that enumeration or recorded above as not a limit, so the next
// analyzer cannot land outside both lists.
//
// IT IS DELIBERATELY ONE-DIRECTIONAL. It asserts nothing about an analyzer
// lang/lint names that has no row here — signature is exactly that, its limit
// stated in its own Doc without a mandated phrase — because this module's map is
// not the authority on lang/lint's census, only on its own gate.
func TestMandatedDisclosuresAreAccountedForInTheShippedCensus(t *testing.T) {
	raw, err := os.ReadFile(lintDocPath) //nolint:gosec // a test reading a sibling module's shipped doc
	if err != nil {
		t.Fatalf("reading %s: %v", lintDocPath, err)
	}
	doc := string(raw)

	// THE CONTROL. A moved or renamed doc would otherwise let every membership
	// check below pass over an empty string.
	if !strings.Contains(doc, "IT DOES NOT CLAIM MORE THAN THE ANALYZERS DO.") {
		t.Fatalf("CONTROL FAILED: %s no longer carries the limit census this test reads", lintDocPath)
	}
	_, census, cut := strings.Cut(doc, "bounded by every\n// one of them:")
	if !cut {
		t.Fatalf("CONTROL FAILED: %s carries no enumeration clause after its census sentence", lintDocPath)
	}
	census, _, _ = strings.Cut(census, "\n//\n")

	registered := map[string]bool{}
	for _, a := range All() {
		registered[a.Name] = true
	}
	if len(registered) == 0 {
		t.Fatal("CONTROL FAILED: the registry is empty, so this coupling would hold vacuously")
	}

	var accounted, excused int
	for _, name := range sortedKeys(requiredDisclosures) {
		if !registered[name] {
			// A constructed analyzer ships no rule in the batch tool's registry, so
			// lang/lint's census does not speak about it.
			continue
		}
		if reason, isExcused := notALimit[name]; isExcused {
			excused++
			t.Logf("%s is excused from the shipped census: %s", name, reason)

			continue
		}
		if !regexp.MustCompile(`\b` + regexp.QuoteMeta(name) + `\b`).MatchString(census) {
			t.Errorf("%s carries a mandated disclosure and is neither named in %s's limit census nor recorded "+
				"in notALimit with a reason; a disclosure outside both lists is one no consumer is told about",
				name, lintDocPath)

			continue
		}
		accounted++
	}

	if accounted == 0 {
		t.Fatal("CONTROL FAILED: no mandated disclosure was matched in the shipped census, so this coupling " +
			"is not reaching the population it exists to gate")
	}
	// AN EXCUSE FOR AN ANALYZER THAT MANDATES NOTHING is a stale record, and a
	// stale record is what this whole test exists to stop accumulating.
	for name := range notALimit {
		if _, mandated := requiredDisclosures[name]; !mandated {
			t.Errorf("notALimit excuses %s, which mandates no disclosure here", name)
		}
	}
	t.Logf("%d mandated disclosures named in %s's census, %d excused as modeling choices",
		accounted, lintDocPath, excused)
}
