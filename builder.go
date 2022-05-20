// Package machine - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.
package machine

import (
	"context"
	"fmt"
)

const (
	nodeNext  nodeChild = "next"
	nodeLeft  nodeChild = "left"
	nodeRight nodeChild = "right"
	nodeLoop  nodeChild = "loop"
)

type nodeChild string

// Stream is a representation of a data stream and its associated logic.
//
// The Builder method is the entrypoint into creating the data processing flow.
// All branches of the Stream are required to end in an OutputTo call.
type Stream[T Identifiable] interface {
	Start(ctx context.Context) error
	StartWith(ctx context.Context, input ...chan []T) error
	Consume(input ...chan []T)
	Clear()
	Builder() Builder[T]
}

// Builder is the interface provided for creating a data processing stream.
type Builder[T Identifiable] interface {
	Map(a Applicative[T]) Builder[T]
	Window(x Window[T]) Builder[T]
	Sort(x Comparator[T]) Builder[T]
	Remove(x Remover[T]) Builder[T]
	Combine(x Combiner[T]) Builder[T]
	Filter(f Filter[T]) (Builder[T], Builder[T])
	Loop(x Filter[T]) (loop, out Builder[T])
	OutputTo(x chan []T)
}

type builder[T Identifiable] struct {
	s    *stream[T]
	n    *node[T]
	wrap func(*node[T])
}

type stream[T Identifiable] struct {
	name        string
	n           *node[T]
	consumables []Edge[T]
	provider    EdgeProvider[T]
	option      *Option[T]
}

type node[T Identifiable] struct {
	name string
	Vertex[T]
	run   func(ctx context.Context, channel chan []T, option *Option[T]) error
	nodes map[nodeChild]*node[T]
}

func (x *stream[T]) Start(ctx context.Context) error {
	if x.n == nil {
		return fmt.Errorf("non-terminated builder %s", x.name)
	}

	channel := make(chan []T, x.option.BufferSize)

	for _, c := range x.consumables {
		c.OutputTo(ctx, channel)
	}

	return x.n.run(ctx, channel, x.option)
}

func (x *stream[T]) StartWith(ctx context.Context, input ...chan []T) error {
	x.Consume(input...)
	return x.Start(ctx)
}

func (x *stream[T]) Consume(input ...chan []T) {
	for _, c := range input {
		x.consumables = append(x.consumables, AsEdge(c))
	}
}

func (x *stream[T]) Clear() {
	x.consumables = []Edge[T]{}
}

func (x *stream[T]) Builder() Builder[T] {
	return &builder[T]{
		s: x,
		n: &node[T]{
			name:  x.name,
			nodes: map[nodeChild]*node[T]{},
		},
		wrap: func(n *node[T]) {
			x.n = n
		},
	}
}

// Map apply a mutation to each individual element of the payload.
func (x builder[T]) Map(fn Applicative[T]) Builder[T] {
	return x.component("map", fn)
}

// Window apply a mutation on the entire payload at once.
func (x builder[T]) Window(fn Window[T]) Builder[T] {
	return x.component("window", fn)
}

// Sort modifies the order of the payload based on the Comparator
func (x builder[T]) Sort(fn Comparator[T]) Builder[T] {
	return x.component("sort", fn)
}

// Remove data from the payload based on the Remover func
func (x builder[T]) Remove(fn Remover[T]) Builder[T] {
	return x.component("remove", fn)
}

// Combine the data into a singleton payload
func (x builder[T]) Combine(fn Combiner[T]) Builder[T] {
	return x.component("combine", fn)
}

// Filter splits the data into multiple stream branches
func (x builder[T]) Filter(fn Filter[T]) (left, right Builder[T]) {
	return x.filterComponent("filter", fn, false)
}

// Loop creates a loop in the stream based on the filter
func (x builder[T]) Loop(fn Filter[T]) (loop, out Builder[T]) {
	return x.filterComponent("loop", fn, true)
}

