// Package generated stands in for the package flowc WRITES.
//
// Its node functions are the .flow's own, lifted verbatim by the emitter, so a
// reach here is a reach in a .flow file the author can edit — which is the
// corpus the .flow-resident host-accessor check already reads. Reporting it here
// as well would point an author at a file that is rewritten on every generation.
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
