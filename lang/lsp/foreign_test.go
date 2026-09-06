// Package lsp - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package lsp

import (
	"path/filepath"
	"testing"

	"github.com/whitaker-io/machine/lang/analysis"
	"github.com/whitaker-io/machine/lang/ast"
	"go.lsp.dev/uri"
)

// fileURI is a document URI under the test's own temporary directory, the way
// every other test in this package builds one.
func fileURI(t *testing.T, name string) uri.URI {
	t.Helper()

	return uri.File(filepath.Join(t.TempDir(), name))
}

// consumerGo is the path of a hand-written Go file no editor in these tests has
// open, and no .flow document could ever be.
const consumerGo = "/work/orders/cmd/wire.go"

// goFinding is a diagnostic of the shape an analysis that reads consumer Go
// reports: a real Go file at a real position, marked as a file the run did not
// parse.
func goFinding() analysis.Diagnostic {
	return analysis.Diagnostic{
		Pos:      ast.Position{Offset: 412, Line: 24, Col: 14},
		End:      ast.Position{Offset: 418, Line: 24, Col: 20},
		Message:  "the node function WireOrders.func1, passed to Map at wire.go:22:19, reaches the accessor",
		Severity: analysis.SeverityError,
		Code:     "hostreach",
		Path:     consumerGo,
		Foreign:  true,
	}
}

// TestAConsumerGoFindingIsNeverPositionedThroughAFlowDocumentsMapper is the arm
// the widening exists to keep honest.
//
// THE HAZARD IS REAL AND IS DEMONSTRATED IN THE SAME RUN. A Mapper indexes ITS
// OWN document's bytes and answers the zero Position for a line that document
// does not have, so a Go-file position mapped through a .flow buffer lands at
// 0:0 — a squiggle on the first character of the wrong file, with no error
// anywhere. What keeps it from happening is that diagnostics are bucketed BY
// PATH before any mapper is consulted, and this test asserts the bucketing rather
// than trusting it.
func TestAConsumerGoFindingIsNeverPositionedThroughAFlowDocumentsMapper(t *testing.T) {
	store := NewStore()
	store.Open(fileURI(t, "alpha.flow"), []byte(flowWithAFinding))
	docs := store.Documents()
	if len(docs) != 1 {
		t.Fatalf("CONTROL FAILED: the store holds %d documents, want 1", len(docs))
	}
	doc := docs[0]

	mixed := []analysis.Diagnostic{
		{Path: doc.Path, Pos: ast.Position{Offset: 0, Line: 1, Col: 1},
			End: ast.Position{Offset: 4, Line: 1, Col: 5}, Message: "about the flow"},
		goFinding(),
	}

	byPath := groupByPath(mixed)
	for _, d := range byPath[doc.Path] {
		if d.Foreign {
			t.Errorf("a finding about %s was bucketed under the open document %s, so it would be "+
				"positioned through that document's mapper", d.Path, doc.Path)
		}
	}
	if len(byPath[consumerGo]) != 1 {
		t.Errorf("the consumer-Go finding is not in its own bucket: %v", byPath)
	}

	// THE DEMONSTRATION. Mapping it through this document's mapper anyway is what
	// the bucketing prevents, and it is silent rather than an error — which is why
	// the assertion above is on the routing and not on the mapper.
	hazard := convert(doc, []analysis.Diagnostic{goFinding()})
	if len(hazard) != 1 {
		t.Fatalf("the conversion produced %d diagnostics, want 1", len(hazard))
	}
	if hazard[0].Range.Start.Line != 0 || hazard[0].Range.Start.Character != 0 {
		t.Logf("this document does have line 24, so the hazard is not visible in this fixture: %v",
			hazard[0].Range)
	} else {
		t.Logf("confirmed: mapped through the wrong document a Go position becomes %v", hazard[0].Range)
	}
}

// TestARefreshPublishesOnlyForTheDocumentsTheEditorHasOpen records the decision
// the widening forces, and its cost.
//
// A refresh publishes one set PER OPEN DOCUMENT, keyed by path. A finding about a
// file the editor does not have open therefore reaches no publish — it is not
// dropped from the run, it stays in the snapshot, but nothing shows it to a user.
// THE DECISION IS TO LEAVE THAT AS IT IS, and the reason is that publishing it
// would mean inventing a range in a document this server never read: the Mapper
// is per-document and indexes bytes the server holds. In production this server
// cannot produce such a finding at all — it runs analysis.All(), and every
// analysis that reads consumer Go is constructed rather than registered — so the
// case is unreachable rather than merely unhandled, and this test is what says so.
func TestARefreshPublishesOnlyForTheDocumentsTheEditorHasOpen(t *testing.T) {
	for _, a := range analysis.All() {
		if a.Name == "hostreach" {
			t.Fatalf("%s is registered now, so this server CAN produce a finding about a file no editor "+
				"has open, and the unreachable case above has become a silent drop", a.Name)
		}
	}

	store := NewStore()
	store.Open(fileURI(t, "alpha.flow"), []byte(flowWithAFinding))
	docs := store.Documents()

	byPath := groupByPath([]analysis.Diagnostic{goFinding()})
	published := 0
	for _, doc := range docs {
		published += len(byPath[doc.Path])
	}
	if published != 0 {
		t.Errorf("%d findings about a file no editor has open would be published against an open "+
			"document", published)
	}
	t.Logf("%d open documents, %d buckets, none of them publishable: %v", len(docs), len(byPath), sortedPaths(byPath))
}

// TestASuppressionKeyedOnADamagedParseLetsAConsumerGoFindingThrough pins the one
// consumer whose key is a path AND whose meaning is about parsing.
//
// attributable drops analyzer findings ABOUT a document that failed to parse,
// keyed on Path. A consumer-Go file is never in that map — it is not a document
// this server parses at all — so the finding must pass through rather than be
// dropped by a lookup that cannot apply to it.
func TestASuppressionKeyedOnADamagedParseLetsAConsumerGoFindingThrough(t *testing.T) {
	damaged := map[string]bool{"/work/orders/broken.flow": true}
	mixed := []analysis.Diagnostic{
		{Path: "/work/orders/broken.flow", Message: "about the damaged flow"},
		goFinding(),
	}

	kept := attributable(mixed, damaged)
	if len(kept) != 1 {
		t.Fatalf("attribution kept %d of 2 findings: %v", len(kept), kept)
	}
	if !kept[0].Foreign || kept[0].Path != consumerGo {
		t.Errorf("attribution kept the wrong finding: %+v", kept[0])
	}
}

// sortedPaths lists a bucketing's keys, for a failure message.
func sortedPaths(byPath map[string][]analysis.Diagnostic) []string {
	out := make([]string, 0, len(byPath))
	for path := range byPath {
		out = append(out, path)
	}

	return out
}
