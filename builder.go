package machine

// Builder builder type for starting a machine
type Builder struct {
	x *Machine
}

// VertexBuilder builder type for adding a processor to the machine
type VertexBuilder struct {
	x *node
}

// RouterBuilder builder type for adding a router to the machine
type RouterBuilder struct {
	x *router
}

// New func for providing an instance of Builder
func New(id, name string, fifo bool, i Initium) *Builder {
	return &Builder{
		x: i.convert(id, name, fifo, nil),
	}
}

// NewVertex func for providing an instance of VertexBuilder
func NewVertex(id, name string, fifo bool, p Processus) *VertexBuilder {
	return &VertexBuilder{
		x: p.convert(id, name, fifo),
	}
}

// NewRouter func for providing an instance of RouterBuilder
func NewRouter(id, name string, fifo bool, r RouteHandler) *RouterBuilder {
	return &RouterBuilder{
		x: r.convert(id, name, fifo),
	}
}

// Build func for providing the underlying machine
func (m *Builder) Build(recorders ...func(string, string, []*Packet)) *Machine {
	m.x.recorder = func(id, name string, payload []*Packet) {
		for _, recorder := range recorders {
			recorder(id, name, payload)
		}
	}
	return m.x
}

// Then func for sending the payload to a processor
func (m *Builder) Then(v *VertexBuilder) *Builder {
	m.x.child = v.x
	return m
}

// Route func for sending the payload to a router
func (m *Builder) Route(r *RouterBuilder) *Builder {
	m.x.child = r.x
	return m
}

// Terminate func for sending the payload to a cap
func (m *Builder) Terminate(id, name string, fifo bool, t Terminus) *Builder {
	x := t.convert(id, name, fifo)

	m.x.child = x
	return m
}

// Then func for sending the payload to a processor
func (m *VertexBuilder) Then(v *VertexBuilder) *VertexBuilder {
	m.x.child = v.x
	return m
}

// Route func for sending the payload to a router
func (m *VertexBuilder) Route(r *RouterBuilder) *VertexBuilder {
	m.x.child = r.x
	return m
}

// Terminate func for sending the payload to a cap
func (m *VertexBuilder) Terminate(id, name string, fifo bool, t Terminus) *VertexBuilder {
	x := t.convert(id, name, fifo)

	m.x.child = x
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
func (m *RouterBuilder) TerminateLeft(id, name string, fifo bool, t Terminus) *RouterBuilder {
	m.x.left = t.convert(id, name, fifo)
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
func (m *RouterBuilder) TerminateRight(id, name string, fifo bool, t Terminus) *RouterBuilder {
	m.x.right = t.convert(id, name, fifo)
	return m
}
