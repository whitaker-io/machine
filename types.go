// Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package machine

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"

	"github.com/mitchellh/mapstructure"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/whitaker-io/data"
)

var (
	// ForkDuplicate is a Fork that creates a deep copy of the
	// payload and sends it down both branches.
	ForkDuplicate Fork = func(payload []*Packet) (a, b []*Packet) {
		payload2 := []*Packet{}
		buf := &bytes.Buffer{}
		enc, dec := gob.NewEncoder(buf), gob.NewDecoder(buf)

		_ = enc.Encode(payload)
		_ = dec.Decode(&payload2)

		for i, packet := range payload {
			payload2[i].span = packet.span
		}

		return payload, payload2
	}

	// ForkError is a Fork that splits machine.Packets based on
	// the presence of an error. Errors are sent down the
	// right side path.
	ForkError Fork = func(payload []*Packet) (s, f []*Packet) {
		s = []*Packet{}
		f = []*Packet{}

		for _, packet := range payload {
			if packet.Error != nil {
				f = append(f, packet)
			} else {
				s = append(s, packet)
			}
		}

		return s, f
	}

	defaultOptions = &Option{
		DeepCopy:   boolP(false),
		FIFO:       boolP(false),
		Injectable: boolP(true),
		Metrics:    boolP(true),
		Span:       boolP(true),
		BufferSize: intP(0),
	}
)

// Data is a map[string]interface{} used to hold the data flowing into
// the Stream it can be converted to a struct using the As method, though
// this is temporary and can be costly.
type Data data.Data

// Packet type that holds information traveling through the machine.
type Packet struct {
	ID    string `json:"id"`
	Data  Data   `json:"data"`
	Error error  `json:"error"`
	span  trace.Span
}

// Option type for holding machine settings.
type Option struct {
	// DeepCopy uses encoding/gob to create a deep copy of the payload
	// before the processing to ensure concurrent map exceptions cannot
	// happen. Is fairly resource intensive so use with caution.
	// Default: false
	DeepCopy *bool
	// FIFO controls the processing order of the payloads
	// If set to true the system will wait for one payload
	// to be processed before starting the next.
	// Default: false
	FIFO *bool
	// Injectable controls whether the vertex accepts injection calls
	// if set to false the data will be logged and processing will not
	// take place.
	// Default: true
	Injectable *bool
	// BufferSize sets the buffer size on the edge channels between the
	// vertices, this setting can be useful when processing large amounts
	// of data with FIFO turned on.
	// Default: 0
	BufferSize *int
	// Span controls whether opentelemetry spans are created for tracing
	// Packets processed by the system.
	// Default: true
	Span *bool
	// Metrics controls whether opentelemetry metrics are recorded for
	// Packets processed by the system.
	// Default: true
	Metrics *bool
}

// Retriever is a function that provides data to a generic Stream
// must stop when the context receives a done signal.
type Retriever func(ctx context.Context) chan []Data

// Applicative is a function that is applied on an individual
// basis for each Packet in the payload. The data may be modified
// or checked for correctness. Any resulting error is combined
// with current errors in the wrapping Packet.
type Applicative func(data Data) error

// Fold is a function used to combine a payload into a single Packet.
// It may be used with either a Fold Left or Fold Right operation,
// which starts at the corresponding side and moves through the payload.
// The returned instance of Data is used as the aggregate in the subsequent
// call.
type Fold func(aggregate, next Data) Data

// Fork is a function for splitting the payload into 2 separate paths.
// Default Forks for duplication and error checking are provided by
// ForkDuplicate and ForkError respectively.
type Fork func(list []*Packet) (a, b []*Packet)

// ForkRule is a function that can be converted to a Fork via the Handler
// method allowing for Forking based on the contents of the data.
type ForkRule func(data Data) bool

type handler func(payload []*Packet)

type recorder func(id, vertexType, operation string, payload []*Packet)

type edge struct {
	channel chan []*Packet
}

// As helper function used for converting the Packet into the given struct or map
// uses github.com/mitchellh/mapstructure under the covers and input must be a
// pointer to a map or struct
func (p *Packet) As(i interface{}) error {
	return p.Data.As(i)
}

// As helper function used for converting the Data into the given struct or map
// uses github.com/mitchellh/mapstructure under the covers and input must be a
// pointer to a map or struct
func (d *Data) As(i interface{}) error {
	return mapstructure.Decode(d, i)
}

func (p *Packet) apply(id string, a Applicative) {
	p.handleError(id, a(p.Data))
}

func (p *Packet) handleError(id string, err error) {
	if err != nil {
		p.Error = fmt.Errorf("%s %s %w", id, err.Error(), p.Error)
	}
}

func (p *Packet) newSpan(ctx context.Context, tracer trace.Tracer, name string, vertexAttributes trace.LifeCycleOption) {
	_, p.span = tracer.Start(ctx, p.ID, trace.WithAttributes(attribute.String("packet_id", p.ID)))
	p.span.AddEvent(name, vertexAttributes)
}

func (o *Option) merge(options ...*Option) *Option {
	if len(options) < 1 {
		return o
	} else if len(options) == 1 {
		return o.join(options[0])
	}

	return o.join(options[0]).merge(options[1:]...)
}

func (o *Option) join(option *Option) *Option {
	out := &Option{
		DeepCopy:   o.DeepCopy,
		FIFO:       o.FIFO,
		BufferSize: o.BufferSize,
		Injectable: o.Injectable,
		Metrics:    o.Metrics,
		Span:       o.Span,
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

	if option.Injectable != nil {
		out.Injectable = option.Injectable
	}

	if option.Metrics != nil {
		out.Metrics = option.Metrics
	}

	if option.Span != nil {
		out.Span = option.Span
	}

	return out
}

// Handler is a method for turning the ForkRule into an instance of Fork
func (r ForkRule) Handler(payload []*Packet) (t, f []*Packet) {
	t = []*Packet{}
	f = []*Packet{}

	for _, packet := range payload {
		if r(packet.Data) {
			t = append(t, packet)
		} else {
			f = append(f, packet)
		}
	}

	return t, f
}

func (out *edge) sendTo(ctx context.Context, in *edge) {
	go func() {
	Loop:
		for {
			select {
			case <-ctx.Done():
				break Loop
			case list := <-out.channel:
				if len(list) > 0 {
					in.channel <- list
				}
			}
		}
	}()
}

func newEdge(bufferSize *int) *edge {
	b := 0

	if bufferSize != nil {
		b = *bufferSize
	}

	return &edge{
		make(chan []*Packet, b),
	}
}

func boolP(v bool) *bool {
	return &v
}

func intP(v int) *int {
	return &v
}

func deepCopy(data []Data) []Data {
	out := []Data{}
	buf := &bytes.Buffer{}
	enc, dec := gob.NewEncoder(buf), gob.NewDecoder(buf)

	_ = enc.Encode(data)
	_ = dec.Decode(&out)

	return out
}
