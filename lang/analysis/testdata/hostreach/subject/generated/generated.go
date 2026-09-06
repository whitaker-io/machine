// Package generated stands in for the package flowc WRITES.
//
// The package flowc writes is routinely the consumer's OWN package, so a reach
// here is reported like a reach in any other consumer file. An earlier draft
// excluded this package on the reasoning that its node functions are the .flow's
// own; the seam test showed that wrong, and this positive pins the reversal.
package generated

import (
	"context"

	machine "github.com/whitaker-io/machine/v4"
)

// Order is the generated package's own payload type.
type Order struct {
	Amount int
}

// cell is the generated package's heap cell.
var cell = machine.NewCell[int]("counter")

// WireGenerated carries the same violation as the hand-written positives, in the
// package the generator owns.
func WireGenerated(m *machine.Machine) {
	ingest, _ := m.Source[Order]("ingest")
	charged := ingest.Map("charge", func(f machine.Frame[Order]) Order {
		order := f.Value()
		if held, ok, err := m.Host().Load(context.Background(), cell); err == nil && ok {
			order.Amount += held
		}

		return order
	})
	charged.Drop("charge#drain")
}