// OutputTo caps the builder and sends the output to the provided channel
func (x builder[T]) OutputTo(channel chan []T) {
	this := &node[T]{
		name: x.n.name + ":output",
		Vertex: Vertex[T](func(payload []T) {
			channel <- payload
		}),
		nodes: map[nodeChild]*node[T]{},
	}

	this.run = func(ctx context.Context, channel chan []T, option *Option[T]) error {
		this.Run(ctx, this.name, channel, option)
		return nil
	}

	x.wrap(this)
}

func (x *builder[T]) component(typeName string, fn Component[T]) Builder[T] {
	name := x.n.name + ":" + typeName
	e := x.s.provider.New(name, x.s.option)

	this := &node[T]{
		name:   name,
		Vertex: fn.Component(e),
		nodes:  map[nodeChild]*node[T]{},
	}

	setup := this.setup(e, nodeNext)

	this.run = func(ctx context.Context, channel chan []T, option *Option[T]) error {
		if err := setup(ctx, channel, option); err != nil {
			return err
		}

		this.Run(ctx, name, channel, option)

		return nil
	}

	x.wrap(this)

	return x.new(this, e, nodeNext, false)
}

func (x *builder[T]) filterComponent(typeName string, fn Filter[T], loop bool) (Builder[T], Builder[T]) {
	name := x.n.name + ":loop"
	l := x.s.provider.New(name+":left", x.s.option)
	r := x.s.provider.New(name+":right", x.s.option)

	this := &node[T]{
		name:   name,
		Vertex: fn.Component(l, r, x.s.option),
		nodes:  map[nodeChild]*node[T]{},
	}

	setupLeft := this.setup(l, nodeLeft)
	setupRight := this.setup(r, nodeRight)

	alreadySetup := false
	c := make(chan []T, x.s.option.BufferSize)

	this.run = func(ctx context.Context, channel chan []T, option *Option[T]) error {
		(&edge[T]{channel}).OutputTo(ctx, c)

		if alreadySetup {
			return nil
		}

		alreadySetup = true

		if err := setupLeft(ctx, channel, option); err != nil {
			return err
		} else if err := setupRight(ctx, channel, option); err != nil {
			return err
		}

		this.Run(ctx, name, c, option)

		return nil
	}

	x.wrap(this)

	return x.new(this, l, nodeLeft, loop), x.new(this, r, nodeRight, false)
}

func (x *builder[T]) new(this *node[T], e Edge[T], c nodeChild, loop bool) Builder[T] {
	return &builder[T]{
		s: x.s,
		n: this,
		wrap: func(n *node[T]) {
			if loop {
				n.nodes[nodeLoop] = this
			} else {
				n.nodes[nodeLoop] = this.nodes[nodeLoop]
			}

			this.nodes[c] = n
		},
	}
}

func (x *node[T]) setup(e Edge[T], c nodeChild) func(ctx context.Context, channel chan []T, option *Option[T]) error {
	return func(ctx context.Context, channel chan []T, option *Option[T]) error {
		if x.nodes[c] == nil && x.nodes[nodeLoop] != nil {
			x.nodes[c] = x.nodes[nodeLoop]
		}

		if x.nodes[c] != nil {
			ch := make(chan []T, option.BufferSize)
			e.OutputTo(ctx, ch)
			return x.nodes[c].run(ctx, ch, option)
		}

		return fmt.Errorf("non-terminated builder next %s", x.name)
	}
}

// New is a function for creating a new Stream.
//
// name string
// provider EdgeProvider[T]
// option *Option[T]
//
func New[T Identifiable](name string, provider EdgeProvider[T], options *Option[T]) Stream[T] {
	return &stream[T]{
		name:        name,
		provider:    provider,
		consumables: []Edge[T]{},
		option:      options,
	}
}

// NewWithChannels is a function for creating a new Stream using channels to pass the data.
//
// name string
// option *Option[T]
//
func NewWithChannels[T Identifiable](name string, options *Option[T]) Stream[T] {
	return &stream[T]{
		name:        name,
		provider:    &edgeProvider[T]{},
		consumables: []Edge[T]{},
		option:      options,
	}
}
