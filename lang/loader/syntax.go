// Package loader - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package loader

import (
	goast "go/ast"
	"go/token"
	"go/types"
)

// Syntax is one loaded package's parsed Go together with the type information
// that attributes it.
//
// IT IS THE LOAD'S OWN VALUES RATHER THAN A COPY OF THEM. go/packages fills the
// files and the *types.Info once per load, they are large, and every consumer
// reads them; handing back a duplicate would double the memory a generation run
// holds to protect against a mutation no caller has a reason to make. A caller
// that mutates a parsed tree corrupts the run it was handed, which is the same
// contract Scope already carries for the package scope.
//
// THE FILE SET RIDES WITH THE FILES because neither is usable without the other:
// a go/ast node carries a token.Pos, and only the set that parsed it turns that
// into a file name, a line and a column.
type Syntax struct {
	// Path is the import path of the package these files belong to.
	Path string
	// Fset resolves every position in Files.
	Fset *token.FileSet
	// Files are the package's parsed Go files, in the order go/packages parsed
	// them.
	Files []*goast.File
	// Info attributes the expressions in Files: what each identifier denotes,
	// what each expression's type is, and which object each selector selects.
	Info *types.Info
}

// Syntax reports one package's parsed files and their type information, and
// whether the package was loaded WITH BOTH.
//
// THE SECOND RESULT DISCRIMINATES THREE STATES, and the caller needs it to. A
// package that was never loaded, a dependency loaded for its types alone, and a
// package whose type-checking produced no information are all answered false —
// because what a caller does with any of them is the same thing: leave that
// package unread and say so. What it must never do is walk an empty file list
// and report the package clean, which is what an accessor answering with a zero
// value beside no signal would produce.
//
// loadMode requests NeedSyntax and NeedTypesInfo for exactly this, and load.go
// says so at the mode's own declaration: they are what let a consumer map a Go
// expression back to a type. Nothing handed either out before this method, so
// every consumer that needed to read Go had to re-parse it.
func (p *Packages) Syntax(pkgPath string) (Syntax, bool) {
	pkg, ok := p.byPath[pkgPath]
	if !ok || pkg.TypesInfo == nil || pkg.Fset == nil || len(pkg.Syntax) == 0 {
		return Syntax{}, false
	}

	return Syntax{Path: pkgPath, Fset: pkg.Fset, Files: pkg.Syntax, Info: pkg.TypesInfo}, true
}

// Roots reports the import paths of the packages the load's PATTERNS matched, in
// a stable order.
//
// IT IS NOT THE INDEX. Load indexes every REACHABLE package, standard library
// and transitive dependency included, because a spelling in one package
// routinely names a type another declares. A consumer that walks the index is
// therefore walking the standard library on every run; a consumer that walks
// this walks the code the patterns were about. Both questions are real, and this
// is the narrow one.
//
// THE SLICE IS A COPY, because the run's own root set is not a caller's to
// reorder or overwrite — unlike the parsed trees Syntax hands back, this one is
// small enough that the copy costs nothing worth reasoning about.
func (p *Packages) Roots() []string {
	out := make([]string, len(p.roots))
	copy(out, p.roots)

	return out
}
