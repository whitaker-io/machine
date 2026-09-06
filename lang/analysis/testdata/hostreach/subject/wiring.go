// Package subject is a hand-written CONSUMER of the runtime: the corpus the
// host-reach analyzer reads.
//
// EVERY DECLARATION HERE IS AN ARM, and the four positives are deliberately the
// four shapes a spelling-keyed check gets wrong. Nothing in this file runs; it
// exists to be type-checked and walked. The near misses sit beside the positives
// on purpose — a corpus of violations alone would pass an analyzer that reported
// every function it saw.
package subject

import (
	"context"

	"example.com/hostreach/subject/helpers"
	machine "github.com/whitaker-io/machine/v4"
)

// Order is the payload every flow in this fixture carries.
type Order struct {
	Kind   string
	Amount int
}

// counter is the heap cell both the sanctioned frame path and the unsanctioned
// host path reach, so the two arms differ in HOW they reach it and in nothing
// else.
var counter = machine.NewCell[int]("counter")

// hosted is the machine a top-level node function reaches without capturing one,
// which is the shape a closure-only walk cannot see.
var hosted *machine.Machine

// POSITIVE 1: an inline Transformation closure that CAPTURES the machine and
// calls the host accessor. This is the shape the ticket names.
func WireCapturing(m *machine.Machine) {
	ingest, _ := m.Source[Order]("ingest")
	charged := ingest.Map("charge", func(f machine.Frame[Order]) Order {
		order := f.Value()
		if held, ok, err := m.Host().Load(context.Background(), counter); err == nil && ok {
			order.Amount += held
		}

		return order
	})
	charged.Drop("charge#drain")
}

// POSITIVE 2: a NAMED top-level func passed to the constructor by name. The
// machine is a package-level var rather than a capture, so the reach is invisible
// to anything that walks only the closures written at the call site.
func WireNamed(m *machine.Machine) {
	ingest, _ := m.Source[Order]("ingest")
	enriched := ingest.Map("enrich", EnrichFromHost)
	enriched.Drop("enrich#drain")
}

// EnrichFromHost is a node function only because WireNamed passes it to Map.
func EnrichFromHost(f machine.Frame[Order]) Order {
	order := f.Value()
	if err := hosted.Host().Save(context.Background(), counter, order.Amount); err != nil {
		order.Kind = "failed"
	}

	return order
}

// wiring carries the machine on a struct, so a node function reaches it through
// a FIELD rather than through a bare captured variable.
type wiring struct {
	m *machine.Machine
}

// POSITIVE 3: a Filter closure reaching the accessor through a struct field.
func (w *wiring) Route(m *machine.Machine) {
	ingest, _ := m.Source[Order]("ingest")
	billable, rest := ingest.If("route", func(f machine.Frame[Order]) bool {
		held, ok, err := w.m.Host().Load(context.Background(), counter)

		return err == nil && ok && held < f.Value().Amount
	})
	billable.Drop("billable#drain")
	rest.Drop("rest#drain")
}

// POSITIVE 4: a Tee closure. Duplicator takes the PAYLOAD rather than a Frame, so
// a check that identifies a node function by its Frame parameter misses this one
// entirely; the accessor is reached through an intermediate variable.
func WireTeeing(m *machine.Machine) {
	ingest, _ := m.Source[Order]("ingest")
	left, right := ingest.Tee("split", func(d Order) (Order, Order) {
		host := m.Host()
		if held, ok, err := host.Load(context.Background(), counter); err == nil && ok {
			d.Amount += held
		}

		return d, d
	})
	left.Drop("left#drain")
	right.Drop("right#drain")
}

// NEAR MISS 1: a node function that reaches the SAME heap cell the sanctioned
// way, through the frame. It is a Frame-taking closure passed to the same
// constructor as POSITIVE 1 and differs in one thing only: Frame.Load takes no
// context because the frame carries one, and HostState.Load does.
func WireFramePath(m *machine.Machine) {
	ingest, _ := m.Source[Order]("ingest")
	counted := ingest.Map("count", func(f machine.Frame[Order]) Order {
		order := f.Value()
		if held, ok, err := f.Load(counter); err == nil && ok {
			order.Amount += held
		}
		if err := f.Save(counter, order.Amount); err != nil {
			order.Kind = "failed"
		}
		if _, err := f.Update(counter, func(current int) int { return current + 1 }); err != nil {
			order.Kind = "failed"
		}

		return order
	}, machine.WithReads[Order](counter), machine.WithWrites[Order](counter))
	counted.Drop("count#drain")
}

// NEAR MISS 2: a legitimate host caller, outside every constructor. Its signature
// satisfies no node-function type, so no constructor could accept it; seeding the
// heap before Start is exactly what the accessor is for.
func SeedFromHost(ctx context.Context, m *machine.Machine) error {
	if _, ok, err := m.Host().Load(ctx, counter); err != nil || ok {
		return err
	}

	return m.Host().Save(ctx, counter, 0)
}

// NEAR MISS 3: an ErrorHandler body that reaches the accessor. A handler runs in
// the SUPERVISOR rather than in a node, and it reaches the constructor through an
// option rather than as a node function.
func WireHandled(m *machine.Machine) {
	ingest, _ := m.Source[Order]("ingest")
	handled := ingest.Map("handle", func(f machine.Frame[Order]) Order {
		return f.Value()
	}, machine.WithErrorHandler(func(failure machine.NodeError[Order]) {
		_ = m.Host().Save(context.Background(), counter, failure.Payload.Amount)
	}))
	handled.Drop("handle#drain")
}

