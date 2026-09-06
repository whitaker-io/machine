// Package analysis - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package analysis

import (
	"fmt"
	goast "go/ast"
	gotoken "go/token"
	"go/types"
	"reflect"
	"sort"
	"strconv"

	"github.com/whitaker-io/machine/lang/ast"
	"github.com/whitaker-io/machine/lang/loader"
)

// hostReachName is the analyzer's Name, and therefore the Code every finding it
// reports carries.
//
// It is deliberately NOT the .flow-resident check's name: the two read different
// corpora, and a consumer suppressing or routing one rule must not silence the
// other. hostAccessorName, the accessor they both look for, IS shared — it is one
// fact about the runtime and a second copy could only ever disagree.
const hostReachName = "hostreach"

// The runtime's node-function types and the builder methods that take one.
//
// THE SET IS THE RUNTIME'S AND IS PINNED IN TWO OTHER PLACES — the emitter's
// builder vocabulary and the runtime's own declarations — and NO COMPILER JOINS
// THE THREE. If the runtime grows an eighth builder method that takes a node
// function, this list is the copy that will not move, and the failure is silence
// rather than a red. The names are checked against a RESOLVED object's package
// and name rather than against a spelling, so an alias or a dot import cannot
// evade them, but the SET itself is a declaration.
var (
	nodeConstructors = map[string]bool{"Map": true, "If": true, "Tee": true}
	nodeFunctionType = map[string]bool{"Transformation": true, "Filter": true, "Duplicator": true}
)

// The runtime types a constructor call and an accessor call hang off.
const (
	flowTypeName    = "Flow"
	machineTypeName = "Machine"
)

// hostReachDoc is a constant so a gate can assert the text without constructing
// an analyzer, which this one cannot be without a caller-supplied package set.
const hostReachDoc = "hostreach refuses a node function written in HAND-WRITTEN CONSUMER GO that reaches " +
	"the machine's host-side store accessor. A node reaches shared storage through its FRAME, which is " +
	"scoped to the datum it is running for; the host accessor is the machine's own side of the store, and " +
	"the runtime states that rule where the accessor is declared. IT IDENTIFIES A NODE FUNCTION THROUGH " +
	"go/types AND NEVER BY SPELLING: the callee must resolve to the runtime's own builder method on a Flow, " +
	"and the argument must sit at a parameter the runtime declares as a Transformation, a Filter or a " +
	"Duplicator — so an aliased import, a dot import and a local method that merely happens to be named Map " +
	"are each answered correctly rather than by luck. IT READS CONSUMER GO ONLY WHEN flowc GENERATES: the " +
	"corpus is the package set that generation run loaded, so a consumer who hand-builds a machine with no " +
	".flow beside it is NOT GATED by this check at all, and a clean result over one directory says nothing " +
	"about another. ErrorHandler bodies are not node functions here: a handler runs in the SUPERVISOR " +
	"rather than in a node and reaches a constructor through an option, and the same is true of an " +
	"EdgeFactory, which builds the node's inbound transport. THE CORPUS IS EVERY PACKAGE THE PATTERNS " +
	"NAMED, the generated one included, because a consumer routinely generates into the package their own " +
	"wiring lives in and excluding it would exclude the hand-written code this check exists to read. WHAT " +
	"IT DOES NOT SEE, so that a silence is not " +
	"over-read: a node function reached through a variable, a struct field or a parameter rather than " +
	"written at the call site or named there directly; a function whose declaring package was loaded " +
	"without syntax; and anything the node function CALLS, since the walk reads that function's own body " +
	"and the closures inside it rather than the helpers it invokes."

// HostReach is one reach for the host accessor from inside a node function,
// positioned in the consumer's own Go file.
type HostReach struct {
	// Path is the file the reach is IN, which is the file declaring the node
	// function's body rather than the one holding the constructor call. The two
	// differ whenever a node function is passed by name.
	Path string
	// Func names the node function, after the declaration that holds it.
	Func string
	// Constructor is the builder method that made it a node function.
	Constructor string
	Pos         ast.Position
	End         ast.Position
}

