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

// Stream interface for Running and injecting data
type Stream interface {
	ID() string
	Run(ctx context.Context, recorders ...recorder) error
	Inject(ctx context.Context, events map[string][]*Packet)
	Builder() Builder
}

// Builder interface for plotting out the data flow of the system
type Builder interface {
	Map(id string, a Applicative) Builder
	FoldLeft(id string, f Fold) Builder
	FoldRight(id string, f Fold) Builder
	Fork(id string, f Fork) (Builder, Builder)
	Link(id, target string)
	Transmit(id string, s Sender)
}

type nexter func(*node) *node

type builder struct {
	vertex
	next      *node
	recorder  recorder
	option    *Option
	vertacies map[string]*vertex
}

type node struct {
	vertex
	next  *node
	left  *node
	right *node
}

// ID func to return the ID for the system
func (m *builder) ID() string {
	return m.id
}

// Run func for starting the system
func (m *builder) Run(ctx context.Context, recorders ...recorder) error {
	if m.next == nil {
		return fmt.Errorf("non-terminated builder")
	}

	if len(recorders) > 0 {
		m.recorder = mergeRecorders(recorders...)
	}

	return m.cascade(ctx, m.recorder, m.vertacies, m.option, m.input)
}

// Inject func for injecting events into the system
func (m *builder) Inject(ctx context.Context, events map[string][]*Packet) {
	for node, payload := range events {
		if v, ok := m.vertacies[node]; ok {
			if *m.option.Span {
				for _, packet := range payload {
					packet.newSpan(ctx, v.metrics.tracer, v.vertexType+".inject", v.id, v.vertexType)
				}
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

// Then apply a mutation
func (n nexter) Map(id string, x Applicative) Builder {
	next := &node{}
	edge := newEdge()

	next.vertex = vertex{
		id:         id,
		vertexType: "map",
		metrics:    createMetrics(id, "map"),
		handler: func(payload []*Packet) {
			for _, packet := range payload {
				packet.apply(id, x)
			}

			edge.channel <- payload
		},
		connector: func(ctx context.Context, r recorder, vertacies map[string]*vertex, option *Option) error {
			if next.next == nil {
				return fmt.Errorf("non-terminated map")
			}
			return next.next.cascade(ctx, r, vertacies, option, edge)
		},
	}

	next = n(next)

	return nexter(func(n *node) *node {
		next.next = n
		return n
	})
}

// FoldLeft the data
func (n nexter) FoldLeft(id string, x Fold) Builder {
	next := &node{}
	edge := newEdge()

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
		handler: func(payload []*Packet) {
			edge.channel <- []*Packet{fr(payload...)}
		},
		connector: func(ctx context.Context, r recorder, vertacies map[string]*vertex, option *Option) error {
			if next.next == nil {
				return fmt.Errorf("non-terminated fold")
			}
			return next.next.cascade(ctx, r, vertacies, option, edge)
		},
	}
	next = n(next)

	return nexter(func(n *node) *node {
		next.next = n
		return n
	})
}

// FoldRight the data
func (n nexter) FoldRight(id string, x Fold) Builder {
	next := &node{}
	edge := newEdge()

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
		handler: func(payload []*Packet) {
			edge.channel <- []*Packet{fr(payload...)}
		},
		connector: func(ctx context.Context, r recorder, vertacies map[string]*vertex, option *Option) error {
			if next.next == nil {
				return fmt.Errorf("non-terminated node")
			}
			return next.next.cascade(ctx, r, vertacies, option, edge)
		},
	}

	next = n(next)

	return nexter(func(n *node) *node {
		next.next = n
		return n
	})
}

// Fork the data
func (n nexter) Fork(id string, x Fork) (left, right Builder) {
	next := &node{}

	leftEdge := newEdge()
	rightEdge := newEdge()

	next.vertex = vertex{
		id:         id,
		vertexType: "fork",
		metrics:    createMetrics(id, "fork"),
		handler: func(payload []*Packet) {
			lpayload, rpayload := x(payload)
			leftEdge.channel <- lpayload
			rightEdge.channel <- rpayload
		},
		connector: func(ctx context.Context, r recorder, vertacies map[string]*vertex, option *Option) error {
			if next.left == nil || next.right == nil {
				return fmt.Errorf("non-terminated fork")
			} else if err := next.left.cascade(ctx, r, vertacies, option, leftEdge); err != nil {
				return err
			} else if err := next.right.cascade(ctx, r, vertacies, option, rightEdge); err != nil {
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

// Link the data to an existing operation creating a loop, target must be an ancestor
func (n nexter) Link(id, target string) {
	edge := newEdge()
	n(&node{
		vertex: vertex{
			id:         id,
			vertexType: "link",
			metrics:    createMetrics(id, "link"),
			handler:    func(payload []*Packet) { edge.channel <- payload },
			connector: func(ctx context.Context, r recorder, vertacies map[string]*vertex, option *Option) error {
				v, ok := vertacies[target]

				if !ok {
					return fmt.Errorf("invalid target - not in list of ancestors")
				}

				return v.cascade(ctx, r, vertacies, option, edge)
			},
		},
	})
}

// Transmit the data outside the system
func (n nexter) Transmit(id string, x Sender) {
	n(&node{
		vertex: vertex{
			id:         id,
			vertexType: "transmit",
			metrics:    createMetrics(id, "transmit"),
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
			connector: func(ctx context.Context, r recorder, vertacies map[string]*vertex, option *Option) error { return nil },
		},
	})
}

// NewStream func for providing a Stream
func NewStream(id string, retriever Retriever, options ...*Option) Stream {
	mtrx := createMetrics(id, "stream")
	edge := newEdge()
	input := newEdge()

	builder := &builder{
		vertex: vertex{
			id:         id,
			vertexType: "stream",
			metrics:    mtrx,
			handler: func(p []*Packet) {
				edge.channel <- p
			},
			input: input,
		},
		vertacies: map[string]*vertex{},
		option:    defaultOptions.merge(options...),
	}

	builder.connector = func(ctx context.Context, r recorder, vertacies map[string]*vertex, option *Option) error {
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
						packet := &Packet{
							ID:   uuid.New().String(),
							Data: item,
						}
						if *option.Span {
							packet.newSpan(ctx, mtrx.tracer, "stream.begin", id, "stream")
						}
						payload[i] = packet
					}

					input.channel <- payload
				}
			}
		}()
		return builder.next.cascade(ctx, r, vertacies, option, edge)
	}

	return builder
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
}