// NEAR MISS 4: an EdgeFactory that reaches the accessor. It is a function reaching
// a node constructor, and it is not a node function: it builds the node's inbound
// transport, which is host-side work.
func WireEdged(m *machine.Machine) {
	ingest, _ := m.Source[Order]("ingest", machine.WithEdge(
		func(node string, report machine.Report) (machine.Edge[Order], error) {
			_ = m.Host().Save(context.Background(), counter, 0)

			return nil, nil
		}))
	ingest.Drop("ingest#drain")
}

// capabilityOf is the generated code's own helper shape: it takes a node function
// as an IGNORED parameter purely so Go can infer the payload type. A func value
// passed HERE has not been made a node function by it.
func capabilityOf[U, V any](_ machine.Transformation[U, V], refs ...machine.KeyRef) machine.NodeOption[U] {
	return machine.WithReads[U](refs...)
}

// NEAR MISS 5: a Host-reaching closure passed to that helper and to nothing else.
func WireDeclaringOnly(m *machine.Machine) machine.NodeOption[Order] {
	return capabilityOf(func(f machine.Frame[Order]) Order {
		_ = m.Host().Save(context.Background(), counter, 0)

		return f.Value()
	}, counter)
}

// registry is a local type with its own method named Map, so the constructor
// match cannot be a match on the spelling "Map".
type registry struct{}

// Map is this fixture's own method of that name, on a receiver the runtime knows
// nothing about.
func (registry) Map(name string, fn func(Order) Order) Order { return fn(Order{Kind: name}) }

// NEAR MISS 6: a Host-reaching closure passed to a local Map that is not the
// runtime's.
func WireLocalMap(m *machine.Machine) Order {
	var r registry

	return r.Map("charge", func(d Order) Order {
		_ = m.Host().Save(context.Background(), counter, d.Amount)

		return d
	})
}

// NEAR MISS 7: a node function reached through a package-level VARIABLE. The
// walk resolves a named function to its declaration; a variable holding a
// function value denotes no declaration, so the body is never read and the
// analyzer says so rather than reporting what it did not see.
var leaking machine.Transformation[Order, Order] = func(f machine.Frame[Order]) Order {
	order := f.Value()
	_ = hosted.Host().Save(context.Background(), counter, order.Amount)

	return order
}

// WireThroughAVariable passes that variable to a real constructor.
func WireThroughAVariable(m *machine.Machine) {
	ingest, _ := m.Source[Order]("ingest")
	ingest.Map("charge", leaking).Drop("charge#drain")
}

// pick returns a node function, so the argument at the call site is a CALL
// rather than a literal or a name.
func pick() machine.Transformation[Order, Order] { return leaking }

// NEAR MISS 8: a node function reached through a call. Same disclosure, second
// shape.
func WireThroughACall(m *machine.Machine) {
	ingest, _ := m.Source[Order]("ingest")
	ingest.Map("charge", pick()).Drop("charge#drain")
}

// gateway is a local type with its own zero-argument method named Host, which is
// the accessor's spelling on a type the runtime knows nothing about.
type gateway struct{}

// Host is that method.
func (gateway) Host() string { return "gateway" }

// NEAR MISS 9: a node function calling a zero-argument Host() that is not the
// machine's. A check keyed on the SPELLING reports this one.
func WireLocalHost(m *machine.Machine) {
	var g gateway
	ingest, _ := m.Source[Order]("ingest")
	ingest.Map("charge", func(f machine.Frame[Order]) Order {
		order := f.Value()
		order.Kind = g.Host()

		return order
	}).Drop("charge#drain")
}

// box is a generic type whose method wires a node function, so the walk names a
// closure after a receiver written with a type argument.
type box[T any] struct {
	m *machine.Machine
}

// POSITIVE 7: a generic receiver's method wiring a node function that reaches
// the accessor.
func (b *box[T]) Wire() {
	ingest, _ := b.m.Source[Order]("ingest")
	ingest.Map("charge", func(f machine.Frame[Order]) Order {
		order := f.Value()
		if held, ok, err := b.m.Host().Load(context.Background(), counter); err == nil && ok {
			order.Amount += held
		}

		return order
	}).Drop("charge#drain")
}

// POSITIVE 8: a node function declared in ANOTHER PACKAGE and passed by its
// qualified name. The reach is reported at the file that declares the body, not
// at the file that wires it.
func WireFromAnotherPackage(m *machine.Machine) {
	ingest, _ := m.Source[helpers.Order]("ingest")
	ingest.Map("enrich", helpers.EnrichFromHost).Drop("enrich#drain")
}

// hostField is a struct with a FUNC-TYPED FIELD named Host, which is the shape a
// spelling-keyed check cannot separate from the accessor.
type hostField struct {
	Host func() string
}

// NEAR MISS 10: a node function calling a zero-argument field named Host.
func WireHostField(m *machine.Machine) {
	var h hostField
	ingest, _ := m.Source[Order]("ingest")
	ingest.Map("charge", func(f machine.Frame[Order]) Order {
		order := f.Value()
		order.Kind = h.Host()

		return order
	}).Drop("charge#drain")
}
