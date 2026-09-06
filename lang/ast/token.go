// Package ast - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package ast

// tokenKind identifies what a token is. The set is closed: the lexer emits
// nothing outside it, and the parser switches over it exhaustively.
type tokenKind int

// The structural token kinds. tokGoSpan and tokGoFuncSpan carry verbatim Go
// text the flow language never inspects; tokNoteText carries a raw note body.
const (
	tokEOF tokenKind = iota
	tokNewline
	tokIdent
	tokString
	tokNumber
	tokArrow
	tokComma
	tokDot
	tokAssign
	tokLBrace
	tokRBrace
	tokLParen
	tokRParen
	tokNoteText
	tokGoSpan
	tokGoFuncSpan
	tokIllegal
)

// The keyword token kinds, one per ruled spelling. The inventory is closed at 27;
// keywords below is the authoritative table and keyword_census_test.go states the
// same list a second time so a divergence in either direction fails.
//
// THE 27 IN THIS SENTENCE IS NOT GATED, and a reader deserves to know which
// numerals here are checked and which are only written down. The spelled-out
// numeral census reads exactly two anchor declarations — `var spanStopKeywords`
// and `var clauseStarters` — and names this file among the derived statements it
// deliberately does not read (keyword_numeral_census_test.go, at its boundary
// note). What IS gated is the SET rather than the count: len(keywords) is
// asserted against the second list by keyword_census_test.go, so a keyword added
// without its census row fails there. The count in this prose is a courtesy to a
// reader and drifts silently; it read 26 against a measured 27 until the line
// comment landed and this file was touched.
const (
	kwFlow tokenKind = iota + 100
	kwNote
	kwImport
	kwState
	kwVar
	kwConst
	kwParam
	kwOn
	kwSource
	kwTransform
	kwBranch
	kwSwitch
	kwTee
	kwSink
	kwDrop
	kwLoop
	kwSend
	kwUse
	kwFrom
	kwOver
	kwReads
	kwWrites
	kwClone
	kwElse
	kwCheckpoint
	kwIdempotent
	kwFunc
)

// The RESERVED spellings the Go span scanner stops at, named because two tables
// state them: the keyword inventory below, and the scanner's stop set.
//
// Seven of them are the CLAUSE keywords, which end a span because a clause follows
// it. `else` is the eighth and is there for a different reason: reserving it
// makes an else arm unmatchable as an ordinary switch arm, which is what lets
// the grammar's trailing option express that an else must come last. The
// distinction between these eight and "any keyword" is still load bearing —
// `func` in particular must NOT join them.
const (
	textReads      = "reads"
	textWrites     = "writes"
	textOver       = "over"
	textCheckpoint = "checkpoint"
	textIdempotent = "idempotent"
	textOn         = "on"
	textNote       = "note"
	textElse       = "else"
)

// keywords is THE authoritative keyword inventory. The lexer, the grammar
// coverage gate and the tokenization record all read it; nothing else restates
// the list except the census test, whose whole job is to be the second,
// independent statement of it.
//
// `error` is deliberately absent. It appears in the grammar only as a quoted
// terminal of the `on error` clause and is not a keyword; adding it here is the
// natural mistake the census test refuses.
var keywords = map[string]tokenKind{
	"flow":         kwFlow,
	textNote:       kwNote,
	"import":       kwImport,
	"state":        kwState,
	"var":          kwVar,
	"const":        kwConst,
	"param":        kwParam,
	textOn:         kwOn,
	"source":       kwSource,
	"transform":    kwTransform,
	"branch":       kwBranch,
	"switch":       kwSwitch,
	"tee":          kwTee,
	"sink":         kwSink,
	"drop":         kwDrop,
	"loop":         kwLoop,
	"send":         kwSend,
	"use":          kwUse,
	"from":         kwFrom,
	textOver:       kwOver,
	textReads:      kwReads,
	textWrites:     kwWrites,
	"clone":        kwClone,
	textElse:       kwElse,
	textCheckpoint: kwCheckpoint,
	textIdempotent: kwIdempotent,
	"func":         kwFunc,
}

// token is one lexeme: what it is, the source text it covers, and where it
// starts.
type token struct {
	kind tokenKind
	text string
	pos  Position
}
