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

	"github.com/karlseguin/typed"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"
)

var (
	// ForkDuplicate is a SplitHandler that sends data to both outputs
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

	// ForkError is a SplitHandler for splitting errors from successes
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
		FIFO:       boolP(false),
		Injectable: boolP(true),
		Metrics:    boolP(true),
		Span:       boolP(true),
		BufferSize: intP(0),
	}
)

// Data wrapper on typed.Typed
type Data typed.Typed

// Packet type that holds information traveling through the machine
type Packet struct {
	ID    string
	Data  Data
	Error error
	span  trace.Span
}

// Option type for holding machine settings
type Option struct {
	FIFO       *bool
	Injectable *bool
	BufferSize *int
	Span       *bool
	Metrics    *bool
}

// Retriever type for providing the data to flow into the system
type Retriever func(context.Context) chan []Data

// Applicative type for applying a change to a typed.Typed
type Applicative func(Data) error

// Fold type for folding a 2 Data into a single element
type Fold func(Data, Data) Data

// Fork func for splitting a payload into 2
type Fork func(list []*Packet) (a, b []*Packet)

// ForkRule provides a SplitHandler for splitting based on the return bool
type ForkRule func(Data) bool

// Sender type for sending data out of the system
type Sender func([]Data) error

type edge struct {
	channel chan []*Packet
}

func (p *Packet) apply(id string, a Applicative) {
	p.handleError(id, a(p.Data))
}

func (p *Packet) handleError(id string, err error) {
	if err != nil {
		p.Error = fmt.Errorf("%s %s %w", id, err.Error(), p.Error)
	}
}

func (p *Packet) newSpan(ctx context.Context, tracer trace.Tracer, name, vertexID, vertexType string) {
	_, span := tracer.Start(
		ctx,
		p.ID,
		trace.WithAttributes(
			label.String("vertex_id", vertexID),
			label.String("vertex_type", vertexType),
			label.String("packet_id", p.ID),
		),
	)
	p.span = span
	p.span.AddEvent(ctx, name,
		label.String("vertex_id", vertexID),
		label.String("vertex_type", vertexType),
		label.String("packet_id", p.ID),
	)
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
		FIFO:       o.FIFO,
		BufferSize: o.BufferSize,
		Injectable: o.Injectable,
		Metrics:    o.Metrics,
		Span:       o.Span,
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

// Handler func for providing a SplitHandler
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
