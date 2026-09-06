// Package lint - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package lint

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/whitaker-io/machine/lang/analysis"
	"github.com/whitaker-io/machine/lang/ast"
)

// TestCommentsChangeNoDiagnostic is the linter's leg of the comment form: a
// source and the same source with every comment blanked out lint to the same
// diagnostics, position for position.
//
// THE CONTROL IS BLANKING, NOT DELETING. Each comment span is overwritten with
// spaces of the same length, so every byte offset, line and column the two
// sources share stays shared and a diagnostic that moved would be visible as a
// moved position rather than hidden by a shifted line. The fixture is required
// to carry comments and the two sources are required to differ, so an empty
// comparison cannot pass by accident.
func TestCommentsChangeNoDiagnostic(t *testing.T) {
	fixture := filepath.Join(astTestdata, "valid", "comments.flow")
	src, err := os.ReadFile(fixture)
	if err != nil {
		t.Fatalf("read %s: %v", fixture, err)
	}
	file, err := ast.Parse(src)
	if err != nil {
		t.Fatalf("parse %s: %v", fixture, err)
	}
	if len(file.Comments) == 0 {
		t.Fatalf("CONTROL FAILED: %s carries no comments, so an equal diagnostic set proves nothing", fixture)
	}

	blanked := append([]byte(nil), src...)
	for _, c := range file.Comments {
		for i := c.Start.Offset; i < c.Stop.Offset; i++ {
			blanked[i] = ' '
		}
	}
	if bytes.Equal(blanked, src) {
		t.Fatal("CONTROL FAILED: blanking the comments changed nothing")
	}

	dir := t.TempDir()
	withComments := filepath.Join(dir, "with", "comments.flow")
	withoutComments := filepath.Join(dir, "without", "comments.flow")
	for path, body := range map[string][]byte{withComments: src, withoutComments: blanked} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, body, 0o644); err != nil {
			t.Fatal(err)
		}
	}

	render := func(path string) []string {
		batch, err := Load([]string{path})
		if err != nil {
			t.Fatalf("load %s: %v", path, err)
		}
		result, err := Check(batch, analysis.SeverityHint)
		if err != nil {
			t.Fatalf("check %s: %v", path, err)
		}
		out := make([]string, 0, len(result.Diagnostics))
		for _, d := range result.Diagnostics {
			out = append(out, fmt.Sprintf("%d:%d-%d:%d %s %s %s",
				d.Pos.Line, d.Pos.Col, d.End.Line, d.End.Col, d.Severity, d.Code, d.Message))
		}
		return out
	}

	got, want := render(withComments), render(withoutComments)
	if len(got) != len(want) {
		t.Fatalf("the commented source lints to %d diagnostics and the blanked one to %d:\n%v\n%v", len(got), len(want), got, want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("diagnostic %d differs:\n  with comments: %s\n  blanked:       %s", i+1, got[i], want[i])
		}
	}
	t.Logf("comments and their blanked control lint to the same %d diagnostics", len(got))
}
