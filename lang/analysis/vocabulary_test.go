// Package analysis - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package analysis

import (
	goast "go/ast"
	goparser "go/parser"
	gotoken "go/token"
	"os"
	"strings"
	"testing"
)

// The runtime's own sources, read ACROSS THE MODULE BOUNDARY the way astTestdata
// and lang/lint's package doc are. This module cannot IMPORT the root module —
// the root module imports nothing of lang — so the coupling is read from source.
const (
	runtimeTypes = "../../types.go"
	runtimeFlow  = "../../flow.go"
)

// TestTheNodeConstructorSetIsTheRuntimesOwn joins this package's copy of the
// builder vocabulary to the runtime that declares it.
//
// WHY IT EXISTS. hostreach.go names three builder methods and three node-function
// types. That is a THIRD copy of a fact the runtime declares and the assembler's
// emitter also holds, and no compiler joins the three: if the runtime grows an
// eighth builder method that takes a node function, this analyzer keeps passing
// and stays silent about every node function wired through it. Silence is the
// failure mode a check like this one cannot afford, so the set is DERIVED from
// the runtime's source here and compared to the declaration.
//
// THE DERIVATION IS THE ASSEMBLER'S OWN RULE, restated over the same sources its
// plan_test uses: a builder method is an EXPORTED method whose receiver is
// Flow[...]; it takes a node function when one of its parameters is spelled with
// a function type the runtime declares in types.go.
func TestTheNodeConstructorSetIsTheRuntimesOwn(t *testing.T) {
	declared := declaredFuncTypes(t, runtimeTypes)
	if len(declared) < 3 {
		t.Fatalf("CONTROL FAILED: %s declares %d function types, so the derivation below read nothing "+
			"useful: %v", runtimeTypes, len(declared), sortedKeys(declared))
	}
	if !declared["Transformation"] {
		t.Fatalf("CONTROL FAILED: the runtime's declared function types are %v, and Transformation is not "+
			"among them — the derivation is reading the wrong file", sortedKeys(declared))
	}

	methods, taken := buildersTakingAFunction(t, runtimeFlow, declared)

	if got, want := sortedKeys(methods), sortedKeys(nodeConstructors); strings.Join(got, ",") !=
		strings.Join(want, ",") {
		t.Errorf("the runtime's builder methods that take a node function are %v and this package looks for "+
			"%v; a builder outside the second list is one whose node functions are never read", got, want)
	}
	if got, want := sortedKeys(taken), sortedKeys(nodeFunctionType); strings.Join(got, ",") !=
		strings.Join(want, ",") {
		t.Errorf("the runtime's builders take %v and this package recognizes %v as node-function types",
			got, taken)
	}
	t.Logf("derived from %s and %s: builders %v taking %v", runtimeFlow, runtimeTypes,
		sortedKeys(methods), sortedKeys(taken))
}

// declaredFuncTypes is every named FUNCTION type the runtime declares in one
// file, which is the vocabulary a builder's parameter can be spelled with.
func declaredFuncTypes(t *testing.T, path string) map[string]bool {
	t.Helper()

	out := map[string]bool{}
	for _, decl := range parseRuntimeFile(t, path).Decls {
		gen, isGen := decl.(*goast.GenDecl)
		if !isGen {
			continue
		}
		for _, spec := range gen.Specs {
			typed, isType := spec.(*goast.TypeSpec)
			if !isType {
				continue
			}
			if _, isFunc := typed.Type.(*goast.FuncType); isFunc {
				out[typed.Name.Name] = true
			}
		}
	}

	return out
}

// buildersTakingAFunction reports the exported Flow methods whose parameters name
// one of those function types, and which of those types they name.
func buildersTakingAFunction(t *testing.T, path string, declared map[string]bool) (
	map[string]bool, map[string]bool) {
	t.Helper()

	methods, taken := map[string]bool{}, map[string]bool{}
	for _, decl := range parseRuntimeFile(t, path).Decls {
		fn, isFunc := decl.(*goast.FuncDecl)
		if !isFunc || fn.Recv == nil || len(fn.Recv.List) == 0 || !fn.Name.IsExported() {
			continue
		}
		if receiverName(fn.Recv.List[0].Type) != flowTypeName {
			continue
		}
		for _, param := range fn.Type.Params.List {
			name := baseTypeName(param.Type)
			if !declared[name] {
				continue
			}
			methods[fn.Name.Name] = true
			taken[name] = true
		}
	}

	return methods, taken
}

// baseTypeName is a parameter type's name through its type arguments, so
// Transformation[U, V] reads as Transformation.
func baseTypeName(expr goast.Expr) string {
	switch typ := expr.(type) {
	case *goast.Ident:
		return typ.Name
	case *goast.IndexExpr:
		return baseTypeName(typ.X)
	case *goast.IndexListExpr:
		return baseTypeName(typ.X)
	default:
		return ""
	}
}

// parseRuntimeFile parses one of the runtime's own sources.
func parseRuntimeFile(t *testing.T, path string) *goast.File {
	t.Helper()

	body, err := os.ReadFile(path) //nolint:gosec // a test reading the runtime's own source
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	parsed, err := goparser.ParseFile(gotoken.NewFileSet(), path, body, goparser.SkipObjectResolution)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}

	return parsed
}
