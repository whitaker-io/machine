package machine

// TypeRule - rule
const TypeRule = "rule"

// TypeErrorRouter - error_router
const TypeErrorRouter = "error_router"

// RouteHandler func for splitting a payload into 2
type RouteHandler func(list Payload) (t, f Payload)

// RouterRule type for validating a context at the beginning of a Machine
type RouterRule func(map[string]interface{}) bool

// RouterError type for splitting errors from successes
type RouterError struct{}

func (r RouteHandler) convert(id, name string, fifo bool) *router {
	return &router{
		labels: labels{
			id:   id,
			name: name,
			fifo: fifo,
		},
		handler: r,
	}
}

// Handler func for providing a RouteHandler
func (r RouterRule) Handler(list Payload) (t, f Payload) {
	t = Payload{}
	f = Payload{}

	for _, ctx := range list {
		if r(ctx.Data) {
			t = append(t, ctx)
		} else {
			f = append(f, ctx)
		}
	}

	return t, f
}

// Handler func for providing a RouteHandler
func (e RouterError) Handler(list Payload) (s, f Payload) {
	s = Payload{}
	f = Payload{}

	for _, ctx := range list {
		if ctx.Error != nil {
			f = append(f, ctx)
		} else {
			s = append(s, ctx)
		}
	}

	return s, f
}