// NodeFunction is one function the walk identified as a node function, whether or
// not it reached the accessor.
//
// IT EXISTS SO A SILENCE CAN BE BELIEVED. A walk that read nothing reports
// nothing, and that result is byte-identical to a clean corpus; recording what
// was read is what lets a test assert that the functions it expects silence about
// were actually reached.
type NodeFunction struct {
	Path        string
	Func        string
	Constructor string
	Pos         ast.Position
}

// HostReaches is the analyzer's result: what it found, what it read, and what it
// could not read.
type HostReaches struct {
	Sites     []HostReach
	Inspected []NodeFunction
	// Unread is every node-function argument the walk identified and could not
	// resolve to a body: a function held in a variable, returned by a call, taken
	// from a struct field, or declared in a package this run loaded without
	// syntax. THE DOC DISCLOSES THIS GAP AND THIS FIELD MEASURES IT — a consumer
	// asking whether a clean result covered its wiring can see how much of it the
	// walk could not open, rather than being told in prose.
	Unread []NodeFunction
	// Packages are the import paths the walk read, and Files is how many parsed
	// files it walked. Both are the census a floor is asserted against.
	Packages []string
	Files    int
}

// HostReachAnalyzer builds the consumer-Go host-reach pass over a caller-supplied
// package set.
//
// IT IS A CONSTRUCTOR RATHER THAN A REGISTERED VAR, on exactly the terms type
// inference and the serialization derivation are: the *loader.Packages it reads
// is owned by the caller above both modules, and Pass has no channel through
// which a driver could deliver one. Registering it would leave it silent whenever
// no package set was supplied, which is a silently degraded lane. It must never
// be added to analyzers.go.
//
// IT TAKES NO PACKAGE PATH, unlike the two constructed analyzers beside it, and
// the reason is worth stating because the symmetry is tempting. Those two resolve
// a flow's spellings AGAINST one package, so they need to know which. This one
// reads every package the run loaded, INCLUDING the one being generated: a
// consumer routinely generates into the package their own wiring lives in, so
// excluding the generated path would silently exclude the hand-written code this
// check exists to read. The generated code itself cannot reach the accessor —
// the emitter's node functions receive a frame and never a machine — so there is
// nothing to exclude it for.
func HostReachAnalyzer(pkgs *loader.Packages) *Analyzer {
	run := &hostReachRun{pkgs: pkgs}

	return &Analyzer{
		Name:       hostReachName,
		Doc:        hostReachDoc,
		Run:        run.run,
		ResultType: reflect.TypeOf((*HostReaches)(nil)),
	}
}

// hostReachRun carries one analysis run's state: the package set and the memo of
// resolved package syntax.
type hostReachRun struct {
	pkgs *loader.Packages
	// syntax memoizes the loader lookup, because a node function passed BY NAME
	// sends the walk back to its declaring package once per call site.
	syntax map[string]loader.Syntax
}

// run walks the consumer packages the caller's patterns named.
//
// THE WALK IS OVER THE ROOTS RATHER THAN THE INDEX. A load indexes every
// REACHABLE package — the standard library and every transitive dependency — and
// walking that would re-parse the world on every generation. The roots are the
// code the patterns were about: the input directory's own packages, the generated
// package, and the modules the .flow sources import.
//
// A NIL PACKAGE SET IS REFUSED, wrapping the shared sentinel so errors.Is holds
// for a caller both directly and through the driver.
func (r *hostReachRun) run(p *Pass) (any, error) {
	if r.pkgs == nil {
		return nil, fmt.Errorf("host reach: %w", errNoPackages)
	}

	out := &HostReaches{}
	for _, path := range r.pkgs.Roots() {
		syntax, loaded := r.syntaxOf(path)
		if !loaded {
			continue
		}
		out.Packages = append(out.Packages, path)
		out.Files += len(syntax.Files)
		for _, file := range syntax.Files {
			r.inspectFile(p, syntax, file, out)
		}
	}
	// THE RESULT IS ORDERED, because a consumer reading Sites reads a table and
	// the driver's own sort orders only the DIAGNOSTICS. Two runs over one
	// package set answer identically.
	out.Sites = sortedReaches(out.Sites)

	return out, nil
}

