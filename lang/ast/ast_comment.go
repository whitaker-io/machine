// Package ast - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package ast

// Comment is one line comment, carried with both ends of its span.
//
// IT IS TRIVIA, NOT A DECLARATION, and the difference is where it hangs. A
// comment is attached to the File as its own positioned slice rather than as an
// entry in File.Decls, for a reason outside this package: lang/analysis routes
// every Decl it does not recognize to an error-severity diagnostic, so a comment
// in Decls would cost one diagnostic per comment in a module this package is
// versioned separately from. Hanging it off its own field means the analysis
// engine's declaration switch never sees it at all.
//
// IT IS NOT ATTACHED TO A STATEMENT EITHER. Doc-comment semantics — a comment
// that documents the thing below it — is a binding rule about which comment
// belongs to which node, and it is deliberately not part of this form. What is
// carried is the position, which is all a formatter needs to re-emit a comment
// where its author wrote it and all an editor needs to map one.
//
// Text is the body AFTER the marker, verbatim and including its leading space,
// because a formatter that re-emits `// text` from a trimmed body cannot tell an
// author who wrote no space from one who wrote three.
type Comment struct {
	Text  string
	Start Position
	Stop  Position
}

// Pos returns the position of the comment's first marker byte.
func (c Comment) Pos() Position { return c.Start }

// End returns the position just past the comment's last byte, which is the line
// break that ended it or the end of the file.
func (c Comment) End() Position { return c.Stop }

func (Comment) isNode() {}
