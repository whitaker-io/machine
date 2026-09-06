// Package subject's ALIASED-IMPORT arm: the same violation written through an
// import qualifier that is not the package's own name.
//
// Imports are per-file, so this file spells the runtime `mx` while wiring.go
// spells it `machine`. A check keyed on the identifier at the call site sees two
// different programs; a check that resolves the callee through go/types sees one.
package subject

import (
	"context"

	mx "github.com/whitaker-io/machine/v4"
)

// POSITIVE 5: a node function reaching the accessor, written entirely through an
// aliased qualifier.
func WireAliased(m *mx.Machine) {
	ingest, _ := m.Source[Order]("ingest")
	charged := ingest.Map("charge", func(f mx.Frame[Order]) Order {
		order := f.Value()
		if held, ok, err := m.Host().Load(context.Background(), counter); err == nil && ok {
			order.Amount += held
		}

		return order
	})
	charged.Drop("charge#drain")
}