// syntaxOf is the memoized loader lookup.
//
// A PACKAGE LOADED WITHOUT SYNTAX IS ABSENT rather than empty, which is the
// loader's own distinction: a walk over an empty file list would report that
// package clean, and this analyzer's Doc says instead that it did not read it.
func (r *hostReachRun) syntaxOf(pkgPath string) (loader.Syntax, bool) {
	if syntax, memoized := r.syntax[pkgPath]; memoized {
		return syntax, syntax.Info != nil
	}
	if r.syntax == nil {
		r.syntax = map[string]loader.Syntax{}
	}

	syntax, loaded := r.pkgs.Syntax(pkgPath)
	r.syntax[pkgPath] = syntax

	return syntax, loaded
}

// inspectFile reports every host-accessor reach inside the node functions one
// file wires.
//
// THE LABELS ARE BUILT FIRST, IN A SEPARATE PASS. A func literal is named after
// the declaration that holds it and its ordinal within it, the way Go names a
// closure, and the ordinal is only knowable once the whole declaration has been
// walked — while the constructor call that makes the literal a node function is
// encountered BEFORE the literal itself.
func (r *hostReachRun) inspectFile(p *Pass, syntax loader.Syntax, file *goast.File, out *HostReaches) {
	labels := closureLabels(file)

	goast.Inspect(file, func(n goast.Node) bool {
		call, isCall := n.(*goast.CallExpr)
		if !isCall {
			return true
		}
		constructor, argument, isConstructor := nodeConstructorCall(syntax.Info, call)
		if !isConstructor {
			return true
		}
		r.inspectNodeFunction(p, syntax, nodeFunctionSite{
			constructor: constructor,
			argument:    argument,
			call:        syntax.Fset.Position(call.Lparen),
			labels:      labels,
		}, out)

		return true
	})
}

// nodeFunctionSite is one constructor call's node-function argument, with what
// the walk needs to name and position it.
type nodeFunctionSite struct {
	constructor string
	argument    goast.Expr
	call        gotoken.Position
	labels      map[*goast.FuncLit]string
}

// inspectNodeFunction resolves one node-function argument to a body and walks it.
//
// AN ARGUMENT THAT RESOLVES TO NO BODY IS LEFT UNREAD AND IS NOT REPORTED. A node
// function reached through a variable, a struct field or a parameter is a
// function this analyzer never saw the body of, and reporting it would be a
// finding about a body nobody read; the Doc discloses the gap instead, because a
// silence a consumer over-reads is the failure mode here.
func (r *hostReachRun) inspectNodeFunction(p *Pass, syntax loader.Syntax, site nodeFunctionSite,
	out *HostReaches) {
	body, resolved := r.bodyOf(syntax, site)
	if !resolved {
		out.Unread = append(out.Unread, NodeFunction{
			Path:        site.call.Filename,
			Func:        "",
			Constructor: site.constructor,
			Pos:         positionOf(site.call),
		})

		return
	}

	out.Inspected = append(out.Inspected, NodeFunction{
		Path:        body.in.Fset.Position(body.block.Lbrace).Filename,
		Func:        body.name,
		Constructor: site.constructor,
		Pos:         positionOf(body.in.Fset.Position(body.block.Lbrace)),
	})

	// THE WALK COVERS THE WHOLE BODY, closures and goroutines included. A node
	// body that hands the reach to a func literal, or to a `go func()`, is the
	// same reach in a different wrapper — and the goroutine case is precisely the
	// one a runtime stack walk cannot see, which is why this check is static.
	goast.Inspect(body.block, func(n goast.Node) bool {
		sel, isAccessor := hostAccessorReach(body.in.Info, n)
		if !isAccessor {
			return true
		}
		reach := HostReach{
			Path:        body.in.Fset.Position(sel.Sel.NamePos).Filename,
			Func:        body.name,
			Constructor: site.constructor,
			Pos:         positionOf(body.in.Fset.Position(sel.Sel.NamePos)),
			End:         positionOf(body.in.Fset.Position(sel.End())),
		}
		out.Sites = append(out.Sites, reach)
		reportHostReach(p, reach, site.call)

		return true
	})
}

