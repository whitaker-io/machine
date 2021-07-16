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
	"io"
	"net/http"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"github.com/whitaker-io/data"
)

// HTTPStream is a Stream that also provides a fiber.Handler for receiving data
type HTTPStream interface {
	Stream
	Handler() fiber.Handler
}

// Stream is a representation of a data stream and its associated logic.
// Creating a new Stream is handled by the appropriately named NewStream function.
//
// The Builder method is the entrypoint into creating the data processing flow.
// All branches of the Stream are required to end in either a Publish or
// a Link in order to be considered valid.
type Stream interface {
	ID() string
	Run(ctx context.Context, gracePeriod time.Duration, clusters ...Cluster) error
	InjectionCallback(ctx context.Context) func(logs ...*Log)
	Builder() Builder
	Errors() chan error
}

// Builder is the interface provided for creating a data processing stream.
type Builder interface {
	Map(id string, a Applicative) Builder
	FoldLeft(id string, f Fold) Builder
	FoldRight(id string, f Fold) Builder
	Fork(id string, f Fork) (Builder, Builder)
	Loop(id string, x Fork) (loop, out Builder)
	Publish(id string, s Publisher)
}

type nexter func(*node) *node

type httpStream struct {
	Stream
	handler fiber.Handler
}

type builder struct {
	errorChannel chan error
	vertex
	next      *node
	option    *Option
	clusters  []Cluster
	vertacies map[string]*vertex
}

type node struct {
	vertex
	loop  *node
	next  *node
	left  *node
	right *node
}

// ID is a method used to return the ID for the Stream
func (m *builder) ID() string {
	return m.id
}

// Run is the method used for starting the stream processing. It requires a context
// and an optional list of recorder functions. The recorder function has the signiture
// func(vertexID, vertexType, state string, paylaod []*Packet) and is called at the
// beginning of every vertex.
func (m *builder) Run(ctx context.Context, gracePeriod time.Duration, clusters ...Cluster) error {
	if m.next == nil {
		return fmt.Errorf("non-terminated builder")
	}

	for _, cluster := range clusters {
		if err := cluster.Join(m.id, m.InjectionCallback(ctx)); err != nil {
			return err
		}

		go func(c Cluster) {
		Loop:
			for {
				select {
				case <-ctx.Done():
					if err := c.Leave(m.id); err != nil {
						m.errorChannel <- err
					}
					break Loop
				default:
					<-time.After(gracePeriod)
				}
			}
		}(cluster)
	}

	m.clusters = clusters

	return m.cascade(ctx, m, m.input)
}

// Inject is a method for restarting work that has been dropped by the Stream
// typically in a distributed system setting. Though it can be used to side load
// data into the Stream to be processed
func (m *builder) InjectionCallback(ctx context.Context) func(logs ...*Log) {
	vType := trace.WithAttributes(attribute.String("vertex_type", "inject"))

	return func(logs ...*Log) {
		for _, log := range logs {
			if v, ok := m.vertacies[log.VertexID]; ok {
				payload := []*Packet{log.Packet}

				if !*m.option.Injectable {
					m.record(v.id, v.vertexType, "injection-denied", payload)
					continue
				}

				log.Packet.spanCtx, log.Packet.span = tracer.Start(ctx, v.id, vType)
				v.input.channel <- payload
			}
		}
	}
}

func (m *builder) Builder() Builder {
	return nexter(func(n *node) *node {
		m.next = n
		return n
	})
}

func (m *builder) Errors() chan error {
	return m.errorChannel
}

func (m *builder) record(vertexID, vertexType, state string, payload []*Packet) {
	if len(m.clusters) > 0 {
		out := []*Packet{}
		buf := &bytes.Buffer{}
		enc, dec := gob.NewEncoder(buf), gob.NewDecoder(buf)

		_ = enc.Encode(payload)
		_ = dec.Decode(&out)

		go func() {
			logs := make([]*Log, len(out))
			now := time.Now()
			for i, packet := range out {
				logs[i] = &Log{
					StreamID:   m.id,
					VertexID:   vertexID,
					VertexType: vertexType,
					State:      state,
					Packet:     packet,
					When:       now,
				}
			}

			for _, cluster := range m.clusters {
				cluster.Write(logs...)
			}
		}()
	}
}

