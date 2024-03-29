// Package machine - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.
package machine

import (
	"context"
	"fmt"
)

// Machine is the interface provided for creating a data processing stream.
type Machine[T any] interface {
	// Name returns the name of the Machine path. Useful for debugging or reasoning about the path.
	Name() string
	// Then apply a mutation to each individual element of the payload.
	Then(a ...Monad[T]) Machine[T]
	// Recurse applies a recursive function to the payload through a Y Combinator.
	// f is a function used by the Y Combinator to perform a recursion
	// on the payload.
	// Example:
	//
	//	func(f Monad[int]) Monad[int] {
	//		 return func(x int) int {
	//			 if x <= 0 {
	//				 return 1
	//			 } else {
	//				 return x * f(x-1)
	//			 }
	//		 }
	//	}
	Recurse(x Monad[Monad[T]]) Machine[T]
	// Memoize applies a recursive function to the payload through a Y Combinator
	// and memoizes the results based on the index func.
	// f is a function used by the Y Combinator to perform a recursion
	// on the payload.
	// Example:
	//
	//	func(f Monad[int]) Monad[int] {
	//		 return func(x int) int {
	//			 if x <= 0 {
	//				 return 1
	//			 } else {
	//				 return x * f(x-1)
	//			 }
	//		 }
	//	}
	Memoize(x Monad[Monad[T]], index func(T) string) Machine[T]
	// Or runs all of the functions until one succeeds or sends the payload to the right branch
	Or(x ...Filter[T]) (Machine[T], Machine[T])
	// And runs all of the functions and if one doesnt succeed sends the payload to the right branch
	And(x ...Filter[T]) (Machine[T], Machine[T])
	// Filter splits the data into multiple stream branches
	If(f Filter[T]) (Machine[T], Machine[T])
	// Select applies a series of Filters to the payload and returns a list of Builders
	// the last one being for any unmatched payloads.
	Select(fns ...Filter[T]) []Machine[T]
	// Tee duplicates the data into multiple stream branches.
	Tee(func(T) (a, b T)) (Machine[T], Machine[T])
	// While creates a loop in the stream based on the filter
	While(x Filter[T]) (loop, out Machine[T])
	// Drop terminates the data from further processing without passing it on
	Drop()
	// Distribute is a function used for fanout
	Distribute(Edge[T]) Machine[T]
	// Output provided channel
	Output() chan T

	component(typeName string, fn func(output chan T) vertex[T]) Machine[T]
	filterComponent(typeName string, fn filterComponent[T], loop bool) (Machine[T], Machine[T])
	setup(ctx context.Context)
	next(name string) *builder[T]
}

type builder[T any] struct {
	name   string
	option *config
	output chan T
	start  func(ctx context.Context, channel chan T)
	loop   *builder[T]
}

// New is a function for creating a new Machine.
//
// name string
// input chan T
// option *Option[T]
//
// Call the startFn returned by New to start the Machine once built.
func New[T any](name string, input chan T, options ...Option) (startFn func(context.Context), x Machine[T]) {
	c := &config{}

	for _, o := range options {
		o.apply(c)
	}

	b := &builder[T]{
		name:   name,
		loop:   nil,
		option: c,
		output: input,
	}
	return func(ctx context.Context) {
		b.start(ctx, input)
	}, b
}

// Transform is a function for converting the type of the Machine. Cannot be used inside a loop
// until I figure out how to do it without some kind of run time error or overly complex
// tracking method that isn't type safe. I really wish method level generics were a thing.
func Transform[T, U any](m Machine[T], fn func(d T) U) (Machine[U], error) {
	x := m.(*builder[T])

	if x.loop != nil {
		return nil, fmt.Errorf("transform cannot be used in a loop")
	}

	this := &builder[U]{
		name:   x.name + ":" + "transform",
		loop:   nil,
		option: x.option,
		output: make(chan U, x.option.bufferSize),
	}

	x.start = func(ctx context.Context, channel chan T) {
		this.setup(ctx)
		vertex[T](func(_ context.Context, payload T) {
			this.output <- fn(payload)
		}).run(ctx, this.name, channel, x.option)
	}

	return this, nil
}

// Name returns the name of the Machine path. Useful for debugging or reasoning about the path.
func (x *builder[T]) Name() string {
	return x.name
}

// Then apply a mutation to each individual element of the payload.
func (x *builder[T]) Then(fn ...Monad[T]) Machine[T] {
	return x.component("then", monadList[T](fn).combine().component)
}

// Select applies a series of Filters to the payload and returns a list of Builders
// the last one being for any unmatched payloads.
func (x *builder[T]) Select(fns ...Filter[T]) []Machine[T] {
	out := []Machine[T]{}

	var last = x
	for i, fn := range fns {
		o, l := last.filterComponent(fmt.Sprintf("select-%d", i), fn.component, false)
		out = append(out, o)
		last = l.(*builder[T])
	}

	out = append(out, last)

	return out
}

