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

	"github.com/google/uuid"
)

// Stream is a representation of a data stream and its associated logic.
// It may be used individually or hosted by a Pipe. Creating a new Stream
// is handled by the appropriately named NewStream function.
//
// The Builder method is the entrypoint into creating the data processing flow.
// All branches of the Stream are required to end in either a Transmit or
// a Link in order to be considered valid.
type Stream interface {
	ID() string
	Run(ctx context.Context, recorders ...recorder) error
	Inject(ctx context.Context, events map[string][]*Packet)
	Builder() Builder
}

// Builder is the interface provided for creating a data processing stream.
type Builder interface {
	Map(id string, a Applicative, options ...*Option) Builder
	FoldLeft(id string, f Fold, options ...*Option) Builder
	FoldRight(id string, f Fold, options ...*Option) Builder
	Fork(id string, f Fork, options ...*Option) (Builder, Builder)
	Link(id, target string, options ...*Option)
	Transmit(id string, s Sender, options ...*Option)
}

type nexter func(*node) *node

type builder struct {
	vertex
	next      *node
	recorder  recorder
	vertacies map[string]*vertex
}

type node struct {
	vertex
	next  *node
	left  *node
	right *node
}

// ID is a method used to return the ID for the system
func (m *builder) ID() string {
	return m.id
}

// Run is the method used for starting the stream processing. It requires a context
// and an optional list of recorder functions. The recorder function has the signiture
// func(vertexID, vertexType, state string, paylaod []*Packet) and is called at the
// beginning of every vertex.
func (m *builder) Run(ctx context.Context, recorders ...recorder) error {
	if m.next == nil {
		return fmt.Errorf("non-terminated builder")
	}

	if len(recorders) > 0 {
		m.recorder = mergeRecorders(recorders...)
	}

	return m.cascade(ctx, m, m.input)
}

// Inject is a method for restarting work that has been dropped by the Stream
// typically in a distributed system setting. Though it can be used to side load
// data into the Stream to be processed
func (m *builder) Inject(ctx context.Context, events map[string][]*Packet) {
	for node, payload := range events {
		if v, ok := m.vertacies[node]; ok {
			if *m.option.Span {
				for _, packet := range payload {
					packet.newSpan(ctx, v.metrics.tracer, v.vertexType+".inject", v.id, v.vertexType)
				}
			}

			if !*v.option.Injectable {
				m.recorder(v.id, v.vertexType, "injection-denied", payload)
				continue
			}

			v.input.channel <- payload
		}
	}
}

func (m *builder) Builder() Builder {
	return nexter(func(n *node) *node {
		m.next = n
		return n
	})
}

// Map apply a mutation, options default to the set used when creating the Stream
func (n nexter) Map(id string, x Applicative, options ...*Option) Builder {
	opt := &Option{
		BufferSize: intP(0),
	}

	if len(options) > 0 {
		opt = opt.merge(options...)
	}

	next := &node{}
	edge := newEdge(opt.BufferSize)

	next.vertex = vertex{
		id:         id,
		vertexType: "map",
		metrics:    createMetrics(id, "map"),
		option:     opt,
		handler: func(payload []*Packet) {
			for _, packet := range payload {
				packet.apply(id, x)
			}

			edge.channel <- payload
		},
		connector: func(ctx context.Context, b *builder) error {
			if next.next == nil {
				return fmt.Errorf("non-terminated map")
			}
			return next.next.cascade(ctx, b, edge)
		},
	}

	next = n(next)

	return nexter(func(n *node) *node {
		next.next = n
		return n
	})
}

// FoldLeft the data, options default to the set used when creating the Stream
func (n nexter) FoldLeft(id string, x Fold, options ...*Option) Builder {
	opt := &Option{
		BufferSize: intP(0),
	}

	if len(options) > 0 {
		opt = opt.merge(options...)
	}

	next := &node{}
	edge := newEdge(opt.BufferSize)

	fr := func(payload ...*Packet) *Packet {
		if len(payload) == 1 {
			return payload[0]
		}

		d := payload[0]

		for i := 1; i < len(payload); i++ {
			d.Data = x(d.Data, payload[i].Data)
		}

		return d
	}

	next.vertex = vertex{
		id:         id,
		vertexType: "fold",
		metrics:    createMetrics(id, "fold"),
		option:     opt,
		handler: func(payload []*Packet) {
			edge.channel <- []*Packet{fr(payload...)}
		},
		connector: func(ctx context.Context, b *builder) error {
			if next.next == nil {
				return fmt.Errorf("non-terminated fold")
			}
			return next.next.cascade(ctx, b, edge)
		},
	}
	next = n(next)

	return nexter(func(n *node) *node {
		next.next = n
		return n
	})
}

// FoldRight the data, options default to the set used when creating the Stream
func (n nexter) FoldRight(id string, x Fold, options ...*Option) Builder {
	opt := &Option{
		BufferSize: intP(0),
	}

	if len(options) > 0 {
		opt = opt.merge(options...)
	}

	next := &node{}
	edge := newEdge(opt.BufferSize)

	var fr func(...*Packet) *Packet
	fr = func(payload ...*Packet) *Packet {
		if len(payload) == 1 {
			return payload[0]
		}

		payload[len(payload)-1].Data = x(payload[0].Data, fr(payload[1:]...).Data)

		return payload[len(payload)-1]
	}

	next.vertex = vertex{
		id:         id,
		vertexType: "fold",
		metrics:    createMetrics(id, "fold"),
		option:     opt,
		handler: func(payload []*Packet) {
			edge.channel <- []*Packet{fr(payload...)}
		},
		connector: func(ctx context.Context, b *builder) error {
			if next.next == nil {
				return fmt.Errorf("non-terminated node")
			}
			return next.next.cascade(ctx, b, edge)
		},
	}

	next = n(next)

	return nexter(func(n *node) *node {
		next.next = n
		return n
	})
}

