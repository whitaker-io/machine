// Package analysis - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package analysis

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// langRoot is the four sibling modules that ship prose about this registry,
// reached across the module boundary the way astTestdata is.
const langRoot = ".."

// spelledNumbers is this module's spelled-out numeral vocabulary, cardinal and
// ordinal. It is the whole small range rather than the census window, because
// the disclosure gate next door reads a count out of its own doc comment and
// that count is smaller than anything the registry census looks at.
var spelledNumbers = map[string]int{
	"one": 1, "first": 1,
	"two": 2, "second": 2,
	"three": 3, "third": 3,
	"four": 4, "fourth": 4,
	"five": 5, "fifth": 5,
	"six": 6, "sixth": 6,
	"seven": 7, "seventh": 7,
	"eight": 8, "eighth": 8,
	"nine": 9, "ninth": 9,
	"ten": 10, "tenth": 10,
	"eleven": 11, "eleventh": 11,
	"twelve": 12, "twelfth": 12,
	"thirteen": 13, "thirteenth": 13,
	"fourteen": 14, "fourteenth": 14,
	"fifteen": 15, "fifteenth": 15,
	"sixteen": 16, "sixteenth": 16,
	"seventeen": 17, "seventeenth": 17,
	"eighteen": 18, "eighteenth": 18,
	"nineteen": 19, "nineteenth": 19,
	"twenty": 20, "twentieth": 20,
}

// countNumerals is the WINDOW a claim about the registry can plausibly reach:
// the current size, the constructed-inclusive size, and enough room either side
// that a stale claim still lands inside the window rather than falling out of it
// and passing in silence. Numerals below it are ordinary English beside the word
// analyzer — "six of the thirteen" is a claim about a subset — and pinning them
// would red on prose that is not about the registry's size at all.
var countNumerals = windowed(spelledNumbers, 10, 20)

// windowed is the census window as a derivation rather than a second table.
func windowed(all map[string]int, low, high int) map[string]int {
	out := map[string]int{}
	for word, n := range all {
		if n >= low && n <= high {
			out[word] = n
		}
	}

	return out
}

var countNumeralRe = regexp.MustCompile(
	`(?i)\b(ten|tenth|eleven|eleventh|twelve|twelfth|thirteen|thirteenth|fourteen|fourteenth|` +
		`fifteen|fifteenth|sixteen|sixteenth|seventeen|seventeenth|eighteen|eighteenth|` +
		`nineteen|nineteenth|twenty|twentieth)\b`)