// bodyOf resolves a node-function argument to the block that is its body, the
// package syntax that block belongs to, and the name to report it under.
//
// TWO SHAPES RESOLVE and they resolve differently. A func literal is written at
// the call site and its body is right there. A function passed BY NAME is
// resolved through go/types to the object it denotes and then to that object's
// declaration, WHICH MAY BE IN ANOTHER FILE OR ANOTHER PACKAGE — that is the case
// a walk over the call site's own closures cannot see at all, and it is the
// ordinary shape in generated wiring.
func (r *hostReachRun) bodyOf(syntax loader.Syntax, site nodeFunctionSite) (nodeBody, bool) {
	switch argument := goast.Unparen(site.argument).(type) {
	case *goast.FuncLit:
		return nodeBody{block: argument.Body, in: syntax, name: site.labels[argument]}, true
	case *goast.Ident:
		return r.declaredBody(syntax, argument)
	case *goast.SelectorExpr:
		// A node function named through a package qualifier: the identifier that
		// denotes the function is the SELECTED one, and go/types resolves it to an
		// object in the package that declares it rather than in this one.
		return r.declaredBody(syntax, argument.Sel)
	default:
		return nodeBody{}, false
	}
}

// nodeBody is one resolved node function: the block to walk, the package syntax
// that block belongs to, and the name to report it under.
//
// IT IS A STRUCT RATHER THAN THREE RESULTS BESIDE A BOOL because the module's
// linter caps a function at three return results, and two of the three would be
// unnamed at the call site — the same reason the assembler's gate carries its
// three answers in one value.
type nodeBody struct {
	block *goast.BlockStmt
	in    loader.Syntax
	name  string
}

// declaredBody resolves a named function to its declaration's body.
func (r *hostReachRun) declaredBody(syntax loader.Syntax, name *goast.Ident) (nodeBody, bool) {
	obj, isFunc := syntax.Info.Uses[name].(*types.Func)
	if !isFunc || obj.Pkg() == nil {
		return nodeBody{}, false
	}

	declaring, loaded := r.syntaxOf(obj.Pkg().Path())
	if !loaded {
		return nodeBody{}, false
	}
	for _, file := range declaring.Files {
		for _, decl := range file.Decls {
			fn, isFunc := decl.(*goast.FuncDecl)
			if !isFunc || fn.Body == nil || fn.Name.NamePos != obj.Pos() {
				continue
			}

			return nodeBody{block: fn.Body, in: declaring, name: declarationName(fn)}, true
		}
	}

	return nodeBody{}, false
}

// nodeConstructorCall reports whether a call is one of the runtime's node
// constructors, and returns the argument that is the node function.
//
// BOTH HALVES ARE RESOLVED THROUGH go/types, and each rules out a different way
// of being wrong. Matching the callee's OBJECT — its package and its name, on a
// receiver that is the runtime's Flow — is what makes an aliased import, a dot
// import and a local method named Map answer correctly. Finding the argument by
// its PARAMETER'S declared type rather than by position is what makes Tee work:
// its Duplicator takes a payload rather than a Frame, so a Frame-parameter
// heuristic misses every Tee argument, and a positional guess breaks the moment a
// builder gains an argument.
func nodeConstructorCall(info *types.Info, call *goast.CallExpr) (string, goast.Expr, bool) {
	obj, signature, isBuilder := runtimeBuilder(info, call)
	if !isBuilder {
		return "", nil, false
	}

	params := signature.Params()
	for i := 0; i < params.Len() && i < len(call.Args); i++ {
		if nodeFunctionParameter(params.At(i).Type()) {
			return obj.Name(), call.Args[i], true
		}
	}

	return "", nil, false
}