// Map apply a mutation, options default to the set used when creating the Stream
func (n nexter) Map(id string, x Applicative) Builder {
	next := &node{}
	var edge *edge

	next.vertex = vertex{
		id:         id,
		vertexType: "map",
		handler: func(payload []*Packet) {
			for _, packet := range payload {
				packet.apply(id, x)
			}

			edge.channel <- payload
		},
		connector: func(ctx context.Context, b *builder) error {
			edge = newEdge(b.option.BufferSize)

			if next.loop != nil && next.next == nil {
				next.next = next.loop
			}

			if next.next == nil {
				return fmt.Errorf("non-terminated map")
			}
			return next.next.cascade(ctx, b, edge)
		},
	}

	next = n(next)

	return nexter(func(n *node) *node {
		n.loop = next.loop
		next.next = n
		return n
	})
}

// FoldLeft the data, options default to the set used when creating the Stream
func (n nexter) FoldLeft(id string, x Fold) Builder {
	next := &node{}
	var edge *edge

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
		handler: func(payload []*Packet) {
			edge.channel <- []*Packet{fr(payload...)}
		},
		connector: func(ctx context.Context, b *builder) error {
			edge = newEdge(b.option.BufferSize)

			if next.loop != nil && next.next == nil {
				next.next = next.loop
			}

			if next.next == nil {
				return fmt.Errorf("non-terminated fold")
			}
			return next.next.cascade(ctx, b, edge)
		},
	}
	next = n(next)

	return nexter(func(n *node) *node {
		n.loop = next.loop
		next.next = n
		return n
	})
}

// FoldRight the data, options default to the set used when creating the Stream
func (n nexter) FoldRight(id string, x Fold) Builder {
	next := &node{}
	var edge *edge

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
		handler: func(payload []*Packet) {
			edge.channel <- []*Packet{fr(payload...)}
		},
		connector: func(ctx context.Context, b *builder) error {
			edge = newEdge(b.option.BufferSize)

			if next.loop != nil && next.next == nil {
				next.next = next.loop
			}

			if next.next == nil {
				return fmt.Errorf("non-terminated node")
			}
			return next.next.cascade(ctx, b, edge)
		},
	}

	next = n(next)

	return nexter(func(n *node) *node {
		n.loop = next.loop
		next.next = n
		return n
	})
}

// Fork the data, options default to the set used when creating the Stream
func (n nexter) Fork(id string, x Fork) (left, right Builder) {
	next := &node{}

	var leftEdge *edge
	var rightEdge *edge

	next.vertex = vertex{
		id:         id,
		vertexType: "fork",
		handler: func(payload []*Packet) {
			lpayload, rpayload := x(payload)
			leftEdge.channel <- lpayload
			rightEdge.channel <- rpayload
		},
		connector: func(ctx context.Context, b *builder) error {
			leftEdge = newEdge(b.option.BufferSize)
			rightEdge = newEdge(b.option.BufferSize)

			if next.loop != nil && next.left == nil {
				next.left = next.loop
			}

			if next.loop != nil && next.right == nil {
				next.right = next.loop
			}

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
			n.loop = next.loop
			next.left = n
			return n
		}), nexter(func(n *node) *node {
			n.loop = next.loop
			next.right = n
			return n
		})
}

// Publish the data outside the system, options default to the set used when creating the Stream
func (n nexter) Publish(id string, x Publisher) {
	v := vertex{
		id:         id,
		vertexType: "publish",
		connector:  func(ctx context.Context, b *builder) error { return nil },
	}

	v.handler = func(payload []*Packet) {
		d := make([]data.Data, len(payload))
		for i, packet := range payload {
			d[i] = packet.Data
		}

		if err := x.Send(d); err != nil {
			v.errorHandler(&Error{
				Err:        fmt.Errorf("publish %w", err),
				VertexID:   id,
				VertexType: "publish",
				Packets:    payload,
				Time:       time.Now(),
			})
		}

		for _, packet := range payload {
			if packet.span != nil {
				packet.span.End()
			}
		}
	}

	n(&node{vertex: v})
}

