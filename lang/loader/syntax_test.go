// Package loader - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package loader

import (
	goast "go/ast"
	"testing"
)

// stdlibDependency is a package the subject module imports and does not declare.
// It is the same-run control for the root set: it IS indexed, because Scope
// answers for it, and it is NOT a root, because no pattern named it.
const stdlibDependency = "encoding/gob"

// TestSyntaxHandsBackTheParsedFilesAndTheirTypeInformation is the whole of the
// new accessor's contract against a real module on disk.
//
// IT IS ONE TEST BECAUSE THE LEGS SHARE ONE LOAD, and because the root leg's
// control — a package that is indexed but is not a root — is only a control if
// both halves are measured in the same run.
func TestSyntaxHandsBackTheParsedFilesAndTheirTypeInformation(t *testing.T) {
	loaded, err := Load(subjectDir, []string{"./..."})
	if err != nil {
		t.Fatalf("the clean fixture module did not load: %v", err)
	}

	t.Run("the roots are the packages the patterns named, not every reachable one", func(t *testing.T) {
		roots := loaded.Roots()
		if len(roots) == 0 {
			t.Fatal("CONTROL FAILED: the load reported no roots at all, so nothing below discriminates")
		}
		if !contains(roots, subjectPath) {
			t.Fatalf("the root set %v does not carry the package the pattern matched, %s", roots, subjectPath)
		}

		// THE CONTROL. encoding/gob is reachable and indexed — Scope answers for
		// it — and it is not a root. A Roots that answered with every indexed path
		// would carry it, and a walk over that answer walks the standard library.
		if _, indexed := loaded.Scope(stdlibDependency); !indexed {
			t.Fatalf("CONTROL FAILED: %s is not indexed, so its absence from the roots proves nothing",
				stdlibDependency)
		}
		if contains(roots, stdlibDependency) {
			t.Errorf("the root set carries %s, which no pattern named: %v", stdlibDependency, roots)
		}
	})

	t.Run("the root set is the caller's copy", func(t *testing.T) {
		first := loaded.Roots()
		if len(first) == 0 {
			t.Fatal("CONTROL FAILED: an empty root set cannot show whether a caller can corrupt it")
		}
		first[0] = "example.com/scribbled-over"

		if second := loaded.Roots(); second[0] == "example.com/scribbled-over" {
			t.Errorf("a caller's write reached the run's own root set: %v", second)
		}
	})

	t.Run("a loaded package hands back its files and their type information", func(t *testing.T) {
		syntax, ok := loaded.Syntax(subjectPath)
		if !ok {
			t.Fatalf("%s was loaded with syntax and type information, and Syntax reported neither", subjectPath)
		}
		if syntax.Path != subjectPath {
			t.Errorf("the syntax names package %q, want %q", syntax.Path, subjectPath)
		}
		if syntax.Fset == nil {
			t.Fatal("the syntax carries no file set, so no position in it can be resolved")
		}
		if len(syntax.Files) == 0 {
			t.Fatal("the syntax carries no parsed files")
		}
		if syntax.Info == nil {
			t.Fatal("the syntax carries no type information")
		}

		// THE POSITIVE CONTROL ON THE TYPE INFORMATION. A non-nil *types.Info
		// whose maps are empty would satisfy every assertion above and attribute
		// nothing, which is the exact shape a load without NeedTypesInfo produces.
		if !defines(syntax, "Payload") {
			t.Errorf("the type information attributes no definition of Payload, so it is empty rather than filled")
		}

		named := 0
		for _, file := range syntax.Files {
			if position := syntax.Fset.Position(file.Pos()); position.Filename != "" {
				named++
			}
		}
		if named != len(syntax.Files) {
			t.Errorf("%d of %d parsed files resolve to a file name through the set", named, len(syntax.Files))
		}
	})

	t.Run("a package that was never loaded is absent rather than empty", func(t *testing.T) {
		syntax, ok := loaded.Syntax("example.com/never-loaded")
		if ok {
			t.Fatal("Syntax invented an answer for a package that was never loaded")
		}
		if syntax.Path != "" || syntax.Files != nil || syntax.Info != nil || syntax.Fset != nil {
			t.Errorf("Syntax refused but still handed back %+v", syntax)
		}
	})
}

// contains reports whether a string is in a slice, for the root-set assertions.
func contains(all []string, want string) bool {
	for _, one := range all {
		if one == want {
			return true
		}
	}

	return false
}

// defines reports whether the type information carries a definition for a
// top-level name, which is what makes the *types.Info a filled one.
func defines(syntax Syntax, name string) bool {
	for _, file := range syntax.Files {
		for _, decl := range file.Decls {
			gen, isGen := decl.(*goast.GenDecl)
			if !isGen {
				continue
			}
			for _, spec := range gen.Specs {
				typed, isType := spec.(*goast.TypeSpec)
				if isType && typed.Name.Name == name && syntax.Info.Defs[typed.Name] != nil {
					return true
				}
			}
		}
	}

	return false
}
