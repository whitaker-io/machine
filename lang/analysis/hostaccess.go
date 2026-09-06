// Package analysis - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package analysis

import (
	"errors"
	goast "go/ast"
	goparser "go/parser"
	goscanner "go/scanner"
	gotoken "go/token"
	"reflect"
	"sort"

	"github.com/whitaker-io/machine/lang/ast"
)

// hostAccessorName is the accessor a node function must not reach.
const hostAccessorName = "Host"

// The synthetic prologue a lifted .flow span is parsed inside, and the number of
// lines it adds. They travel together because changing one without the other
// silently moves every reported position.
const (
	flowFuncPrologue      = "package flow\n\n"
	flowFuncPrologueLines = 2
)

// HostAccessAnalyzer refuses a node function that reaches the machine's
// host-side store accessor.
var HostAccessAnalyzer = &Analyzer{
	Name: "hostaccess",
	Doc: "hostaccess refuses a node function that reaches the machine's host-side store accessor. A " +
		"node reaches shared storage through its FRAME, which is scoped to the datum it is running " +
		"for; the host accessor is the machine's own side of the store and reaching it from inside " +
		"a node body is a visible contortion the structural boundary exists to refuse. The check " +
		"was chosen over a runtime guard because a stack-walk cannot cross a goroutine boundary, " +
		"so any `go func()` inside a node evades it. IT READS .flow-RESIDENT FUNC BODIES ONLY: the " +
		"corpus is the func declarations this module's own parser captured out of .flow sources, " +
		"reconstructed exactly as the emitter reconstructs them and parsed with go/parser. Node " +
		"functions written as closures or as funcs in HAND-WRITTEN CONSUMER Go are out of its " +
		"sight entirely, because identifying one needs Go type resolution this module does not " +
		"do, so a clean hostaccess result is not a proof that no node reaches the host. THE MATCH " +
		"IS A ZERO-ARGUMENT CALL of a selector named Host, which is what separates the accessor " +
		"from a struct FIELD named Host and from a call named Host that carries arguments. The " +
		"uncalled method value is deliberately NOT matched: this module resolves no types, so " +
		"flagging every uncalled selector named Host would flag every request URL's Host field. A " +
		"func body that does not parse as Go is REPORTED at the declaration rather than skipped, " +
		"because a body this analyzer could not read is a body it cannot clear.",
	Requires:   []*Analyzer{SymbolsAnalyzer},
	Run:        runHostAccess,
	ResultType: reflect.TypeOf((*HostAccesses)(nil)),
}

// HostAccess is one reach for the host accessor, positioned in the .flow source
// the func body was lifted out of.
type HostAccess struct {
	Path string
	Func string
	Pos  ast.Position
	End  ast.Position
}

// HostAccesses is the analyzer's result: every host-accessor reach in the run.
type HostAccesses struct {
	Sites []HostAccess
}

// runHostAccess reads every .flow-resident func declaration the symbols analyzer
// tabled and inspects its verbatim Go body.
//
// THE INPUT IS THE SYMBOL TABLE RATHER THAN Source.File. FileSymbols.Funcs
// carries the whole ast.FuncDecl, body span included, so re-walking the tree
// would produce the same declarations by a second route that could disagree with
// the first.
func runHostAccess(p *Pass) (any, error) {
	table, ok := p.ResultOf[SymbolsAnalyzer].(*SymbolTable)
	if !ok {
		return nil, errNoSymbols
	}

	out := &HostAccesses{}
	for f := range table.Files {
		file := &table.Files[f]
		// SORTED, because Funcs is a map and an unsorted walk would order two
		// diagnostics at equal offsets by whatever the runtime handed back.
		for _, name := range sortedFuncNames(file.Funcs) {
			inspectFuncBody(p, file.Src, name, file.Funcs[name], out)
		}
	}

	return out, nil
}

// sortedFuncNames is a func table's keys in a stable order.
func sortedFuncNames(funcs map[string]ast.FuncDecl) []string {
	out := make([]string, 0, len(funcs))
	for name := range funcs {
		out = append(out, name)
	}
	sort.Strings(out)

	return out
}

// inspectFuncBody reconstructs one declaration's Go, parses it, and reports every
// host-accessor reach inside it.
func inspectFuncBody(p *Pass, src Source, name string, decl ast.FuncDecl, out *HostAccesses) {
	span := flowFuncPrologue + "func " + name + decl.Body.Text + "\n"
	mapper := newBodyMapper(name, decl)

	fset := gotoken.NewFileSet()
	parsed, err := goparser.ParseFile(fset, src.Path, span, goparser.SkipObjectResolution)
	if err != nil {
		reportUnparseableBody(p, src, decl, mapper, err)

		return
	}

	// THE WALK COVERS THE WHOLE RECONSTRUCTED FILE, closures included. A node
	// body that hands the reach to a func literal, or to a `go func()`, is the
	// same reach in a different wrapper — and the goroutine case is precisely the
	// one a runtime stack-walk cannot see, which is why this check is static.
	goast.Inspect(parsed, func(n goast.Node) bool {
		sel, isAccessor := hostAccessorCall(n)
		if !isAccessor {
			return true
		}
		site := HostAccess{
			Path: src.Path,
			Func: name,
			Pos:  mapper.at(fset.Position(sel.Sel.NamePos)),
			End:  mapper.at(fset.Position(sel.Sel.End() + gotoken.Pos(len("()")))),
		}
		out.Sites = append(out.Sites, site)
		reportHostAccess(p, src, site)

		return true
	})
}

