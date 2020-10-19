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

// To func for sending the payload to a processor
func (m *Builder) To(v *VertexBuilder) *Builder {
	m.x.child = v.x
	return m
}

// RouteTo func for sending the payload to a router
func (m *Builder) RouteTo(r *RouterBuilder) *Builder {
	m.x.child = r.x
	return m
}

// Terminate func for sending the payload to a cap
func (m *Builder) Terminate(id, name string, fifo bool, t Terminus) *Builder {
	x := t.convert(id, name, fifo)

	m.x.child = x
	return m
}

// To func for sending the payload to a processor
func (m *VertexBuilder) To(v *VertexBuilder) *VertexBuilder {
	m.x.child = v.x
	return m
}

// RouteTo func for sending the payload to a router
func (m *VertexBuilder) RouteTo(r *RouterBuilder) *VertexBuilder {
	m.x.child = r.x
	return m
}

// Terminate func for sending the payload to a cap
func (m *VertexBuilder) Terminate(id, name string, fifo bool, t Terminus) *VertexBuilder {
	x := t.convert(id, name, fifo)

	m.x.child = x
	return m
}

// ToLeft func for sending the payload to a processor
func (m *RouterBuilder) ToLeft(left *VertexBuilder) *RouterBuilder {
	m.x.left = left.x
	return m
}

// RouteToLeft func for sending the payload to a router
func (m *RouterBuilder) RouteToLeft(left *RouterBuilder) *RouterBuilder {
	m.x.left = left.x
	return m
}

// TerminateLeft func for sending the payload to a cap
func (m *RouterBuilder) TerminateLeft(id, name string, fifo bool, t Terminus) *RouterBuilder {
	m.x.left = t.convert(id, name, fifo)
	return m
}

// ToRight func for sending the payload to a processor
func (m *RouterBuilder) ToRight(right *VertexBuilder) *RouterBuilder {
	m.x.right = right.x
	return m
}

// RouteToRight func for sending the payload to a router
func (m *RouterBuilder) RouteToRight(right *RouterBuilder) *RouterBuilder {
	m.x.right = right.x
	return m
}

// TerminateRight func for sending the payload to a cap
func (m *RouterBuilder) TerminateRight(id, name string, fifo bool, t Terminus) *RouterBuilder {
	m.x.right = t.convert(id, name, fifo)
	return m
}
