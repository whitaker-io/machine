// Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package machine

import (
	"context"

	"github.com/mitchellh/copystructure"
)

// Builder builder type for starting a machine
type Builder struct {
	x *root
}

// VertexBuilder builder type for adding a processor to the machine
type VertexBuilder struct {
	x *node
}

// RouterBuilder builder type for adding a router to the machine
type RouterBuilder struct {
	x *router
}

// TerminationBuilder builder type for adding a termination to the machine
type TerminationBuilder struct {
	x vertex
}

// New func for providing an instance of Builder
func New(id string, i Initium, options ...*Option) *Builder {
	b := &Builder{
		x: i.convert(id),
	}

	b.x.option = defaultOptions.merge(options...)

	return b
}

// NewVertex func for providing an instance of VertexBuilder
func NewVertex(id string, p Processus) *VertexBuilder {
	return &VertexBuilder{
		x: p.convert(id),
	}
}

// NewRouter func for providing an instance of RouterBuilder
func NewRouter(id string, r RouteHandler) *RouterBuilder {
	return &RouterBuilder{
		x: r.convert(id),
	}
}

// NewTermination func for providing an instance of TerminationBuilder
func NewTermination(id string, t Terminus) *TerminationBuilder {
	return &TerminationBuilder{
		x: t.convert(id),
	}
}

// ID func to return the ID for the machine
func (m *Builder) ID() string {
	return m.x.id
}

// Run func for starting the machine
func (m *Builder) Run(ctx context.Context, recorders ...func(string, string, string, []*Packet)) error {
	m.x.recorder = func(id, name string, state string, payload []*Packet) {
		if len(recorders) > 0 {
			out := []*Packet{}
			for _, v := range payload {
				x, _ := copystructure.Copy(v.Data)
				out = append(out, &Packet{
					ID:    v.ID,
					Data:  x.(map[string]interface{}),
					Error: v.Error,
				})
			}
			for _, recorder := range recorders {
				recorder(id, name, state, out)
			}
		}
	}
	return m.x.run(ctx)
}

// Inject func for injecting events into the machine
func (m *Builder) Inject(ctx context.Context, events map[string][]*Packet) {
	m.x.inject(ctx, events)
}

// Then func for sending the payload to a processor
func (m *Builder) Then(v *VertexBuilder) *Builder {
	m.x.next = v.x
	return m
}

// Route func for sending the payload to a router
func (m *Builder) Route(r *RouterBuilder) *Builder {
	m.x.next = r.x
	return m
}

// Terminate func for sending the payload to a cap
func (m *Builder) Terminate(t *TerminationBuilder) *Builder {
	m.x.next = t.x
	return m
}

// Then func for sending the payload to a processor
func (m *VertexBuilder) Then(v *VertexBuilder) *VertexBuilder {
	m.x.next = v.x
	return m
}

// Route func for sending the payload to a router
func (m *VertexBuilder) Route(r *RouterBuilder) *VertexBuilder {
	m.x.next = r.x
	return m
}

// Terminate func for sending the payload to a cap
func (m *VertexBuilder) Terminate(t *TerminationBuilder) *VertexBuilder {
	m.x.next = t.x
	return m
}

// ThenLeft func for sending the payload to a processor
func (m *RouterBuilder) ThenLeft(left *VertexBuilder) *RouterBuilder {
	m.x.left = left.x
	return m
}

// RouteLeft func for sending the payload to a router
func (m *RouterBuilder) RouteLeft(left *RouterBuilder) *RouterBuilder {
	m.x.left = left.x
	return m
}

// TerminateLeft func for sending the payload to a cap
func (m *RouterBuilder) TerminateLeft(t *TerminationBuilder) *RouterBuilder {
	m.x.left = t.x
	return m
}

// ThenRight func for sending the payload to a processor
func (m *RouterBuilder) ThenRight(right *VertexBuilder) *RouterBuilder {
	m.x.right = right.x
	return m
}

// RouteRight func for sending the payload to a router
func (m *RouterBuilder) RouteRight(right *RouterBuilder) *RouterBuilder {
	m.x.right = right.x
	return m
}

// TerminateRight func for sending the payload to a cap
func (m *RouterBuilder) TerminateRight(t *TerminationBuilder) *RouterBuilder {
	m.x.right = t.x
	return m
}
