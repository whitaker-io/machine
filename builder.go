// Package machine - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.
package machine

import (
	"context"
	"fmt"
	"sort"
)

// Stream is a representation of a data stream and its associated logic.
// Creating a new Stream is handled by the appropriately named NewStream function.
//
// The Builder method is the entrypoint into creating the data processing flow.
// All branches of the Stream are required to end in either a Publish or
// a Link in order to be considered valid.
type Stream[T Identifiable] interface {
	ID() string
	Run(ctx context.Context) error
	Inject(id string, payload ...T)
	VertexIDs() []string
	Builder() Builder[T]
}

// Builder is the interface provided for creating a data processing stream.
type Builder[T Identifiable] interface {
	Map(id string, a Applicative[T]) Builder[T]
	Sort(id string, x Comparator[T]) Builder[T]
	Remove(id string, x Remover[T]) Builder[T]
	FoldLeft(id string, f Fold[T]) Builder[T]
	FoldRight(id string, f Fold[T]) Builder[T]
	Duplicate(id string) (Builder[T], Builder[T])
	Filter(id string, f Filter[T]) (Builder[T], Builder[T])
	Loop(id string, x Filter[T]) (loop, out Builder[T])
	Channel() chan []T
	Finally(id string, a Applicative[T]) chan []T
}

type nexter[T Identifiable] func(*node[T]) *node[T]

type builder[T Identifiable] struct {
	vertex[T]
	next   *node[T]
	option *Option[T]
	edges  map[string]Edge[T]
}

type node[T Identifiable] struct {
	vertex[T]
	edge  Edge[T]
	loop  *node[T]
	next  *node[T]
	left  *node[T]
	right *node[T]
}

// ID is a method used to return the ID for the Stream
func (m *builder[T]) ID() string {
	return m.id
}

// Run is the method used for starting the stream processing. It requires a context
// and an optional list of recorder functions. The recorder function has the signiture
// func(vertexID, vertexType, state string, paylaod []*Packet) and is called at the
// beginning of every vertex.
func (m *builder[T]) Run(ctx context.Context) error {
	if m.next == nil {
		return fmt.Errorf("non-terminated builder %s", m.id)
	}

	return m.cascade(ctx, m, &edge[T]{})
}

// Inject is a method for restarting work that has been dropped by the Stream
// typically in a distributed system setting. Though it can be used to side load
// data into the Stream to be processed
func (m *builder[T]) Inject(id string, payload ...T) {
	m.edges[id].Input(payload...)
}

func (m *builder[T]) VertexIDs() []string {
	ids := []string{}

	for name := range m.edges {
		if name != "__channel__" {
			ids = append(ids, name)
		}
	}

	return ids
}

func (m *builder[T]) Builder() Builder[T] {
	return nexter[T](func(n *node[T]) *node[T] {
		m.next = n
		return n
	})
}

func (n *node[T]) closeLoop() {
	if n.loop != nil && n.next == nil {
		n.next = n.loop
	}
}

// Map apply a mutation
func (n nexter[T]) Map(id string, x Applicative[T]) Builder[T] {
	next := &node[T]{}

	next.vertex = vertex[T]{
		id:         id,
		vertexType: "map",
		handler: func(payload []T) {
			for _, packet := range payload {
				packet = x(packet)
			}

			next.edge.Input(payload...)
		},
		connector: newConnector(id, next),
	}

	next = n(next)

	return nextBuilder(next)
}

// Sort modifies the order of the T based on the Comparator
func (n nexter[T]) Sort(id string, x Comparator[T]) Builder[T] {
	next := &node[T]{}

	next.vertex = vertex[T]{
		id:         id,
		vertexType: "sort",
		handler: func(payload []T) {
			sort.Slice(payload, func(i, j int) bool {
				return x(payload[i], payload[j]) < 0
			})

			next.edge.Input(payload...)
		},
		connector: newConnector(id, next),
	}

	next = n(next)

	return nextBuilder(next)
}