// Recurse applies a recursive function to the payload through a Y Combinator.
func (x *builder[T]) Recurse(fn Monad[Monad[T]]) Machine[T] {
	g := func(h recursiveBaseFn[T]) Monad[T] {
		return func(payload T) T {
			return fn(h(h))(payload)
		}
	}
	p := g(g)

	return x.component("recurse", p.component)
}

// Memoize applies a recursive function to the payload through a Y Combinator
// and memoizes the results based on the index func.
func (x *builder[T]) Memoize(fn Monad[Monad[T]], index func(T) string) Machine[T] {
	g := func(h memoizedBaseFn[T], m map[string]T) Monad[T] {
		return func(payload T) T {
			id := index(payload)
			if v, ok := m[id]; ok {
				return v
			}

			m[id] = fn(h(h, m))(payload)
			return m[id]
		}
	}
	p := Monad[T](func(payload T) T {
		m := map[string]T{}
		return g(g, m)(payload)
	})

	return x.component("memoize", p.component)
}

// Drop terminates the data from further processing without passing it on
func (x *builder[T]) Drop() {
	x.start = func(ctx context.Context, input chan T) {
		go transfer(ctx, input, func(_ context.Context, _ T) {}, "", &config{})
	}
}

// Or runs all of the functions until one succeeds or sends the payload to the right branch
func (x *builder[T]) Or(list ...Filter[T]) (left, right Machine[T]) {
	return x.filterComponent("or", filterList[T](list).or().component, false)
}

// And runs all of the functions and if one doesnt succeed sends the payload to the right branch
func (x *builder[T]) And(list ...Filter[T]) (left, right Machine[T]) {
	return x.filterComponent("and", filterList[T](list).and().component, false)
}

// If splits the data into multiple stream branches
func (x *builder[T]) If(fn Filter[T]) (left, right Machine[T]) {
	return x.filterComponent("if", fn.component, false)
}

// Tee duplicates the data into multiple stream branches. The payload/vertexes are
// responsible for concurrent read/write controls
func (x *builder[T]) Tee(fn func(T) (a, b T)) (left, right Machine[T]) {
	return x.filterComponent("tee",
		func(left, right chan T) vertex[T] {
			return func(_ context.Context, payload T) {
				a, b := fn(payload)
				left <- a
				right <- b
			}
		},
		false,
	)
}

// While creates a loop in the stream based on the filter
func (x *builder[T]) While(fn Filter[T]) (loop, out Machine[T]) {
	return x.filterComponent("while", fn.component, true)
}

// Distribute is a function used for fanout
func (x *builder[T]) Distribute(edge Edge[T]) Machine[T] {
	this := x.next("distribute")

	this.output = edge.Output()
	x.start = func(ctx context.Context, channel chan T) {
		this.setup(ctx)

		vertex[T](edge.Send).run(ctx, this.name, channel, x.option)
	}

	return this
}

// Output return output channel
func (x *builder[T]) Output() chan T {
	return x.output
}

func (x *builder[T]) component(typeName string, fn func(output chan T) vertex[T]) Machine[T] {
	this := x.next(typeName)

	x.start = func(ctx context.Context, channel chan T) {
		this.setup(ctx)
		fn(this.output).run(ctx, this.name, channel, x.option)
	}

	return this
}

func (x *builder[T]) filterComponent(typeName string, fn filterComponent[T], loop bool) (Machine[T], Machine[T]) {
	name := x.name + ":" + typeName

	l := x.loop

	if loop {
		l = x
	}

	left := &builder[T]{
		name:   name + ":left",
		loop:   l,
		option: x.option,
		output: make(chan T, x.option.bufferSize),
	}

	right := x.next("right")

	alreadySetup := false

	x.start = func(ctx context.Context, channel chan T) {
		if alreadySetup {
			if typeName == "while" {
				go transfer(ctx, channel,
					func(_ context.Context, data T) {
						x.output <- data
					},
					name,
					x.option,
				)
			}
			return
		}

		alreadySetup = true

		left.setup(ctx)
		right.setup(ctx)

		fn(left.output, right.output).run(ctx, name, channel, x.option)
	}

	return left, right
}

func (x *builder[T]) setup(ctx context.Context) {
	if x.start == nil && x.loop != nil {
		x.start = x.loop.start
	}

	if x.start != nil {
		x.start(ctx, x.output)
	}
}

func (x *builder[T]) next(name string) *builder[T] {
	return &builder[T]{
		name:   x.name + ":" + name,
		loop:   x.loop,
		option: x.option,
		output: make(chan T, x.option.bufferSize),
	}
}

func transfer[T any](ctx context.Context, input chan T, fn vertex[T], vertexName string, option *config) {
	for {
		select {
		case <-ctx.Done():
			if option.flushFN != nil && option.gracePeriod > 0 {
				flush(vertexName, input, option)
			}
			return
		case data := <-input:
			fn(ctx, data)
		}
	}
}

func flush[T any](vertexName string, input chan T, option *config) {
	c, cancel := context.WithTimeout(context.Background(), option.gracePeriod)
	defer cancel()
	for {
		select {
		case <-c.Done():
			return
		case data := <-input:
			option.flushFN(vertexName, data)
		}
	}
}
