// Package lint - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

// Package lint is the batch face of the flow analysis core: it resolves path
// arguments to parsed sources, runs the registered analyzers over them, and
// renders the result for a person or for a machine.
//
// IT REIMPLEMENTS NO ANALYSIS. Every diagnostic this package emits was minted by
// lang/analysis — by an analyzer through analysis.Run, or by
// analysis.ParseDiagnostics out of a parse failure. This package supplies a
// threshold comparison, a merge and two renderers, and nothing else. A rule that
// belongs to the language belongs in the analysis core, where the editor and the
// batch tool both reach it.
//
// IT DOES NOT CLAIM MORE THAN THE ANALYZERS DO. Seven of the thirteen registered
// analyzers state a limit in their own Doc, and a clean run is bounded by every
// one of them: typeflow compares declared spellings and IS NOT TYPE CHECKING,
// resolving nothing through go/types; state's bare-type check is a denylist of
// two retired spellings rather than a proof about the rest; switches mandates an
// else precisely because v1 cannot prove a switch is covered; resolve ships no
// unimported-qualifier check; signature says nothing about a use it cannot
// resolve; errorrouting's unhandled-failure finding reports a shape that is
// legal and may well be deliberate; and hostaccess reads .flow-resident func
// bodies only, so its silence is not a proof that no node reaches the host. That
// is why a clean run reports that no registered rule fired rather than that no
// problems exist, and why the -rules listing prints each analyzer's own Doc
// verbatim instead of summarizing it.
//
// THAT CENSUS IS A DIFFERENT POPULATION FROM THE ANALYSIS MODULE'S REQUIRED-
// DISCLOSURE GATE, deliberately. The gate mandates a disclosure wherever a
// consumer could over-read a result, which covers flowgraph's statement that its
// send model is one of two defensible readings — a modeling CHOICE rather than a
// bound on what a clean run proves, so it is not named above. Neither list is
// prose: this one's numerals and names are gated by this package's own census
// test, and an analyzer the analysis module mandates a disclosure for must be
// either named above or recorded there as not a limit, which is what stops the
// next analyzer from landing outside both lists.
//
// There is no rule selection, no configuration file and no suppression syntax.
// Each is a lever for making a finding disappear without fixing it. The one knob
// is the fail threshold, and it can only be made stricter than its default.
package lint