// Remove data from the payload based on the Remover func
func (n nexter[T]) Remove(id string, x Remover[T]) Builder[T] {
	next := &node[T]{}

	next.vertex = vertex[T]{
		id:         id,
		vertexType: "sort",
		handler: func(payload []T) {
			output := []T{}

			for i, v := range payload {
				if !x(i, v) {
					output = append(output, v)
				}
			}

			next.edge.Input(output...)
		},
		connector: newConnector(id, next),
	}

	next = n(next)

	return nextBuilder(next)
}

// FoldLeft the data
func (n nexter[T]) FoldLeft(id string, x Fold[T]) Builder[T] {
	next := &node[T]{}

	fr := func(payload ...T) T {
		if len(payload) == 1 {
			return payload[0]
		}

		d := payload[0]

		for i := 1; i < len(payload); i++ {
			d = x(d, payload[i])
		}

		return d
	}

	next.vertex = vertex[T]{
		id:         id,
		vertexType: "fold",
		handler: func(payload []T) {
			next.edge.Input(fr(payload...))
		},
		connector: newConnector(id, next),
	}
	next = n(next)

	return nextBuilder(next)
}

// FoldRight the data
func (n nexter[T]) FoldRight(id string, x Fold[T]) Builder[T] {
	next := &node[T]{}

	fr := func(payload ...T) T {
		if len(payload) == 1 {
			return payload[0]
		}

		d := payload[len(payload)-1]

		for i := len(payload) - 2; i > -1; i-- {
			d = x(d, payload[i])
		}

		return d
	}

	next.vertex = vertex[T]{
		id:         id,
		vertexType: "fold",
		handler: func(payload []T) {
			next.edge.Input(fr(payload...))
		},
		connector: newConnector(id, next),
	}

	next = n(next)

	return nextBuilder(next)
}

// ForkDuplicate is a Fork that creates a deep copy of the
// payload and sends it down both branches.
func (n nexter[T]) Duplicate(id string) (left, right Builder[T]) {
	return n.fork(id, func(payload []T) (l, r []T) {
		return payload, deepcopy(payload)
	})
}

// Filter the data
func (n nexter[T]) Filter(id string, x Filter[T]) (left, right Builder[T]) {
	return n.fork(id, func(payload []T) (l, r []T) {
		l = []T{}
		r = []T{}

		for _, packet := range payload {
			if x(packet) {
				l = append(l, packet)
			} else {
				r = append(r, packet)
			}
		}

		return
	})
}

// Filter the data
func (n nexter[T]) fork(id string, x func([]T) (l, r []T)) (left, right Builder[T]) {
	next := &node[T]{}

	var leftEdge, rightEdge Edge[T]

	next.vertex = vertex[T]{
		id:         id,
		vertexType: "fork",
		handler: func(payload []T) {
			s, f := x(payload)

			leftEdge.Input(s...)
			rightEdge.Input(f...)
		},
		connector: func(ctx context.Context, b *builder[T]) error {
			leftEdge = b.option.Provider.New(ctx, id, b.option)
			rightEdge = b.option.Provider.New(ctx, id, b.option)

			if next.loop != nil && next.left == nil {
				next.left = next.loop
			}

			if next.loop != nil && next.right == nil {
				next.right = next.loop
			}

			if next.left == nil || next.right == nil {
				return fmt.Errorf("non-terminated fork %s", id)
			} else if err := next.left.cascade(ctx, b, leftEdge); err != nil {
				return err
			} else if err := next.right.cascade(ctx, b, rightEdge); err != nil {
				return err
			}

			return nil
		},
	}

	next = n(next)

	return nextLeft(next, next.loop), nextRight(next, next.loop)
}

