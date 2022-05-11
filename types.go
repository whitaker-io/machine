// Package machine - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.
package machine

import (
	"fmt"
	"sort"
)

const (
	// FilterLeft is a filter result that indicates that the item should move to the left branch.
	FilterLeft FilterResult = iota
	// FilterRight is a filter result that indicates that the item should move to the right branch.
	FilterRight
	// FilterBoth is a filter result that indicates that the item should go down both paths using the option.Deepcopy fn if provided.
	FilterBoth
)

// Duplicate shorthand for FilterBoth.
func Duplicate[T Identifiable](d T) FilterResult {
	return FilterBoth
}

// FilterResult is a type that is used to indicate the result of a filter.
type FilterResult uint8

// Identifiable is an interface that is used for providing an ID for a packet
type Identifiable interface {
	ID() string
}

// Applicative is a function that is applied on an individual
// basis for each Packet in the payload. The resulting data replaces
// the old data
type Applicative[T Identifiable] func(d T) T

// Combiner is a function used to combine a payload into a single Packet.
type Combiner[T Identifiable] func(payload []T) T

// Filter is a function that can be used to filter the payload.
type Filter[T Identifiable] func(d T) FilterResult

// Comparator is a function to compare 2 items
type Comparator[T Identifiable] func(a T, b T) int

// Window is a function to work on a window of data
type Window[T Identifiable] func(payload []T) []T

// Remover func that is used to remove Data based on a true result
type Remover[T Identifiable] func(index int, d T) bool

func (x Applicative[T]) Component(output Edge[T]) Vertex[T] {
	return func(payload []T) {
		for i, packet := range payload {
			payload[i] = x(packet)
		}

		output.Input(payload...)
	}
}

// Component is a function for providing a vertex that can be used to run individual components on the payload.
func (x Combiner[T]) Component(output Edge[T]) Vertex[T] {
	return func(payload []T) {
		output.Input(x(payload))
	}
}

// Component is a function for providing a vertex that can be used to run individual components on the payload.
func (x Filter[T]) Component(left, right Edge[T], option *Option[T]) Vertex[T] {
	return func(payload []T) {
		l := []T{}
		r := []T{}

		for _, packet := range payload {
			fr := x(packet)

			if fr == FilterLeft {
				l = append(l, packet)
			} else if fr == FilterRight {
				r = append(r, packet)
			} else if fr == FilterBoth {
				l = append(l, packet)
				r = append(r, option.deepCopy(packet)...)
			} else {
				panic(fmt.Errorf("Unknown filter result: %v", x(packet)))
			}
		}

		left.Input(l...)
		right.Input(r...)
	}
}

// Component is a function for providing a vertex that can be used to run individual components on the payload.
func (x Comparator[T]) Component(output Edge[T]) Vertex[T] {
	return func(payload []T) {
		sort.Slice(payload, func(i, j int) bool {
			return x(payload[i], payload[j]) < 0
		})

		output.Input(payload...)
	}
}

// Component is a function for providing a vertex that can be used to run individual components on the payload.
func (x Window[T]) Component(output Edge[T]) Vertex[T] {
	return func(payload []T) {
		output.Input(x(payload)...)
	}
}

// Component is a function for providing a vertex that can be used to run individual components on the payload.
func (x Remover[T]) Component(output Edge[T]) Vertex[T] {
	return func(payload []T) {
		out := []T{}

		for i, v := range payload {
			if !x(i, v) {
				out = append(out, v)
			}
		}

		output.Input(out...)
	}
}
