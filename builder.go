// Package machine - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.
package machine

import (
	"context"
	"fmt"
	"sync"
)

// Stream is a representation of a data stream and its associated logic.
//
// The Builder method is the entrypoint into creating the data processing flow.
// All branches of the Stream are required to end in an OutputTo call.
type Stream[T any] interface {
	Start(ctx context.Context, input chan T) error
	Builder() Builder[T]
}

// Builder is the interface provided for creating a data processing stream.
type Builder[T any] interface {
	Then(a Applicative[T]) Builder[T]
	Y(x BaseFnTransform[T]) Builder[T]
	Or(x ...Test[T]) (Builder[T], Builder[T])
	And(x ...Test[T]) (Builder[T], Builder[T])
	Filter(f Filter[T]) (Builder[T], Builder[T])
	Duplicate() (Builder[T], Builder[T])
	Loop(x Filter[T]) (loop, out Builder[T])
	Drop()
	Distribute(Edge[T]) Builder[T]
	OutputTo(x chan T)
}

type builder[T any] struct {
	s      *stream[T]
	name   string
	output chan T
	start  func(ctx context.Context, channel chan T) error
	loop   *builder[T]
}

type stream[T any] struct {
	name    string
	mtx     *sync.Mutex
	builder *builder[T]
	option  *Option[T]
}

func (x *stream[T]) Start(ctx context.Context, input chan T) error {
	if x.builder == nil || x.builder.start == nil {
		return fmt.Errorf("non-terminated builder %s", x.name)
	}

	return x.builder.start(ctx, input)
}

func (x *stream[T]) Builder() Builder[T] {
	x.builder = &builder[T]{
		s:    x,
		name: x.name,
	}

	return x.builder
}

// Then apply a mutation to each individual element of the payload.
func (x *builder[T]) Then(fn Applicative[T]) Builder[T] {
	return x.component("then", fn)
}

// Y applies a recursive function to the payload through a Y Combinator.
func (x *builder[T]) Y(fn BaseFnTransform[T]) Builder[T] {
	return x.component("y", fn)
}

// Drop terminates the data from further processing without passing it on
func (x *builder[T]) Drop() {
	x.start = func(ctx context.Context, input chan T) error {
		go func() {
		Loop:
			for {
				select {
				case <-ctx.Done():
					break Loop
				case <-input:
				}
			}
		}()
		return nil
	}
}

// Or runs all of the functions until one succeeds or sends the payload to the right branch
func (x *builder[T]) Or(list ...Test[T]) (left, right Builder[T]) {
	return x.filterComponent("or", testList[T](list).OrCompose().Component, false)
}

// And runs all of the functions and if one doesnt succeed sends the payload to the right branch
func (x *builder[T]) And(list ...Test[T]) (left, right Builder[T]) {
	return x.filterComponent("or", testList[T](list).AndCompose().Component, false)
}

// Filter splits the data into multiple stream branches
func (x *builder[T]) Filter(fn Filter[T]) (left, right Builder[T]) {
	return x.filterComponent("filter", fn.Component, false)
}

// Filter splits the data into multiple stream branches
func (x *builder[T]) Duplicate() (left, right Builder[T]) {
	return x.filterComponent("duplicate", duplicateComponent[T], false)
}

// Loop creates a loop in the stream based on the filter
func (x *builder[T]) Loop(fn Filter[T]) (loop, out Builder[T]) {
	return x.filterComponent("loop", fn.Component, true)
}

// Distribute is a function used for fanout
func (x *builder[T]) Distribute(edge Edge[T]) Builder[T] {
	this := x.next("distribute")

	x.start = func(ctx context.Context, channel chan T) error {
		if err := this.setup(ctx); err != nil {
			return err
		}

		edge.ReceiveOn(ctx, this.output)
		edgeComponent(edge).Run(ctx, this.name, channel, x.s.option)

		return nil
	}

	return this
}

// OutputTo caps the builder and sends the output to the provided channel
func (x *builder[T]) OutputTo(channel chan T) {
	x.start = func(ctx context.Context, input chan T) error {
		outputTo(ctx, input, channel)
		return nil
	}
}

func (x *builder[T]) component(typeName string, fn Component[T]) Builder[T] {
	this := x.next(typeName)

	x.start = func(ctx context.Context, channel chan T) error {
		if err := this.setup(ctx); err != nil {
			return err
		}

		fn.Component(this.output).Run(ctx, this.name, channel, x.s.option)

		return nil
	}

	return this
}

func (x *builder[T]) filterComponent(typeName string, fn filterComponent[T], loop bool) (Builder[T], Builder[T]) {
	name := x.name + ":" + typeName

	l := x.loop

	if loop {
		l = x
	}

	left := &builder[T]{
		name:   name + ":left",
		s:      x.s,
		loop:   l,
		output: make(chan T, x.s.option.BufferSize),
	}

	right := x.next("right")

	alreadySetup := false

	x.start = func(ctx context.Context, channel chan T) error {
		if alreadySetup {
			if typeName == "loop" {
				outputTo(ctx, channel, x.output)
			}
			return nil
		}

		alreadySetup = true

		if err := left.setup(ctx); err != nil {
			return err
		} else if err := right.setup(ctx); err != nil {
			return err
		}

		fn(left.output, right.output, x.s.option).Run(ctx, name, channel, x.s.option)

		return nil
	}

	return left, right
}

func (x *builder[T]) setup(ctx context.Context) error {
	if x.start == nil && x.loop != nil {
		x.start = x.loop.start
	}

	if x.start == nil {
		return fmt.Errorf("non-terminated builder %s", x.name)
	}

	return x.start(ctx, x.output)
}

func (x *builder[T]) next(name string) *builder[T] {
	return &builder[T]{
		name:   name + ":" + name,
		s:      x.s,
		loop:   x.loop,
		output: make(chan T, x.s.option.BufferSize),
	}
}

// New is a function for creating a new Stream.
//
// name string
// option *Option[T]
func New[T any](name string, options *Option[T]) Stream[T] {
	return &stream[T]{
		name:   name,
		mtx:    &sync.Mutex{},
		option: options,
	}
}

func outputTo[T any](ctx context.Context, in, out chan T) {
	go func() {
	Loop:
		for {
			select {
			case <-ctx.Done():
				break Loop
			case data := <-in:
				out <- data
			}
		}
	}()
}