// Loop the data combining a fork and link the first output is the Builder for the loop
// and the second is the output of the loop
func (n nexter[T]) Loop(id string, x Filter[T]) (loop, out Builder[T]) {
	next := &node[T]{}

	var leftEdge, rightEdge Edge[T]

	next.vertex = vertex[T]{
		id:         id,
		vertexType: "loop",
		handler: func(payload []T) {
			l := []T{}
			r := []T{}

			for _, packet := range payload {
				if x(packet) {
					l = append(l, packet)
				} else {
					r = append(r, packet)
				}
			}
			leftEdge.Input(l...)
			rightEdge.Input(r...)
		},
		connector: func(ctx context.Context, b *builder[T]) error {
			leftEdge = b.option.Provider.New(ctx, id, b.option)
			rightEdge = b.option.Provider.New(ctx, id, b.option)

			if next.loop != nil && next.right == nil {
				next.right = next.loop
			}

			if next.left == nil || next.right == nil {
				return fmt.Errorf("non-terminated loop %s", id)
			} else if err := next.left.cascade(ctx, b, leftEdge); err != nil {
				return err
			} else if err := next.right.cascade(ctx, b, rightEdge); err != nil {
				return err
			}

			return nil
		},
	}

	next = n(next)

	return nextLeft(next, next), nextRight(next, next)
}

// Publish the data outside the system
func (n nexter[T]) Channel() chan []T {
	channel := make(chan []T)
	v := vertex[T]{
		id:         "__channel__",
		vertexType: "channel",
		connector:  func(ctx context.Context, b *builder[T]) error { return nil },
		handler: func(payload []T) {
			channel <- payload
		},
	}

	n(&node[T]{vertex: v})

	return channel
}

// Finally caps the Stream with an Applicative
func (n nexter[T]) Finally(id string, x Applicative[T]) chan []T {
	channel := make(chan []T)
	v := vertex[T]{
		id:         id,
		vertexType: "map",
		handler: func(payload []T) {
			for _, packet := range payload {
				packet = x(packet)
			}

			channel <- payload
		},
		connector: func(ctx context.Context, b *builder[T]) error { return nil },
	}

	n(&node[T]{vertex: v})

	return channel
}

// New is a function for creating a new Stream. It takes an id, a Retriever function,
// and a list of Options that can override the defaults and set new defaults for the
// subsequent vertices in the Stream.
func New[T Identifiable](id string, retriever Retriever[T], options ...*Option[T]) Stream[T] {
	opt := defaultOptions[T]().merge(options...)

	var edge Edge[T]

	x := &builder[T]{
		option: opt,
		vertex: vertex[T]{
			id:         id,
			vertexType: "stream",
			handler: func(p []T) {
				edge.Input(p...)
			},
		},
		edges: map[string]Edge[T]{},
	}

	x.connector = func(ctx context.Context, b *builder[T]) error {
		edge = opt.Provider.New(ctx, id, opt)

		i := retriever(ctx)

		go func() {
		Loop:
			for {
				select {
				case <-ctx.Done():
					break Loop
				case data := <-i:
					if len(data) < 1 {
						continue
					}

					x.input <- data
				}
			}
		}()
		return x.next.cascade(ctx, x, edge)
	}

	return x
}

func newConnector[T Identifiable](id string, next *node[T]) func(ctx context.Context, b *builder[T]) error {
	return func(ctx context.Context, b *builder[T]) error {
		next.edge = b.option.Provider.New(ctx, id, b.option)

		next.closeLoop()

		if next.next == nil {
			return fmt.Errorf("non-terminated vertex %s", id)
		}
		return next.next.cascade(ctx, b, next.edge)
	}
}

func nextBuilder[T Identifiable](next *node[T]) Builder[T] {
	return nexter[T](func(n *node[T]) *node[T] {
		n.loop = next
		next.next = n
		return n
	})
}

func nextLeft[T Identifiable](next, loop *node[T]) Builder[T] {
	return nexter[T](func(n *node[T]) *node[T] {
		n.loop = loop
		next.left = n
		return n
	})
}

func nextRight[T Identifiable](next, loop *node[T]) Builder[T] {
	return nexter[T](func(n *node[T]) *node[T] {
		n.loop = loop
		next.right = n
		return n
	})
}
