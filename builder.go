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
	Consume(ctx context.Context, input chan []T) error
	Builder() Builder[T]
}

// Builder is the interface provided for creating a data processing stream.
type Builder[T Identifiable] interface {
	Map(a Applicative[T]) Builder[T]
	Window(x Window[T]) Builder[T]
	Sort(x Comparator[T]) Builder[T]
	Remove(x Remover[T]) Builder[T]
	FoldLeft(f Fold[T]) Builder[T]
	FoldRight(f Fold[T]) Builder[T]
	Duplicate() (Builder[T], Builder[T])
	Filter(f Filter[T]) (Builder[T], Builder[T])
	Loop(x Filter[T]) (loop, out Builder[T])
	Channel() chan []T
}

type builder[T Identifiable] func(*node[T]) *node[T]

type stream[T Identifiable] struct {
	node[T]
	option *Option[T]
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
func (n *stream[T]) ID() string {
	return n.id
}

// Run is the method used for starting the stream processing. It requires a context
// and an optional list of recorder functions. The recorder function has the signiture
// func(vertexID, vertexType, state string, paylaod []*Packet) and is called at the
// beginning of every vertex.
func (n *stream[T]) Consume(ctx context.Context, input chan []T) error {
	if n.next == nil {
		return fmt.Errorf("non-terminated builder %s", n.id)
	}

	e := &edge[T]{input}

	if n.input != nil {
		e.OutputTo(ctx, n.input)
		return nil
	}

	return n.cascade(ctx, n.id, n.id, n.option, e)
}

func (n *stream[T]) Builder() Builder[T] {
	return builder[T](func(x *node[T]) *node[T] {
		n.next = x
		return x
	})
}

// Map apply a mutation
func (n builder[T]) Map(x Applicative[T]) Builder[T] {
	next := &node[T]{}

	next.vertex = vertex[T]{
		vertexType: "map",
		handler: func(payload []T) {
			for i, packet := range payload {
				payload[i] = x(packet)
			}

			next.edge.Input(payload...)
		},
		connector: newConnector(next),
	}

	next = n(next)

	return nextBuilder(next)
}

// Map apply a mutation on the entire payload
func (n builder[T]) Window(x Window[T]) Builder[T] {
	next := &node[T]{}

	next.vertex = vertex[T]{
		vertexType: "window",
		handler: func(payload []T) {
			next.edge.Input(x(payload)...)
		},
		connector: newConnector(next),
	}

	next = n(next)

	return nextBuilder(next)
}

// Sort modifies the order of the T based on the Comparator
func (n builder[T]) Sort(x Comparator[T]) Builder[T] {
	next := &node[T]{}

	next.vertex = vertex[T]{
		vertexType: "sort",
		handler: func(payload []T) {
			sort.Slice(payload, func(i, j int) bool {
				return x(payload[i], payload[j]) < 0
			})

			next.edge.Input(payload...)
		},
		connector: newConnector(next),
	}

	next = n(next)

	return nextBuilder(next)
}

// Remove data from the payload based on the Remover func
func (n builder[T]) Remove(x Remover[T]) Builder[T] {
	next := &node[T]{}

	next.vertex = vertex[T]{
		vertexType: "remove",
		handler: func(payload []T) {
			output := []T{}

			for i, v := range payload {
				if !x(i, v) {
					output = append(output, v)
				}
			}

			next.edge.Input(output...)
		},
		connector: newConnector(next),
	}

	next = n(next)

	return nextBuilder(next)
}

// FoldLeft the data
func (n builder[T]) FoldLeft(x Fold[T]) Builder[T] {
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
		vertexType: "lfold",
		handler: func(payload []T) {
			next.edge.Input(fr(payload...))
		},
		connector: newConnector(next),
	}
	next = n(next)

	return nextBuilder(next)
}

// FoldRight the data
func (n builder[T]) FoldRight(x Fold[T]) Builder[T] {
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
		vertexType: "rfold",
		handler: func(payload []T) {
			next.edge.Input(fr(payload...))
		},
		connector: newConnector(next),
	}

	next = n(next)

	return nextBuilder(next)
}

// ForkDuplicate is a Fork that creates a deep copy of the
// payload and sends it down both branches.
func (n builder[T]) Duplicate() (left, right Builder[T]) {
	return n.fork("duplicate", func(payload []T) (l, r []T) {
		return payload, deepcopy(payload)
	})
}