// runtimeBuilder resolves a call's callee to one of the runtime's builder methods
// on a Flow, and hands back the object and its signature.
//
// It is split out of the argument scan because the two questions are separable:
// this one is entirely about IDENTITY — whose method is this — and the scan above
// is about the parameter list that identity brings with it.
func runtimeBuilder(info *types.Info, call *goast.CallExpr) (*types.Func, *types.Signature, bool) {
	selector, isSelector := call.Fun.(*goast.SelectorExpr)
	if !isSelector {
		return nil, nil, false
	}
	obj, isFunc := info.Uses[selector.Sel].(*types.Func)
	if !isFunc || !nodeConstructors[obj.Name()] || !declaredByTheRuntime(obj) {
		return nil, nil, false
	}
	signature, isSignature := obj.Type().(*types.Signature)
	if !isSignature || signature.Recv() == nil || !isRuntimeNamed(signature.Recv().Type(), flowTypeName) {
		return nil, nil, false
	}

	return obj, signature, true
}

// nodeFunctionParameter reports whether a parameter's type is one of the
// runtime's declared node-function types.
func nodeFunctionParameter(typ types.Type) bool {
	named, isNamed := types.Unalias(typ).(*types.Named)
	if !isNamed {
		return false
	}
	obj := named.Obj()

	return obj != nil && nodeFunctionType[obj.Name()] && declaredByTheRuntimeName(obj)
}

// hostAccessorReach reports whether a node is a ZERO-ARGUMENT call of the
// runtime's own host accessor on a *Machine, and returns the selector it hangs
// off.
//
// THE OBJECT IS RESOLVED, WHICH IS THE WHOLE DIFFERENCE FROM THE .flow-RESIDENT
// CHECK. That one has no types and so matches a zero-argument call of a selector
// spelled Host, which it must then defend against every struct field named Host.
// Here the call must resolve to the method the runtime declares on its Machine,
// so a request URL's Host, a local Host() of any arity and a Host on any other
// type are all silent by construction rather than by rule.
func hostAccessorReach(info *types.Info, n goast.Node) (*goast.SelectorExpr, bool) {
	call, isCall := n.(*goast.CallExpr)
	if !isCall || len(call.Args) != 0 {
		return nil, false
	}
	selector, isSelector := call.Fun.(*goast.SelectorExpr)
	if !isSelector {
		return nil, false
	}
	obj, isFunc := info.Uses[selector.Sel].(*types.Func)
	if !isFunc || obj.Name() != hostAccessorName || !declaredByTheRuntime(obj) {
		return nil, false
	}
	signature, isSignature := obj.Type().(*types.Signature)
	if !isSignature || signature.Recv() == nil || !isRuntimeNamed(signature.Recv().Type(), machineTypeName) {
		return nil, false
	}

	return selector, true
}

// declaredByTheRuntime reports whether an object is declared by the runtime
// module, which is the match that makes every spelling question moot.
func declaredByTheRuntime(obj types.Object) bool {
	return obj.Pkg() != nil && obj.Pkg().Path() == machinePath
}

// declaredByTheRuntimeName is declaredByTheRuntime for a type name, which carries
// its package the same way.
func declaredByTheRuntimeName(obj *types.TypeName) bool { return declaredByTheRuntime(obj) }

// isRuntimeNamed reports whether a type is the runtime's named type of that name,
// through however many pointers and instantiations it is written behind.
func isRuntimeNamed(typ types.Type, name string) bool {
	if pointer, isPointer := types.Unalias(typ).(*types.Pointer); isPointer {
		typ = pointer.Elem()
	}
	named, isNamed := types.Unalias(typ).(*types.Named)
	if !isNamed {
		return false
	}
	obj := named.Obj()

	return obj != nil && obj.Name() == name && declaredByTheRuntime(obj)
}

