// Package machine - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.
package machine

import (
	"context"
)

type Identifiable interface {
	ID() string
}

// Option type for holding machine settings.
type Option[T Identifiable] struct {
	// DeepCopy uses encoding/gob to create a deep copy of the payload
	// before the processing to ensure concurrent map exceptions cannot
	// happen. Is fairly resource intensive so use with caution.
	// Default: false
	DeepCopy *bool `json:"deep_copy,omitempty" mapstructure:"deep_copy,omitempty"`
	// FIFO controls the processing order of the payloads
	// If set to true the system will wait for one payload
	// to be processed before starting the next.
	// Default: false
	FIFO *bool `json:"fifo,omitempty" mapstructure:"fifo,omitempty"`
	// BufferSize sets the buffer size on the edge channels between the
	// vertices, this setting can be useful when processing large amounts
	// of data with FIFO turned on.
	// Default: 0
	BufferSize *int `json:"buffer_size,omitempty" mapstructure:"buffer_size,omitempty"`
	// Provider determines the edge type to be used, logic for what type of edge
	// for a given id is required if not using homogeneous edges
	// Default: nil
	Provider EdgeProvider[T] `json:"-" mapstructure:"-"`
	// PanicHandler is a function that is called when a panic occurs
	// Default: log the panic and no-op
	PanicHandler func(streamID, vertexID string, err error, payload ...T) `json:"-" mapstructure:"-"`
}

// Retriever is a function that provides data to a generic Stream
// must stop when the context receives a done signal.
type Retriever[T Identifiable] func(ctx context.Context) chan []T

// Applicative is a function that is applied on an individual
// basis for each Packet in the payload. The resulting data replaces
// the old data
type Applicative[T Identifiable] func(d T) T

// Fold is a function used to combine a payload into a single Packet.
// It may be used with either a Fold Left or Fold Right operation,
// which starts at the corresponding side and moves through the payload.
// The returned instance of T is used as the aggregate in the subsequent
// call.
type Fold[T Identifiable] func(aggregate, next T) T

// Filter is a function that can be used to filter the payload.
type Filter[T Identifiable] func(d T) bool

// Comparator is a function to compare 2 items
type Comparator[T Identifiable] func(a T, b T) int

// Window is a function to work on a window of data
type Window[T Identifiable] func(payload []T) []T

// Remover func that is used to remove Data based on a true result
type Remover[T Identifiable] func(index int, d T) bool

func (o *Option[T]) merge(options ...*Option[T]) *Option[T] {
	if len(options) < 1 {
		return o
	} else if len(options) == 1 {
		return o.join(options[0])
	}

	return o.join(options[0]).merge(options[1:]...)
}

func (o *Option[T]) join(option *Option[T]) *Option[T] {
	out := &Option[T]{
		DeepCopy:     o.DeepCopy,
		FIFO:         o.FIFO,
		BufferSize:   o.BufferSize,
		Provider:     o.Provider,
		PanicHandler: o.PanicHandler,
	}

	if option.DeepCopy != nil {
		out.DeepCopy = option.DeepCopy
	}

	if option.FIFO != nil {
		out.FIFO = option.FIFO
	}

	if option.BufferSize != nil {
		out.BufferSize = option.BufferSize
	}

	if option.Provider != nil {
		out.Provider = option.Provider
	}

	if option.PanicHandler != nil {
		out.PanicHandler = option.PanicHandler
	}

	return out
}
