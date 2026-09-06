// Package helpers declares a node function in a DIFFERENT package from the
// wiring that passes it to a constructor.
//
// It is the cross-package arm: the walk has to resolve the qualified name to its
// object, find the package that declares it, and read a body in a file it is not
// currently walking.
package helpers

import (
	"context"

	machine "github.com/whitaker-io/machine/v4"
)

// Order is this package's own payload type.
type Order struct {
	Amount int
}

// Counter is the heap cell the reach below goes through.
var Counter = machine.NewCell[int]("counter")

// Hosted is the machine the node function reaches without capturing one.
var Hosted *machine.Machine

// EnrichFromHost is a node function whenever a caller passes it to a builder.
func EnrichFromHost(f machine.Frame[Order]) Order {
	order := f.Value()
	if held, ok, err := Hosted.Host().Load(context.Background(), Counter); err == nil && ok {
		order.Amount += held
	}

	return order
}