// Filter the data
func (n builder[T]) Filter(x Filter[T]) (left, right Builder[T]) {
	return n.fork("filter", func(payload []T) (l, r []T) {
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
func (n builder[T]) fork(kind string, x func([]T) (l, r []T)) (left, right Builder[T]) {
	next := &node[T]{}

	var leftEdge, rightEdge Edge[T]

	next.vertex = vertex[T]{
		vertexType: "fork",
		handler: func(payload []T) {
			s, f := x(payload)

			leftEdge.Input(s...)
			rightEdge.Input(f...)
		},
		connector: func(ctx context.Context, streamID, currentID string, option *Option[T]) error {
			next.vertex.id = currentID + "-" + kind

			if next.loop != nil && next.left == nil {
				next.left = next.loop
			}

			if next.left == nil || next.right == nil {
				return fmt.Errorf("non-terminated fork %s", next.id)
			}

			next.left.id = next.vertex.id + "-left-" + next.left.vertexType
			next.right.id = next.vertex.id + "-right-" + next.right.vertexType

			leftEdge = option.Provider.New(ctx, next.left.id, option)
			rightEdge = option.Provider.New(ctx, next.right.id, option)

			if err := next.left.cascade(ctx, streamID, next.left.id, option, leftEdge); err != nil {
				return err
			} else if err := next.right.cascade(ctx, streamID, next.right.id, option, rightEdge); err != nil {
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
func (n builder[T]) Loop(x Filter[T]) (loop, out Builder[T]) {
	next := &node[T]{}

	var leftEdge, rightEdge Edge[T]

	next.vertex = vertex[T]{
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
		connector: func(ctx context.Context, streamID, currentID string, option *Option[T]) error {
			next.vertex.id = currentID + "-loop"

			if next.loop != nil && next.right == nil {
				next.right = next.loop
			}

			if next.left == nil || next.right == nil {
				return fmt.Errorf("non-terminated loop %s", next.id)
			}

			next.left.id = next.vertex.id + "-in-" + next.left.vertexType
			next.right.id = next.vertex.id + "-out-" + next.right.vertexType

			leftEdge = option.Provider.New(ctx, next.left.id, option)
			rightEdge = option.Provider.New(ctx, next.right.id, option)

			if err := next.left.cascade(ctx, streamID, next.left.id, option, leftEdge); err != nil {
				return err
			} else if err := next.right.cascade(ctx, streamID, next.right.id, option, rightEdge); err != nil {
				return err
			}

			return nil
		},
	}

	next = n(next)

	return nextLeft(next, next), nextRight(next, next)
}

// Publish the data outside the system
func (n builder[T]) Channel() chan []T {
	channel := make(chan []T)

	var v vertex[T]
	v = vertex[T]{
		vertexType: "channel",
		connector: func(ctx context.Context, streamID, currentID string, option *Option[T]) error {
			v.id = currentID + "-channel"
			return nil
		},
		handler: func(payload []T) {
			channel <- payload
		},
	}

	n(&node[T]{vertex: v})

	return channel
}

func (n *node[T]) closeLoop() {
	if n.loop != nil && n.next == nil {
		n.next = n.loop
	}
}

// New is a function for creating a new Stream. It takes an a Retriever function,
// and a list of Options that can override the defaults and set new defaults for the
// subsequent vertices in the Stream.
func New[T Identifiable](name string, options ...*Option[T]) Stream[T] {
	x := &stream[T]{
		option: defaultOptions[T]().merge(options...),
		node: node[T]{
			vertex: vertex[T]{
				id:         name,
				vertexType: "stream",
			},
		},
	}

	x.handler = func(payload []T) {
		x.next.edge.Input(payload...)
	}
	x.connector = newConnector(&x.node)

	return x
}

func newConnector[T Identifiable](n *node[T]) func(ctx context.Context, streamID, currentID string, option *Option[T]) error {
	return func(ctx context.Context, streamID, currentID string, option *Option[T]) error {
		if n.vertex.id == "" {
			n.vertex.id = currentID + "-" + n.vertex.vertexType
		}

		n.edge = option.Provider.New(ctx, n.id, option)

		n.closeLoop()

		if n.next == nil {
			return fmt.Errorf("non-terminated vertex %s", n.id)
		}
		return n.next.cascade(ctx, streamID, n.vertex.id, option, n.edge)
	}
}

func nextBuilder[T Identifiable](next *node[T]) Builder[T] {
	return builder[T](func(n *node[T]) *node[T] {
		n.loop = next
		next.next = n
		return n
	})
}

func nextLeft[T Identifiable](next, loop *node[T]) Builder[T] {
	return builder[T](func(n *node[T]) *node[T] {
		n.loop = loop
		next.left = n
		return n
	})
}

func nextRight[T Identifiable](next, loop *node[T]) Builder[T] {
	return builder[T](func(n *node[T]) *node[T] {
		n.loop = loop
		next.right = n
		return n
	})
}