// closureLabels names every func literal in a file after the declaration that
// holds it and its ordinal within it — Enrich.func1, Enrich.func2 — which is how
// Go itself names a closure and therefore what an author reading a stack trace
// recognizes.
func closureLabels(file *goast.File) map[*goast.FuncLit]string {
	out := map[*goast.FuncLit]string{}
	for _, decl := range file.Decls {
		fn, isFunc := decl.(*goast.FuncDecl)
		if !isFunc || fn.Body == nil {
			continue
		}
		prefix, seen := declarationName(fn), 0
		goast.Inspect(fn.Body, func(n goast.Node) bool {
			literal, isLiteral := n.(*goast.FuncLit)
			if !isLiteral {
				return true
			}
			seen++
			out[literal] = prefix + ".func" + strconv.Itoa(seen)

			return true
		})
	}

	return out
}

// declarationName is a function declaration's name, qualified by its receiver
// type when it has one, so two methods of the same name on two types are two
// different node functions in a report.
func declarationName(fn *goast.FuncDecl) string {
	if fn.Recv == nil || len(fn.Recv.List) == 0 {
		return fn.Name.Name
	}

	return receiverName(fn.Recv.List[0].Type) + "." + fn.Name.Name
}

// receiverName is the receiver's type name, through a pointer and through type
// parameters.
func receiverName(expr goast.Expr) string {
	switch typ := expr.(type) {
	case *goast.StarExpr:
		return receiverName(typ.X)
	case *goast.IndexExpr:
		return receiverName(typ.X)
	case *goast.IndexListExpr:
		return receiverName(typ.X)
	case *goast.Ident:
		return typ.Name
	default:
		return ""
	}
}

// positionOf converts a go/token position into the framework's own.
//
// BOTH COUNT BYTES IN THE COLUMN, which is what makes the conversion a rename
// rather than a remap: lang/ast states that Position.Col counts bytes, and
// go/token.Position.Column does the same.
func positionOf(pos gotoken.Position) ast.Position {
	return ast.Position{Offset: pos.Offset, Line: pos.Line, Col: pos.Column}
}

// reportHostReach names the node function, the constructor call that made it one,
// and the route that is legal, so an author reading the diagnostic has somewhere
// to go.
//
// THE FILE THIS FINDING IS ABOUT IS CARRIED ON THE DIAGNOSTIC rather than
// composed into the message: ReportForeign keeps the analyzer's own Path, so the
// linter's renderings, the assembler's conversion and an editor's range all see
// the real file, the real line and the real column. The CONSTRUCTOR CALL is a
// second position, in a file that may not be this one, and one diagnostic cannot
// carry two — so that one is named in the text, which is what it is for.
func reportHostReach(p *Pass, site HostReach, at gotoken.Position) {
	p.ReportForeign(Diagnostic{
		Pos:  site.Pos,
		End:  site.End,
		Path: site.Path,
		Message: "the node function " + site.Func + ", passed to " + site.Constructor + " at " +
			at.String() + ", reaches the machine's host-side store accessor. A node reaches shared " +
			"storage through its frame, which is scoped to the datum it is running for; the host " +
			"accessor is the machine's own side and is not a node's to touch. Move the read or the " +
			"write onto the frame, or move the work out of the node",
		Severity: SeverityError,
	})
}

// sortedReaches orders a result's sites so two runs over one package set report
// them identically, whatever order the maps behind go/types handed them back in.
func sortedReaches(sites []HostReach) []HostReach {
	sort.SliceStable(sites, func(i, j int) bool {
		if sites[i].Path != sites[j].Path {
			return sites[i].Path < sites[j].Path
		}

		return sites[i].Pos.Offset < sites[j].Pos.Offset
	})

	return sites
}