// Fork the data, options default to the set used when creating the Stream
func (n nexter) Fork(id string, x Fork, options ...*Option) (left, right Builder) {
	opt := &Option{
		BufferSize: intP(0),
	}

	if len(options) > 0 {
		opt = opt.merge(options...)
	}

	next := &node{}

	leftEdge := newEdge(opt.BufferSize)
	rightEdge := newEdge(opt.BufferSize)

	next.vertex = vertex{
		id:         id,
		vertexType: "fork",
		metrics:    createMetrics(id, "fork"),
		option:     opt,
		handler: func(payload []*Packet) {
			lpayload, rpayload := x(payload)
			leftEdge.channel <- lpayload
			rightEdge.channel <- rpayload
		},
		connector: func(ctx context.Context, b *builder) error {
			if next.left == nil || next.right == nil {
				return fmt.Errorf("non-terminated fork")
			} else if err := next.left.cascade(ctx, b, leftEdge); err != nil {
				return err
			} else if err := next.right.cascade(ctx, b, rightEdge); err != nil {
				return err
			}

			return nil
		},
	}

	next = n(next)

	return nexter(func(n *node) *node {
			next.left = n
			return n
		}), nexter(func(n *node) *node {
			next.right = n
			return n
		})
}

// Link the data to an existing operation creating a loop,
// target must be an ancestor, options default to the set
// used when creating the Stream
func (n nexter) Link(id, target string, options ...*Option) {
	opt := &Option{
		BufferSize: intP(0),
	}

	if len(options) > 0 {
		opt = opt.merge(options...)
	}

	edge := newEdge(opt.BufferSize)
	n(&node{
		vertex: vertex{
			id:         id,
			vertexType: "link",
			metrics:    createMetrics(id, "link"),
			handler:    func(payload []*Packet) { edge.channel <- payload },
			option:     opt,
			connector: func(ctx context.Context, b *builder) error {
				v, ok := b.vertacies[target]

				if !ok {
					return fmt.Errorf("invalid target - not in list of ancestors")
				}

				return v.cascade(ctx, b, edge)
			},
		},
	})
}

// Transmit the data outside the system, options default to the set used when creating the Stream
func (n nexter) Transmit(id string, x Sender, options ...*Option) {
	opt := &Option{
		BufferSize: intP(0),
	}

	if len(options) > 0 {
		opt = opt.merge(options...)
	}

	n(&node{
		vertex: vertex{
			id:         id,
			vertexType: "transmit",
			metrics:    createMetrics(id, "transmit"),
			option:     opt,
			handler: func(payload []*Packet) {
				data := make([]Data, len(payload))
				for i, packet := range payload {
					data[i] = packet.Data
				}

				if err := x(data); err != nil {
					for _, packet := range payload {
						packet.handleError(id, err)
					}
				}
			},
			connector: func(ctx context.Context, b *builder) error { return nil },
		},
	})
}

// NewStream is a function for creating a new Stream. It takes an id, a Retriever function,
// and a list of Options that can override the defaults and set new defaults for the
// subsequent vertices in the Stream.
func NewStream(id string, retriever Retriever, options ...*Option) Stream {
	mtrx := createMetrics(id, "stream")
	opt := defaultOptions.merge(options...)

	edge := newEdge(opt.BufferSize)
	input := newEdge(opt.BufferSize)

	x := &builder{
		vertex: vertex{
			id:         id,
			vertexType: "stream",
			metrics:    mtrx,
			input:      input,
			option:     opt,
			handler: func(p []*Packet) {
				edge.channel <- p
			},
		},
		vertacies: map[string]*vertex{},
		recorder:  func(s1, s2, s3 string, p []*Packet) {},
	}

	x.connector = func(ctx context.Context, b *builder) error {
		i := retriever(ctx)

		go func() {
		Loop:
			for {
				select {
				case <-ctx.Done():
					break Loop
				case data := <-i:
					if len(data) < 1 {
						continue
					}

					payload := make([]*Packet, len(data))
					for i, item := range data {
						var id string

						if val, ok := item["__traceID"]; ok && *opt.TraceID {
							if strval, ok := val.(string); ok {
								id = strval
							}
						}

						if id == "" {
							id = uuid.New().String()
						}

						packet := &Packet{
							ID:   id,
							Data: item,
						}
						if *x.option.Span {
							packet.newSpan(ctx, mtrx.tracer, "stream.begin", id, "stream")
						}
						payload[i] = packet
					}

					input.channel <- payload
				}
			}
		}()
		return x.next.cascade(ctx, x, edge)
	}

	return x
}

func mergeRecorders(recorders ...recorder) recorder {
	return func(id, name string, state string, payload []*Packet) {
		out := []*Packet{}
		buf := &bytes.Buffer{}
		enc, dec := gob.NewEncoder(buf), gob.NewDecoder(buf)

		_ = enc.Encode(payload)
		_ = dec.Decode(&out)

		for _, r := range recorders {
			r(id, name, state, out)
		}
	}
}

func init() {
	gob.Register([]*Packet{})
	gob.Register([]Data{})
}
