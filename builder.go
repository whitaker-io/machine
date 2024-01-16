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
	// Drop terminates the data from further processing without passing it on
	Drop()
	// Distribute is a function used for fanout
	Distribute(Edge[T]) Machine[T]
	// Output provided channel
	Output() chan T

	monadNext(name string, m Monad[T]) (vertex[T], *builder[T])
	filterNext(left, right string, f Filter[T]) (vertex[T], *builder[T], *builder[T])
	next(name string, output chan T) *builder[T]
}

type builder[T any] struct {
	ctx    context.Context
	name   string
	output chan T
	option *config
}

func New[T any](ctx context.Context, name string, channel chan T, option ...Option) Machine[T] {
	c := &config{
		bufferSize:  0,
		gracePeriod: 0,
	}

	for _, o := range option {
		o.apply(c)
	}

	return &builder[T]{
		ctx:    ctx,
		name:   name,
		output: channel,
		option: c,
	}
}

// Transform is a function for converting the type of the Machine.
func Transform[T, U any](m Machine[T], fn func(d T) U) Machine[U] {
	x := m.(*builder[T])

	next := &builder[U]{
		ctx:    x.ctx,
		name:   x.name + ":" + "transform",
		output: make(chan U, x.option.bufferSize),
		option: x.option,
	}

	vertex[T](func(ctx context.Context, payload T) {
		next.output <- fn(payload)
	}).run(next.ctx, next.name, x.output, x.option)

	return next
}

// Name returns the name of the Machine path. Useful for debugging or reasoning about the path.
func (x *builder[T]) Name() string {
	return x.name
}

// Then apply a mutation to each individual element of the payload.
func (x *builder[T]) Then(fn ...Monad[T]) Machine[T] {
	return x.setupMonad("then", monadList[T](fn).combine())
}

// Select applies a series of Filters to the payload and returns a list of Builders
// the last one being for any unmatched payloads.
func (x *builder[T]) Select(fns ...Filter[T]) []Machine[T] {
	out := []Machine[T]{}

	var last = x
	for i, fn := range fns {
		o, l := last.setupFilter(fmt.Sprintf("select-%d", i), fn)
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

	return x.setupMonad("recurse", p)
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

	return x.setupMonad("memoize", p)
}

// Drop terminates the data from further processing without passing it on
func (x *builder[T]) Drop() {
	next := x.next("drop", nil)
	vertex[T](func(ctx context.Context, payload T) {}).run(next.ctx, next.name, x.output, x.option)
}

// Or runs all of the functions until one succeeds or sends the payload to the right branch
func (x *builder[T]) Or(list ...Filter[T]) (left, right Machine[T]) {
	return x.setupFilter("or", filterList[T](list).or())
}

// And runs all of the functions and if one doesnt succeed sends the payload to the right branch
func (x *builder[T]) And(list ...Filter[T]) (left, right Machine[T]) {
	return x.setupFilter("and", filterList[T](list).and())
}

// If splits the data into multiple stream branches
func (x *builder[T]) If(fn Filter[T]) (left, right Machine[T]) {
	return x.setupFilter("if", fn)
}

// Tee duplicates the data into multiple stream branches. The payload/vertexes are
// responsible for concurrent read/write controls
func (x *builder[T]) Tee(fn func(T) (a, b T)) (Machine[T], Machine[T]) {
	name := x.name + ":tee"

	l, r := make(chan T, x.option.bufferSize), make(chan T, x.option.bufferSize)

	v := vertex[T](func(ctx context.Context, payload T) {
		a, b := fn(payload)
		l <- a
		r <- b
	})

	left, right := x.next(name+":left", l), x.next(name+":right", r)

	v.run(left.ctx, left.name, x.output, x.option)

	return left, right
}

// Distribute is a function used for fanout
func (x *builder[T]) Distribute(edge Edge[T]) Machine[T] {
	next := x.next("distribute", edge.Output())

	vertex[T](edge.Send).run(next.ctx, next.name, x.output, x.option)

	return next
}

// Output return output channel
func (x *builder[T]) Output() chan T {
	return x.output
}

func (x *builder[T]) monadNext(name string, m Monad[T]) (vertex[T], *builder[T]) {
	v, output := m.convert(x.option)
	return v, x.next(name, output)
}

func (x *builder[T]) setupMonad(typeName string, m Monad[T]) Machine[T] {
	vert, next := x.monadNext(typeName, m)

	vert.run(next.ctx, next.name, x.output, x.option)

	return next
}

func (x *builder[T]) filterNext(left, right string, f Filter[T]) (vertex[T], *builder[T], *builder[T]) {
	v, l, r := f.convert(x.option)
	return v, x.next(left, l), x.next(right, r)
}

func (x *builder[T]) setupFilter(typeName string, f Filter[T]) (Machine[T], Machine[T]) {
	name := x.name + ":" + typeName

	vert, left, right := x.filterNext(name+":left", name+":right", f)

	vert.run(left.ctx, left.name, x.output, x.option)

	return left, right
}

func (x *builder[T]) next(name string, output chan T) *builder[T] {
	next := &builder[T]{
		ctx:    x.ctx,
		name:   x.name + ":" + name,
		output: output,
		option: x.option,
	}

	return next
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
