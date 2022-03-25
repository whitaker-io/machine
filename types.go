// Package machine - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.
package machine

import "go.opentelemetry.io/otel/attribute"

// Identifiable is an interface that is used for providing an ID for a packet
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
	// Telemetry provides the ability to enable and configure telemetry
	Telemetry *Telemetry `json:"telemetry,omitempty" mapstructure:"telemetry,omitempty"`
	// Provider determines the edge type to be used, logic for what type of edge
	// for a given id is required if not using homogeneous edges
	// Default: nil
	Provider EdgeProvider[T] `json:"-" mapstructure:"-"`
	// PanicHandler is a function that is called when a panic occurs
	// Default: log the panic and no-op
	PanicHandler func(streamID, vertexID string, err error, payload ...T) `json:"-" mapstructure:"-"`
}

// Telemetry type for holding telemetry settings.
type Telemetry struct {
	// Enabled controls whether Telemetry is enabled or not
	Enabled *bool `json:"enabled,omitempty" mapstructure:"enabled,omitempty"`
	// TracerName is the name of the tracer to use
	TracerName *string `json:"tracer_name,omitempty" mapstructure:"tracer_name,omitempty"`
	// LabelPrefix is the prefix for the label keys used in Telemetry
	LabelPrefix *string `json:"prefix,omitempty" mapstructure:"prefix,omitempty"`
	// AdditionalOtelLabels are additional labels that will be added to the
	// metrics and traces.
	AdditionalLabels []attribute.KeyValue `json:"additional_otel_labels,omitempty" mapstructure:"additional_otel_labels,omitempty"`
}

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
		Telemetry:    o.Telemetry,
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

	if option.Telemetry != nil {
		out.Telemetry = &Telemetry{}

		if option.Telemetry.Enabled != nil {
			out.Telemetry.Enabled = option.Telemetry.Enabled
		} else {
			out.Telemetry.Enabled = boolP(false)
		}

		if option.Telemetry.TracerName != nil {
			out.Telemetry.TracerName = option.Telemetry.TracerName
		} else {
			out.Telemetry.TracerName = stringP("machine")
		}

		if option.Telemetry.LabelPrefix != nil {
			out.Telemetry.LabelPrefix = option.Telemetry.LabelPrefix
		} else {
			out.Telemetry.LabelPrefix = stringP("")
		}

		if option.Telemetry.AdditionalLabels != nil {
			out.Telemetry.AdditionalLabels = option.Telemetry.AdditionalLabels
		} else {
			out.Telemetry.AdditionalLabels = []attribute.KeyValue{}
		}
	} else {
		out.Telemetry = &Telemetry{
			Enabled:          boolP(false),
			TracerName:       stringP("machine"),
			LabelPrefix:      stringP(""),
			AdditionalLabels: []attribute.KeyValue{},
		}
	}

	if option.Provider != nil {
		out.Provider = option.Provider
	}

	if option.PanicHandler != nil {
		out.PanicHandler = option.PanicHandler
	}

	return out
}
