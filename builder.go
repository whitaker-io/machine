package machine

// Builder builder type for starting a machine
type Builder struct {
	x *Machine
}

// VertexBuilder builder type for adding a processor to the machine
type VertexBuilder struct {
	m *Builder
	x *node
}

// RouterBuilder builder type for adding a router to the machine
type RouterBuilder struct {
	m *Builder
	x *router
}

// New func for providing an instance of MachineBuilder
func New(id, name string, fifo bool, i Initium, recorder func(string, string, []*Packet)) *Builder {
	return &Builder{
		x: i.convert(id, name, fifo, recorder),
	}
}

// Build func for providing the underlying machine
func (m *Builder) Build() *Machine {
	return m.x
}

// Then func for sending the payload to a processor
func (m *Builder) Then(id, name string, fifo bool, p Processus) *VertexBuilder {
	x := p.convert(id, name, fifo)

	m.x.child = x
	m.x.nodes = map[string]*node{id: x}

	return &VertexBuilder{
		m: m,
		x: x,
	}
}

// Route func for sending the payload to a router
func (m *Builder) Route(id, name string, fifo bool, r RouteHandler) *RouterBuilder {
	x := r.convert(id, name, fifo)

	m.x.child = x

	return &RouterBuilder{
		m: m,
		x: x,
	}
}

// Terminate func for sending the payload to a cap
func (m *Builder) Terminate(id, name string, fifo bool, t Terminus) {
	x := t.convert(id, name, fifo)

	m.x.child = x
}

// To func for sending the payload to a processor
func (m *VertexBuilder) To(v *VertexBuilder) *VertexBuilder {
	m.x.child = v.x
	return v
}

// RouteTo func for sending the payload to a router
func (m *VertexBuilder) RouteTo(r *RouterBuilder) *RouterBuilder {
	m.x.child = r.x
	return r
}

// Then func for sending the payload to a processor
func (m *VertexBuilder) Then(id, name string, fifo bool, p Processus) *VertexBuilder {
	x := p.convert(id, name, fifo)

	m.x.child = x
	m.m.x.nodes[id] = x

	return &VertexBuilder{
		m: m.m,
		x: x,
	}
}

// Route func for sending the payload to a router
func (m *VertexBuilder) Route(id, name string, fifo bool, r RouteHandler) *RouterBuilder {
	x := r.convert(id, name, fifo)

	m.x.child = x

	return &RouterBuilder{
		m: m.m,
		x: x,
	}
}

// Terminate func for sending the payload to a cap
func (m *VertexBuilder) Terminate(id, name string, fifo bool, t Terminus) {
	x := t.convert(id, name, fifo)

	m.x.child = x
}

// ToLeft func for sending the payload to a processor
func (m *RouterBuilder) ToLeft(v *VertexBuilder) *VertexBuilder {
	m.x.left = v.x
	return v
}

// RouteToLeft func for sending the payload to a router
func (m *RouterBuilder) RouteToLeft(r *RouterBuilder) *RouterBuilder {
	m.x.left = r.x
	return r
}

// ThenLeft func for sending the payload to a processor
func (m *RouterBuilder) ThenLeft(id, name string, fifo bool, p Processus) *VertexBuilder {
	x := p.convert(id, name, fifo)

	m.x.left = x
	m.m.x.nodes[id] = x

	return &VertexBuilder{
		m: m.m,
		x: x,
	}
}

// RouteLeft func for sending the payload to a router
func (m *RouterBuilder) RouteLeft(id, name string, fifo bool, r RouteHandler) *RouterBuilder {
	x := r.convert(id, name, fifo)

	m.x.left = x

	return &RouterBuilder{
		m: m.m,
		x: x,
	}
}

// TerminateLeft func for sending the payload to a cap
func (m *RouterBuilder) TerminateLeft(id, name string, fifo bool, t Terminus) {
	x := t.convert(id, name, fifo)

	m.x.left = x
}

// ToRight func for sending the payload to a processor
func (m *RouterBuilder) ToRight(v *VertexBuilder) *VertexBuilder {
	m.x.left = v.x
	return v
}

// RouteToRight func for sending the payload to a router
func (m *RouterBuilder) RouteToRight(r *RouterBuilder) *RouterBuilder {
	m.x.left = r.x
	return r
}

// ThenRight func for sending the payload to a processor
func (m *RouterBuilder) ThenRight(id, name string, fifo bool, p Processus) *VertexBuilder {
	x := p.convert(id, name, fifo)

	m.x.right = x
	m.m.x.nodes[id] = x

	return &VertexBuilder{
		m: m.m,
		x: x,
	}
}

// RouteRight func for sending the payload to a router
func (m *RouterBuilder) RouteRight(id, name string, fifo bool, r RouteHandler) *RouterBuilder {
	x := r.convert(id, name, fifo)

	m.x.right = x

	return &RouterBuilder{
		m: m.m,
		x: x,
	}
}

// TerminateRight func for sending the payload to a cap
func (m *RouterBuilder) TerminateRight(id, name string, fifo bool, t Terminus) {
	x := t.convert(id, name, fifo)

	m.x.right = x
}
