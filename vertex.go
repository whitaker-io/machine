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
type Component[T Identifiable] interface {
	Component(e Edge[T]) Vertex[T]
}

// EdgeProvider is an interface for providing an edge that will be used to communicate between vertices.
type EdgeProvider[T Identifiable] interface {
	New(name string, option *Option[T]) Edge[T]
}

// Edge is an interface that is used for transferring data between vertices
type Edge[T Identifiable] interface {
	OutputTo(ctx context.Context, channel chan []T)
	Input(payload ...T)
}

type edgeProvider[T Identifiable] struct{}

type edge[T Identifiable] struct {
	channel chan []T
}

// Vertex is a type used to process data for a stream.
type Vertex[T Identifiable] func(payload []T)

func (x Vertex[T]) buildHandler(name string, option *Option[T]) func(payload []T) {
	return func(payload []T) {
		var span Span[T]
		start := time.Now()

		if option.Telemetry != nil {
			option.Telemetry.IncrementPayloadCount(name)
			option.Telemetry.PayloadSize(name, int64(len(payload)))
			span = option.Telemetry.StartSpan(name)
			span.RecordPayload(payload...)
		}

		defer func() {
			if r := recover(); r != nil {
				if err, ok := r.(error); !ok {
					if option.Telemetry != nil {
						option.Telemetry.IncrementErrorCount(name)
					}

					if span != nil {
						span.RecordError(err)
					}
				}
			}
		}()

		defer func() {
			if option.Telemetry != nil {
				option.Telemetry.Duration(name, time.Since(start))
			}
		}()

		if option.DeepCopy != nil {
			x(option.deepCopy(payload...))
		} else {
			x(payload)
		}
	}
}

// Run creates a go func to process the data in the channel until the context is canceled.
func (x Vertex[T]) Run(ctx context.Context, name string, channel chan []T, option *Option[T]) {
	h := x.buildHandler(name, option)

	go func() {
	Loop:
		for {
			select {
			case <-ctx.Done():
				break Loop
			case data := <-channel:
				if len(data) < 1 {
					continue
				}

				if option.FIFO {
					h(data)
				} else {
					go h(data)
				}
			}
		}
	}()
}

func (x edgeProvider[T]) New(name string, option *Option[T]) Edge[T] {
	return &edge[T]{
		channel: make(chan []T, option.BufferSize),
	}
}

func (x *edge[T]) OutputTo(ctx context.Context, channel chan []T) {
	go func() {
	Loop:
		for {
			select {
			case <-ctx.Done():
				break Loop
			case list := <-x.channel:
				if len(list) > 0 {
					channel <- list
				}
			}
		}
	}()
}

func (x *edge[T]) Input(payload ...T) {
	x.channel <- payload
}

// AsEdge is a helper function to create an edge from a channel.
func AsEdge[T Identifiable](c chan []T) Edge[T] {
	return &edge[T]{c}
}