// Loop the data combining a fork and link the first output is the Builder for the loop
// and the second is the output of the loop
func (n nexter) Loop(id string, x Fork) (loop, out Builder) {
	next := &node{}

	var leftEdge *edge
	var rightEdge *edge

	next.vertex = vertex{
		id:         id,
		vertexType: "loop",
		handler: func(payload []*Packet) {
			lpayload, rpayload := x(payload)
			leftEdge.channel <- lpayload
			rightEdge.channel <- rpayload
		},
		connector: func(ctx context.Context, b *builder) error {
			leftEdge = newEdge(b.option.BufferSize)
			rightEdge = newEdge(b.option.BufferSize)

			if next.loop != nil && next.right == nil {
				next.right = next.loop
			}

			if next.left == nil || next.right == nil {
				return fmt.Errorf("non-terminated loop")
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
			n.loop = next
			next.left = n
			return n
		}), nexter(func(n *node) *node {
			n.loop = next.loop
			next.right = n
			return n
		})
}

func (hs *httpStream) Handler() fiber.Handler {
	return hs.handler
}

// NewStream is a function for creating a new Stream. It takes an id, a Retriever function,
// and a list of Options that can override the defaults and set new defaults for the
// subsequent vertices in the Stream.
func NewStream(id string, retriever Retriever, options ...*Option) Stream {
	opt := defaultOptions.merge(options...)
	vType := trace.WithAttributes(attribute.String("vertex_type", "stream"))

	edge := newEdge(opt.BufferSize)
	input := newEdge(opt.BufferSize)

	x := &builder{
		errorChannel: make(chan error, 10000),
		option:       opt,
		vertex: vertex{
			id:         id,
			vertexType: "stream",
			input:      input,
			handler: func(p []*Packet) {
				edge.channel <- p
			},
		},
		vertacies: map[string]*vertex{},
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
						packet := &Packet{
							ID:   uuid.NewString(),
							Data: item,
						}
						packet.spanCtx, packet.span = tracer.Start(ctx, id, vType)

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

// NewHTTPStream a method that creates a Stream which takes in data
// through a fiber.Handler
func NewHTTPStream(id string, opts ...*Option) HTTPStream {
	channel := make(chan []data.Data)

	return &httpStream{
		handler: func(ctx *fiber.Ctx) error {
			payload := []data.Data{}
			packet := data.Data{}

			if err := ctx.BodyParser(&packet); err == nil {
				payload = []data.Data{packet}
			} else if err := ctx.BodyParser(&payload); err != nil {
				return ctx.SendStatus(http.StatusBadRequest)
			}

			channel <- deepCopy(payload)

			return ctx.SendStatus(http.StatusAccepted)
		},
		Stream: NewStream(id,
			func(ctx context.Context) chan []data.Data {
				return channel
			},
			opts...,
		),
	}
}

// NewWebsocketStream a method that creates a Stream which takes in data
// through a fiber.Handler that runs a websocket
func NewWebsocketStream(id string, opts ...*Option) HTTPStream {
	channel := make(chan []data.Data)

	acceptedMessage := map[string]interface{}{
		"message": "OK",
		"status":  http.StatusAccepted,
	}

	badMessage := map[string]interface{}{
		"message": "error bad type",
		"status":  http.StatusBadRequest,
	}

	wsHandler := websocket.New(func(c *websocket.Conn) {
		payload := []data.Data{}

		for {
			var err error
			for err = c.ReadJSON(&payload); err == io.ErrUnexpectedEOF; {
				<-time.After(10 * time.Millisecond)
			}

			if err != nil {
				if err := c.WriteJSON(badMessage); err != nil {
					break
				}
			}

			channel <- deepCopy(payload)

			if err := c.WriteJSON(acceptedMessage); err != nil {
				break
			}
		}
	})

	return &httpStream{
		handler: func(c *fiber.Ctx) error {
			if websocket.IsWebSocketUpgrade(c) {
				return wsHandler(c)
			}
			return fiber.ErrUpgradeRequired
		},
		Stream: NewStream(id,
			func(ctx context.Context) chan []data.Data {
				return channel
			},
			opts...,
		),
	}
}

// NewSubscriptionStream creates a Stream from the provider Subscription and pulls data
// continuously after an interval amount of time
func NewSubscriptionStream(id string, sub Subscription, interval time.Duration, opts ...*Option) Stream {
	channel := make(chan []data.Data)

	return NewStream(id,
		func(ctx context.Context) chan []data.Data {
			go func() {
			Loop:
				for {
					select {
					case <-ctx.Done():
						sub.Close()
						break Loop
					case <-time.After(interval):
						channel <- sub.Read(ctx)
					}
				}
			}()

			return channel
		},
		opts...,
	)
}

func init() {
	gob.Register([]*Packet{})
	gob.Register([]data.Data{})
}
