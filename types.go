// Package machine - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.
package machine

import (
	"context"
	"time"
)

// Component is an interface for providing a vertex that can be used to run individual components on the payload.
type Component[T any] interface {
	Component(e chan T) Vertex[T]
}

// Edge is an interface that is used for transferring data between vertices
type Edge[T any] interface {
	ReceiveOn(ctx context.Context, channel chan T)
	Send(payload T)
}

// Vertex is a type used to process data for a stream.
type Vertex[T any] func(payload T)

// Applicative is a function that is applied to payload and used for transformations
type Applicative[T any] func(d T) T

// Test is a function used in composition of And/Or operations and used to
// filter results down different branches with transformations
type Test[T any] func(d T) (T, error)

// Filter is a function that can be used to filter the payload.
type Filter[T any] func(d T) bool

// Option type for holding machine settings.
type Option[T any] struct {
	// FIFO controls the processing order of the payloads
	// If set to true the system will wait for one payload
	// to be processed before starting the next.
	FIFO bool `json:"fifo,omitempty"`
	// BufferSize sets the buffer size on the edge channels between the
	// vertices, this setting can be useful when processing large amounts
	// of data with FIFO turned on.
	BufferSize int `json:"buffer_size,omitempty"`
	// Telemetry provides the ability to enable and configure telemetry
	Telemetry Telemetry[T] `json:"telemetry,omitempty"`
	// PanicHandler is a function that is called when a panic occurs
	PanicHandler func(err error, payload T) `json:"-"`
	// DeepCopy is a function to preform a deep copy of the Payload
	DeepCopy func(T) T `json:"-"`
}

// Telemetry type for holding telemetry settings.
type Telemetry[T any] interface {
	IncrementPayloadCount(vertexName string)
	IncrementErrorCount(vertexName string)
	Duration(vertexName string, duration time.Duration)
	RecordPayload(vertexName string, payload T)
	RecordError(vertexName string, payload T, err error)
}

type testList[T any] []Test[T]

// Component is a function for providing a vertex that can be used to run individual components on the payload.
func (x Applicative[T]) Component(output chan T) Vertex[T] {
	return func(payload T) {
		output <- x(payload)
	}
}

func (x testList[T]) OrCompose() Test[T] {
	if len(x) == 1 {
		return x[0]
	}

	x2 := x[1:].OrCompose()

	return func(d T) (T, error) {
		out, err := x[0](d)

		if err == nil {
			return out, nil
		}

		return x2(d)
	}
}

func (x testList[T]) AndCompose() Test[T] {
	if len(x) == 1 {
		return x[0]
	}

	x2 := x[1:].AndCompose()

	return func(d T) (T, error) {
		out, err := x[0](d)

		if err == nil {
			return x2(d)
		}

		return out, err
	}
}

type filterComponent[T any] func(left, right chan T, option *Option[T]) Vertex[T]

// Component is a function for providing a vertex that can be used to run individual components on the payload.
func (x Test[T]) Component(left, right chan T, _ *Option[T]) Vertex[T] {
	return func(payload T) {
		out, err := x(payload)

		if err == nil {
			left <- out
		} else {
			right <- out
		}
	}
}

// Component is a function for providing a vertex that can be used to run individual components on the payload.
func (x Filter[T]) Component(left, right chan T, _ *Option[T]) Vertex[T] {
	return func(payload T) {
		fr := x(payload)

		if fr {
			left <- payload
		} else {
			right <- payload
		}
	}
}

func duplicateComponent[T any](left, right chan T, option *Option[T]) Vertex[T] {
	return func(payload T) {
		payload2 := payload

		if option.DeepCopy != nil {
			payload2 = option.DeepCopy(payload)
		}

		left <- payload
		right <- payload2
	}
}

func edgeComponent[T any](e Edge[T]) Vertex[T] {
	return func(payload T) {
		e.Send(payload)
	}
}

func (x Vertex[T]) wrap(name string, option *Option[T]) Vertex[T] {
	return func(payload T) {
		start := time.Now()

		if option.Telemetry != nil {
			option.Telemetry.IncrementPayloadCount(name)
			option.Telemetry.RecordPayload(name, payload)
		}

		defer recoverFn(name, start, payload, option)

		if option.DeepCopy != nil {
			x(option.DeepCopy(payload))
		} else {
			x(payload)
		}
	}
}

// Run creates a go func to process the data in the channel until the context is canceled.
func (x Vertex[T]) Run(ctx context.Context, name string, channel chan T, option *Option[T]) {
	h := x.wrap(name, option)

	go func() {
	Loop:
		for {
			select {
			case <-ctx.Done():
				break Loop
			case data := <-channel:
				if option.FIFO {
					h(data)
				} else {
					go h(data)
				}
			}
		}
	}()
}

func recoverFn[T any](name string, start time.Time, payload T, option *Option[T]) {
	if option.Telemetry != nil {
		option.Telemetry.Duration(name, time.Since(start))
	}

	if r := recover(); r != nil {
		if err, ok := r.(error); ok {
			if option.Telemetry != nil {
				option.Telemetry.IncrementErrorCount(name)
				option.Telemetry.RecordError(name, payload, err)
			}

			if option.PanicHandler != nil {
				option.PanicHandler(err, payload)
			}
		}
	}
}
