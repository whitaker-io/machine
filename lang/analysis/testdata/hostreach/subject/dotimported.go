// Package subject's DOT-IMPORT arm: the same violation written with no qualifier
// at all.
//
// A dot import puts the runtime's own names into this file's scope, so the
// constructor call and the accessor call carry no package qualifier for a text
// scan to key on. go/types resolves both to the same objects the qualified
// spellings resolve to.
package subject

import (
	"context"

	. "github.com/whitaker-io/machine/v4"
)

// POSITIVE 6: a node function reaching the accessor with every runtime name
// unqualified.
func WireDotImported(m *Machine) {
	ingest, _ := m.Source[Order]("ingest")
	charged := ingest.Map("charge", func(f Frame[Order]) Order {
		order := f.Value()
		if held, ok, err := m.Host().Load(context.Background(), counter); err == nil && ok {
			order.Amount += held
		}

		return order
	})
	charged.Drop("charge#drain")
}
