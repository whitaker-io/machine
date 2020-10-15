package machine

import "context"

// Initium type for providing the data to flow into the system
type Initium func(context.Context) chan []map[string]interface{}

// Processus type for applying a change to a context
type Processus func(map[string]interface{}) error

// RouteHandler func for splitting a payload into 2
type RouteHandler func(list []*Packet) (t, f []*Packet)

// RouterRule type for validating a context at the beginning of a Machine
type RouterRule func(map[string]interface{}) bool

// RouterError type for splitting errors from successes
type RouterError struct{}

// RouterDuplicate type for splitting errors from successes
type RouterDuplicate struct{}

// Terminus type for ending a chain and returning an error if exists
type Terminus func([]map[string]interface{}) error

type inChannel struct {
	channel chan []*Packet
}

type outChannel struct {
	channel chan []*Packet
}

// Machine func for providing a Machine
func (i Initium) convert(id, name string, fifo bool, recorder Recorder) *Machine {
	return &Machine{
		info: info{
			id:       id,
			name:     name,
			fifo:     fifo,
			recorder: recorder,
		},
		initium: i,
	}
}

// Convert func for providing a Cap
func (p Processus) convert(id, name string, fifo bool) *node {
	return &node{
		info: info{
			id:   id,
			name: name,
			fifo: fifo,
		},
		processus: p,
	}
}

func (r RouteHandler) convert(id, name string, fifo bool) *router {
	return &router{
		info: info{
			id:   id,
			name: name,
			fifo: fifo,
		},
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

// Handler func for providing a RouteHandler
func (e RouterError) Handler(payload []*Packet) (s, f []*Packet) {
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

// Handler func for providing a RouteHandler
func (d RouterDuplicate) Handler(payload []*Packet) (a, b []*Packet) {
	a = []*Packet{}
	b = []*Packet{}

	for _, packet := range payload {
		a = append(a, packet)
		b = append(b, packet)
	}

	return a, b
}

// Convert func for providing a Cap
func (t Terminus) convert(id, name string, fifo bool) vertex {
	return &termination{
		info: info{
			id:   id,
			name: name,
			fifo: fifo,
		},
		terminus: t,
		input:    newInChannel(),
	}
}

func newInChannel() *inChannel {
	return &inChannel{
		make(chan []*Packet),
	}
}

func newOutChannel() *outChannel {
	return &outChannel{
		make(chan []*Packet),
	}
}

func (out *outChannel) convert() *inChannel {
	return &inChannel{
		channel: out.channel,
	}
}

func (out *outChannel) sendTo(ctx context.Context, in *inChannel) {
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