// TestCountProseAcrossLangAgreesWithTheRegistry is the EXECUTED census of the
// shipped sentences that state how many analyzers there are.
//
// WHY IT EXISTS. The roster gate beside it pins the registered set by name, and
// its doc comment enumerates the prose sites that have to move with it — but an
// enumeration in a comment is the same kind of artifact as the sites it lists,
// and it went stale the same way: a registry change moved twelve of the sites
// and missed a thirteenth in lang/lsp, whose ordinal was already one behind
// before that change and two behind after it. A sweep found it in one command,
// so the sweep is a gate now rather than a thing a reviewer has to think of.
//
// THE RULE IT ENFORCES IS THE ONE THAT IS DECIDABLE: a COMMENT line under lang/
// that names an analyzer and spells a numeral in this census's window must spell
// a number the registry currently justifies — either the registered count, or
// that count plus the two analyzers Gate constructs rather than registers.
//
// TWO BOUNDS, both deliberate and both measured rather than assumed. (1) The
// GRANULARITY IS THE LINE: a claim that spells its numeral on a different line
// from the word "analyzer" is invisible here. Widening to the comment block
// swept in prose about switch cases and fixture counts that has nothing to do
// with the registry, so the narrower rule is the one that discriminates.
// (2) NUMERALS OUTSIDE THE WINDOW ARE NOT READ: "six of the thirteen" is a claim
// about a subset, not about the registry's size, and pinning every small numeral
// beside the word analyzer would red on ordinary English.
func TestCountProseAcrossLangAgreesWithTheRegistry(t *testing.T) {
	registered := len(All())
	// Gate runs the registered set plus the two analyzers it constructs, and
	// several shipped sentences state THAT number instead. Both are true claims,
	// so both are allowed — and the second is DERIVED from gate.go's own require
	// list rather than declared here, which is the mistake this census exists to
	// catch. The constructors are handed a nil package set because only pointer
	// IDENTITY is read out of them.
	gateWalks := map[*Analyzer]bool{}
	for _, a := range gateRequires(TypeInferenceAnalyzer(nil, ""), SerializationAnalyzer(nil, ""),
		HostReachAnalyzer(nil)) {
		gateWalks[a] = true
	}
	allowed := map[int]bool{registered: true, len(gateWalks): true}

	var files, commentLines, agreed int
	var wrong []string

	err := filepath.WalkDir(langRoot, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() || filepath.Ext(path) != ".go" {
			return err
		}
		files++

		body, readErr := os.ReadFile(path) //nolint:gosec // a test reading its own tree
		if readErr != nil {
			t.Fatalf("cannot read %s: %v", path, readErr)
		}
		for i, line := range strings.Split(string(body), "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), "//") {
				continue
			}
			commentLines++
			if !strings.Contains(strings.ToLower(line), "analyz") {
				continue
			}
			for _, m := range countNumeralRe.FindAllStringSubmatch(line, -1) {
				n := countNumerals[strings.ToLower(m[1])]
				if allowed[n] {
					agreed++

					continue
				}
				wrong = append(wrong, path+":"+itoa(i+1)+": says "+m[1]+" ("+itoa(n)+"): "+strings.TrimSpace(line))
			}
		}

		return nil
	})
	if err != nil {
		t.Fatalf("walking %s: %v", langRoot, err)
	}

	// THE CONTROLS. A census whose walk found nothing, or whose regex matched
	// nothing, reports a clean tree and reads exactly like a clean tree.
	if files < 50 || commentLines < 1000 {
		t.Fatalf("CONTROL FAILED: the sweep read %d .go files and %d comment lines under %s; a silence over "+
			"that little is not evidence", files, commentLines, langRoot)
	}
	if agreed < 8 {
		t.Fatalf("CONTROL FAILED: the sweep accepted only %d agreeing count claims, so it is not reaching the "+
			"population it exists to gate", agreed)
	}

	for _, site := range wrong {
		t.Errorf("a shipped sentence states a count the registry does not justify (%d registered, %d walked by "+
			"the gate): %s", registered, len(gateWalks), site)
	}
	t.Logf("swept %d .go files and %d comment lines under %s: %d registered, %d walked by the gate; "+
		"%d count claims agree, %d do not",
		files, commentLines, langRoot, registered, len(gateWalks), agreed, len(wrong))
}

// itoa keeps the census's message building free of a strconv import whose only
// use here is a line number.
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for ; n > 0; n /= 10 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
	}

	return string(digits)
}

// docNumeralsAbove returns the spelled-out numbers in the contiguous comment
// block immediately above a declaration, in source order.
//
// It exists because a gate's own doc comment states how much the gate covers,
// and that sentence is prose beside a table: the table grows, the numeral does
// not, and nothing reds. This is the same mechanism lang/ast uses to keep its
// reserved-spelling counts honest.
func docNumeralsAbove(t *testing.T, file, decl string) []int {
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

	var out []int
	for _, line := range lines[first:at] {
		for _, m := range spelledNumberRe.FindAllStringSubmatch(line, -1) {
			out = append(out, spelledNumbers[strings.ToLower(m[1])])
		}
	}

	return out
}

var spelledNumberRe = regexp.MustCompile(
	`(?i)\b(one|first|two|second|three|third|four|fourth|five|fifth|six|sixth|seven|seventh|eight|eighth|` +
		`nine|ninth|ten|tenth|eleven|eleventh|twelve|twelfth|thirteen|thirteenth|fourteen|fourteenth|` +
		`fifteen|fifteenth|sixteen|sixteenth|seventeen|seventeenth|eighteen|eighteenth|nineteen|nineteenth|` +
		`twenty|twentieth)\b`)
