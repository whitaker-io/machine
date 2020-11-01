// Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package machine

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/mitchellh/copystructure"
)

// Builder type for creating and running a system of operations
type Builder struct {
	vertex
	recorder
	next      *vertex
	option    *Option
	vertacies map[string]*vertex
}

// Vertex type for applying a mutation to the data
type Vertex struct {
	vertex
	next *vertex
}

// Splitter type for controlling the flow of data through the system
type Splitter struct {
	vertex
	left  *vertex
	right *vertex
}

// Transmission type for sending data out of the system
type Transmission struct {
	vertex
}

// ID func to return the ID for the system
func (m *Builder) ID() string {
	return m.id
}

// Run func for starting the system
func (m *Builder) Run(ctx context.Context, recorders ...func(string, string, string, []*Packet)) error {
	if m.next == nil {
		return fmt.Errorf("non-terminated builder")
	}

	if len(recorders) > 0 {
		m.recorder = func(id, name string, state string, payload []*Packet) {
			out := make([]*Packet, len(payload))
			for i, v := range payload {
				x, _ := copystructure.Copy(v.Data)
				out[i] = &Packet{
					ID:    v.ID,
					Data:  x.(Data),
					Error: v.Error,
				}

				for _, recorder := range recorders {
					recorder(id, name, state, out)
				}
			}
		}
	}
	return m.cascade(ctx, m.recorder, m.vertacies, m.option, m.input)
}

// Inject func for injecting events into the system
func (m *Builder) Inject(ctx context.Context, events map[string][]*Packet) {
	if payload, ok := events[m.id]; ok {
		if *m.option.Span {
			for _, packet := range payload {
				packet.newSpan(ctx, m.next.metrics.tracer, "retriever.inject", m.id, "retriever")
			}
		}
		m.next.input.channel <- payload
	}

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

// Then apply a mutation
func (m *Builder) Then(v *Vertex) *Builder {
	m.next = &v.vertex
	return m
}

// Split the data
func (m *Builder) Split(r *Splitter) *Builder {
	m.next = &r.vertex
	return m
}

// Transmit the data outside the system
func (m *Builder) Transmit(t *Transmission) *Builder {
	m.next = &t.vertex
	return m
}

// Then apply a mutation
func (m *Vertex) Then(v *Vertex) *Vertex {
	m.next = &v.vertex
	return m
}

// Split the data
func (m *Vertex) Split(r *Splitter) *Vertex {
	m.next = &r.vertex
	return m
}

// Transmit the data outside the system
func (m *Vertex) Transmit(t *Transmission) *Vertex {
	m.next = &t.vertex
	return m
}

// ThenLeft apply a mutation to the left side
func (m *Splitter) ThenLeft(left *Vertex) *Splitter {
	m.left = &left.vertex
	return m
}

// SplitLeft split the data on the left
func (m *Splitter) SplitLeft(left *Splitter) *Splitter {
	m.left = &left.vertex
	return m
}

// TransmitLeft the left side outside the system
func (m *Splitter) TransmitLeft(t *Transmission) *Splitter {
	m.left = &t.vertex
	return m
}

// ThenRight apply a mutation to the right side
func (m *Splitter) ThenRight(right *Vertex) *Splitter {
	m.right = &right.vertex
	return m
}

// SplitRight split the data on the right
func (m *Splitter) SplitRight(right *Splitter) *Splitter {
	m.right = &right.vertex
	return m
}

// TransmitRight the right side outside the system
func (m *Splitter) TransmitRight(t *Transmission) *Splitter {
	m.right = &t.vertex
	return m
}

// New func for providing an instance of Builder
func New(id string, retriever Retriever, options ...*Option) *Builder {
	mtrx := createMetrics(id, "retriever")
	edge := newEdge()
	input := newEdge()

	builder := &Builder{
		vertex: vertex{
			id:         id,
			vertexType: "retriever",
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
							packet.newSpan(ctx, mtrx.tracer, "retriever.begin", id, "retriever")
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

// NewVertex func for providing an instance of Vertex
func NewVertex(id string, a Applicative) *Vertex {
	node := &Vertex{}
	edge := newEdge()

	node.vertex = vertex{
		id:         id,
		vertexType: "applicative",
		metrics:    createMetrics(id, "applicative"),
		handler: func(payload []*Packet) {
			for _, packet := range payload {
				packet.apply(id, a)
			}

			edge.channel <- payload
		},
		connector: func(ctx context.Context, r recorder, vertacies map[string]*vertex, option *Option) error {
			if node.next == nil {
				return fmt.Errorf("non-terminated node")
			}
			return node.next.cascade(ctx, r, vertacies, option, edge)
		},
	}

	return node
}

// NewSplitter func for providing an instance of Splitter
func NewSplitter(id string, s SplitHandler) *Splitter {
	splitter := &Splitter{}

	leftEdge := newEdge()
	rightEdge := newEdge()

	splitter.vertex = vertex{
		id:         id,
		vertexType: "splitter",
		metrics:    createMetrics(id, "splitter"),
		handler: func(payload []*Packet) {
			lpayload, rpayload := s(payload)
			leftEdge.channel <- lpayload
			rightEdge.channel <- rpayload
		},
		connector: func(ctx context.Context, r recorder, vertacies map[string]*vertex, option *Option) error {
			if splitter.left == nil || splitter.right == nil {
				return fmt.Errorf("non-terminated router")
			} else if err := splitter.left.cascade(ctx, r, vertacies, option, leftEdge); err != nil {
				return err
			} else if err := splitter.right.cascade(ctx, r, vertacies, option, rightEdge); err != nil {
				return err
			}

			return nil
		},
	}

	return splitter
}

// NewTransmission func for providing an instance of Transmission
func NewTransmission(id string, s Sender) *Transmission {
	return &Transmission{
		vertex: vertex{
			id:         id,
			vertexType: "sender",
			metrics:    createMetrics(id, "sender"),
			handler: func(payload []*Packet) {
				data := make([]Data, len(payload))
				for i, packet := range payload {
					data[i] = packet.Data
				}

				if err := s(data); err != nil {
					for _, packet := range payload {
						packet.handleError(id, err)
					}
				}
			},
			connector: func(ctx context.Context, r recorder, vertacies map[string]*vertex, option *Option) error { return nil },
		},
	}
}
