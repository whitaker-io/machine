// Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package machine

import (
	"context"
	"fmt"

	"github.com/karlseguin/typed"
	"github.com/mitchellh/copystructure"
)

// Builder type for creating and running a system of operations
type Builder struct {
	r *root
}

// Vertex type for applying a mutation to the data
type Vertex struct {
	id   string
	x    *vertex
	next *vertex
}

// Splitter type for controlling the flow of data through the system
type Splitter struct {
	id    string
	x     *vertex
	left  *vertex
	right *vertex
}

// Transmission type for sending data out of the system
type Transmission struct {
	id string
	x  *vertex
}

// ID func to return the ID for the system
func (m *Builder) ID() string {
	return m.r.id
}

// Run func for starting the system
func (m *Builder) Run(ctx context.Context, recorders ...func(string, string, string, []*Packet)) error {
	m.r.recorder = func(id, name string, state string, payload []*Packet) {
		if len(recorders) > 0 {
			out := []*Packet{}
			for _, v := range payload {
				x, _ := copystructure.Copy(v.Data)
				out = append(out, &Packet{
					ID:    v.ID,
					Data:  x.(typed.Typed),
					Error: v.Error,
				})
			}
			for _, recorder := range recorders {
				recorder(id, name, state, out)
			}
		}
	}
	return m.r.run(ctx)
}

// Inject func for injecting events into the system
func (m *Builder) Inject(ctx context.Context, events map[string][]*Packet) {
	m.r.inject(ctx, events)
}

// Then apply a mutation
func (m *Builder) Then(v *Vertex) *Builder {
	m.r.next = v.x
	return m
}

// Split the data
func (m *Builder) Split(r *Splitter) *Builder {
	m.r.next = r.x
	return m
}

// Transmit the data outside the system
func (m *Builder) Transmit(t *Transmission) *Builder {
	m.r.next = t.x
	return m
}

// Then apply a mutation
func (m *Vertex) Then(v *Vertex) *Vertex {
	m.next = v.x
	return m
}

// Split the data
func (m *Vertex) Split(r *Splitter) *Vertex {
	m.next = r.x
	return m
}

// Transmit the data outside the system
func (m *Vertex) Transmit(t *Transmission) *Vertex {
	m.next = t.x
	return m
}

// ThenLeft apply a mutation to the left side
func (m *Splitter) ThenLeft(left *Vertex) *Splitter {
	m.left = left.x
	return m
}

// SplitLeft split the data on the left
func (m *Splitter) SplitLeft(left *Splitter) *Splitter {
	m.left = left.x
	return m
}

// TransmitLeft the left side outside the system
func (m *Splitter) TransmitLeft(t *Transmission) *Splitter {
	m.left = t.x
	return m
}

// ThenRight apply a mutation to the right side
func (m *Splitter) ThenRight(right *Vertex) *Splitter {
	m.right = right.x
	return m
}

// SplitRight split the data on the right
func (m *Splitter) SplitRight(right *Splitter) *Splitter {
	m.right = right.x
	return m
}

// TransmitRight the right side outside the system
func (m *Splitter) TransmitRight(t *Transmission) *Splitter {
	m.right = t.x
	return m
}

// New func for providing an instance of Builder
func New(id string, r Retriever, options ...*Option) *Builder {
	b := &Builder{
		r: &root{
			id:        id,
			retrieve:  r,
			vertacies: map[string]*vertex{},
		},
	}

	b.r.option = defaultOptions.merge(options...)

	return b
}

// NewVertex func for providing an instance of Vertex
func NewVertex(id string, a Applicative) *Vertex {
	node := &Vertex{
		id: id,
	}
	edge := newEdge()

	node.x = &vertex{
		id:         id,
		vertexType: "applicative",
		metrics:    createMetrics(id, "applicative"),
		handler: func(payload []*Packet) {
			for _, packet := range payload {
				packet.apply(id, a)
			}

			edge.channel <- payload
		},
		connector: func(ctx context.Context, m *root) error {
			if node.next == nil {
				return fmt.Errorf("non-terminated node")
			}
			return node.next.cascade(ctx, m, edge)
		},
	}

	return node
}

// NewSplitter func for providing an instance of Splitter
func NewSplitter(id string, s SplitHandler) *Splitter {
	splitter := &Splitter{
		id: id,
	}

	leftEdge := newEdge()
	rightEdge := newEdge()

	splitter.x = &vertex{
		id:         id,
		vertexType: "splitter",
		metrics:    createMetrics(id, "splitter"),
		handler: func(payload []*Packet) {
			lpayload, rpayload := s(payload)
			leftEdge.channel <- lpayload
			rightEdge.channel <- rpayload
		},
		connector: func(ctx context.Context, m *root) error {
			if splitter.left == nil || splitter.right == nil {
				return fmt.Errorf("non-terminated router")
			} else if err := splitter.left.cascade(ctx, m, leftEdge); err != nil {
				return err
			} else if err := splitter.right.cascade(ctx, m, rightEdge); err != nil {
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
		id: id,
		x: &vertex{
			id:         id,
			vertexType: "sender",
			metrics:    createMetrics(id, "sender"),
			handler: func(payload []*Packet) {
				data := []typed.Typed{}
				for _, packet := range payload {
					data = append(data, packet.Data)
				}

				err := s(data)

				for _, packet := range payload {
					if err != nil {
						packet.handleError(id, err)
					}
				}
			},
			connector: func(ctx context.Context, m *root) error { return nil },
		},
	}
}
