// Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package machine

import (
	"context"
	"fmt"

	"github.com/karlseguin/typed"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"
)

var (
	// RouterDuplicate is a RouteHandler that sends data to both outputs
	RouterDuplicate RouteHandler = func(payload []*Packet) (a, b []*Packet) {
		a = []*Packet{}
		b = []*Packet{}

		for _, packet := range payload {
			a = append(a, packet)
			b = append(b, packet)
		}

		return a, b
	}

	// RouterError is a RouteHandler for splitting errors from successes
	RouterError RouteHandler = func(payload []*Packet) (s, f []*Packet) {
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
		Idempotent: boolP(false),
		Metrics:    boolP(true),
		Span:       boolP(true),
		BufferSize: intP(0),
	}
)

// Packet type that holds information traveling through the machine
type Packet struct {
	ID    string
	Data  typed.Typed
	Error error
	span  trace.Span
}

// Option type for holding machine settings
type Option struct {
	FIFO       *bool
	Idempotent *bool
	BufferSize *int
	Span       *bool
	Metrics    *bool
}

// Initium type for providing the data to flow into the system
type Initium func(context.Context) chan []typed.Typed

// Processus type for applying a change to a context
type Processus func(typed.Typed) error

// RouteHandler func for splitting a payload into 2
type RouteHandler func(list []*Packet) (a, b []*Packet)

// RouterRule type for validating a context at the beginning of a Machine
type RouterRule func(typed.Typed) bool

// Terminus type for ending a chain and returning an error if exists
type Terminus func([]typed.Typed) error

type channel struct {
	channel chan []*Packet
}

func (c *Packet) apply(id string, p func(typed.Typed) error) {
	c.handleError(id, p(c.Data))
}

func (c *Packet) handleError(id string, err error) {
	if err != nil {
		c.Error = fmt.Errorf("%s %s %w", id, err.Error(), c.Error)
	}
}

func (c *Packet) newSpan(ctx context.Context, tracer trace.Tracer, name, vertexID, vertexType string) {
	_, span := tracer.Start(
		ctx,
		c.ID,
		trace.WithAttributes(
			label.String("vertex_id", vertexID),
			label.String("vertex_type", vertexType),
			label.String("packet_id", c.ID),
		),
	)
	c.span = span
	c.span.AddEvent(ctx, name,
		label.String("vertex_id", vertexID),
		label.String("vertex_type", vertexType),
		label.String("packet_id", c.ID),
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
		Idempotent: o.Idempotent,
		Metrics:    o.Metrics,
		Span:       o.Span,
	}

	if option.FIFO != nil {
		out.FIFO = option.FIFO
	}

	if option.BufferSize != nil {
		out.BufferSize = option.BufferSize
	}

	if option.Idempotent != nil {
		out.Idempotent = option.Idempotent
	}

	if option.Metrics != nil {
		out.Metrics = option.Metrics
	}

	if option.Span != nil {
		out.Span = option.Span
	}

	return out
}

// Machine func for providing a Machine
func (i Initium) convert(id string) *root {
	return &root{
		id:      id,
		initium: i,
		nodes:   map[string]*node{},
	}
}

// Convert func for providing a Cap
func (p Processus) convert(id string) *node {
	return &node{
		id:        id,
		processus: p,
	}
}

func (r RouteHandler) convert(id string) *router {
	return &router{
		id:      id,
		handler: r,
	}
}

// Handler func for providing a RouteHandler
func (r RouterRule) Handler(payload []*Packet) (t, f []*Packet) {
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

// Convert func for providing a Cap
func (t Terminus) convert(id string) vertex {
	return &termination{
		id:       id,
		terminus: t,
	}
}

func newChannel(bufferSize int) *channel {
	return &channel{
		make(chan []*Packet, bufferSize),
	}
}

func (out *channel) sendTo(ctx context.Context, in *channel) {
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

func boolP(v bool) *bool {
	return &v
}

func intP(v int) *int {
	return &v
}
