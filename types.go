// Package machine - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.
package machine

import (
	"context"
	"fmt"
	"time"
)

// Monad is a function that is applied to payload and used for transformations
type Monad[T any] func(d T) T

// Filter is a function that can be used to filter the payload.
type Filter[T any] func(d T) bool

// Edge is an interface that is used for transferring data between vertices
type Edge[T any] interface {
	Output() chan T
	Send(payload T)
}

// Telemetry type for holding telemetry settings.
type Telemetry[T any] interface {
	IncrementPayloadCount(vertexName string)
	IncrementErrorCount(vertexName string)
	Duration(vertexName string, duration time.Duration)
	RecordPayload(vertexName string, payload T)
	RecordError(vertexName string, payload T, err error)
}

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
	// DeepCopyBetweenVerticies controls whether DeepCopy is performed between verticies.
	// This is useful if the functions applied are holding copies of the payload for
	// longer than they process it. DeepCopy must be set
	DeepCopyBetweenVerticies bool `json:"deep_copy_between_vetricies,omitempty"`
	// DeepCopy is a function to preform a deep copy of the Payload
	DeepCopy Monad[T] `json:"-"`
}

func getOption[T any](option *Option[T]) *Option[T] {
	opt := &Option[T]{}

	if option != nil {
		opt.FIFO = option.FIFO
		opt.BufferSize = option.BufferSize
		opt.Telemetry = option.Telemetry
		opt.DeepCopyBetweenVerticies = option.DeepCopyBetweenVerticies
		opt.DeepCopy = option.DeepCopy
	}

	if option != nil && option.PanicHandler != nil {
		opt.PanicHandler = option.PanicHandler
	} else {
		opt.PanicHandler = func(err error, payload T) {
			fmt.Printf("panic: %v, payload: %v\n", err, payload)
		}
	}

	return opt
}

type vertex[T any] func(payload T)

type recursiveBaseFn[T any] func(recursiveBaseFn[T]) Monad[T]
type memoizedBaseFn[T any] func(h memoizedBaseFn[T], m map[string]T) Monad[T]

type filterList[T any] []Filter[T]
type filterComponent[T any] func(left, right chan T, option *Option[T]) vertex[T]

func (x Monad[T]) component(output chan T) vertex[T] {
	return func(payload T) {
		output <- x(payload)
	}
}

func (x filterList[T]) or() Filter[T] {
	if len(x) == 1 {
		return x[0]
	}

	return func(d T) bool {
		return x[0](d) || x[1:].or()(d)
	}
}

func (x filterList[T]) and() Filter[T] {
	if len(x) == 1 {
		return x[0]
	}

	return func(d T) bool {
		return x[0](d) && x[1:].and()(d)
	}
}

func (x Filter[T]) component(left, right chan T, _ *Option[T]) vertex[T] {
	return func(payload T) {
		fr := x(payload)

		if fr {
			left <- payload
		} else {
			right <- payload
		}
	}
}

func duplicateComponent[T any](left, right chan T, option *Option[T]) vertex[T] {
	return func(payload T) {
		payload2 := payload

		if option.DeepCopy != nil {
			payload2 = option.DeepCopy(payload)
		}

		left <- payload
		right <- payload2
	}
}

func (x vertex[T]) wrap(name string, option *Option[T]) vertex[T] {
	return func(payload T) {
		start := time.Now()

		if option.Telemetry != nil {
			option.Telemetry.IncrementPayloadCount(name)
			option.Telemetry.RecordPayload(name, payload)
		}

		defer recoverFn(name, start, payload, option)

		if option.DeepCopyBetweenVerticies && option.DeepCopy != nil {
			x(option.DeepCopy(payload))
		} else {
			x(payload)
		}
	}
}

func (x vertex[T]) run(ctx context.Context, name string, channel chan T, option *Option[T]) {
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
			option.PanicHandler(err, payload)
		}
	}
}