// hostAccessorCall reports whether a node is a ZERO-ARGUMENT call of a selector
// named Host, and returns the selector it hangs off.
//
// THE TWO CONDITIONS ARE THE DISCRIMINATION, not defensive typing. Dropping the
// call requirement flags a struct FIELD named Host — every request URL's — and
// dropping the arity requirement flags a call named Host that takes arguments,
// which is an ordinary method name rather than the machine's accessor.
func hostAccessorCall(n goast.Node) (*goast.SelectorExpr, bool) {
	call, isCall := n.(*goast.CallExpr)
	if !isCall || len(call.Args) != 0 {
		return nil, false
	}
	sel, isSelector := call.Fun.(*goast.SelectorExpr)
	if !isSelector || sel.Sel.Name != hostAccessorName {
		return nil, false
	}

	return sel, true
}

// reportHostAccess names the func, the accessor and the route that is legal, so
// an author reading the diagnostic has somewhere to go.
func reportHostAccess(p *Pass, src Source, site HostAccess) {
	p.Report(src, Diagnostic{
		Pos: site.Pos,
		End: site.End,
		Message: "func " + site.Func + " reaches the machine's host-side store accessor from inside a node " +
			"body. A node reaches shared storage through its frame, which is scoped to the datum it is " +
			"running for; the host accessor is the machine's own side and is not a node's to touch. Move " +
			"the read or the write onto the frame, or move the work out of the node",
		Severity: SeverityError,
	})
}

// reportUnparseableBody reports a func body that is not Go AT THE DECLARATION.
//
// It is a report rather than a skip because a body this analyzer could not read
// is a body it could not clear, and a silent skip would render as a clean result
// for exactly the func most likely to be hiding something.
// The name is read off the declaration rather than passed alongside it: the
// symbol table keys Funcs on decl.Name.Name, so a second copy could only ever
// disagree with the first.
func reportUnparseableBody(p *Pass, src Source, decl ast.FuncDecl, mapper bodyMapper, err error) {
	name := decl.Name.Name
	// The failure's own position is inside the RECONSTRUCTED span, so it is
	// mapped back onto the .flow before it is quoted; the diagnostic itself sits
	// on the declaration, which is what an author can act on.
	detail, at := err.Error(), decl.Name.NamePos
	var list goscanner.ErrorList
	if errors.As(err, &list) && len(list) > 0 {
		detail, at = list[0].Msg, mapper.at(list[0].Pos)
	}

	p.Report(src, Diagnostic{
		Pos: decl.Name.NamePos,
		End: endOfName(decl.Name.NamePos, name),
		Message: "the body of func " + name + " does not parse as Go, so this analyzer cannot tell whether " +
			"it reaches the host accessor: " + detail + " at " + at.String(),
		Severity: SeverityError,
	})
}

// bodyMapper carries a position inside the reconstructed Go span back onto the
// .flow the span was lifted out of.
//
// The reconstruction is "func " + name + Body.Text under a two-line prologue, so
// everything from the body span's first byte onward is BYTE-IDENTICAL to the
// .flow. Only the anchor differs, and the anchor is what this holds.
type bodyMapper struct {
	start    ast.Position
	goLine   int
	goCol    int
	goOffset int
}

// newBodyMapper anchors the mapping on the body span's opening parenthesis,
// which is the first byte the two texts share.
func newBodyMapper(name string, decl ast.FuncDecl) bodyMapper {
	return bodyMapper{
		start:    decl.Body.Start,
		goLine:   flowFuncPrologueLines + 1,
		goCol:    len("func ") + len(name) + 1,
		goOffset: len(flowFuncPrologue) + len("func ") + len(name),
	}
}

// at maps one parsed position onto the .flow.
//
// THE COLUMN IS CARRIED UNCHANGED EXCEPT ON THE DECLARATION'S OWN LINE. Every
// later line of the span is verbatim, indentation included, so its columns
// already agree; the first line is the one the reconstruction rewrote, and there
// the column is measured from the anchor instead.
func (m bodyMapper) at(pos gotoken.Position) ast.Position {
	out := ast.Position{
		Offset: m.start.Offset + (pos.Offset - m.goOffset),
		Line:   m.start.Line + (pos.Line - m.goLine),
		Col:    pos.Column,
	}
	if pos.Line == m.goLine {
		out.Col = m.start.Col + (pos.Column - m.goCol)
	}

	return out
}
