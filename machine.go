// Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package machine

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/karlseguin/typed"

	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/metric"
	"go.opentelemetry.io/otel/label"
)

// Vertex interface for defining a child node
type vertex interface {
	cascade(ctx context.Context, output *channel, machine *root) error
}

// root execution graph for a system
type root struct {
	id       string
	initium  Initium
	nodes    map[string]*node
	next     vertex
	output   *channel
	recorder func(string, string, string, []*Packet)
	option   *Option
}

// node graph node for the Machine
type node struct {
	id        string
	processus Processus
	next      vertex
	input     *channel
}

// router graph node for the Machine
type router struct {
	id      string
	handler func([]*Packet) ([]*Packet, []*Packet)
	left    vertex
	right   vertex
	input   *channel
}

// termination graph leaf for the Machine
type termination struct {
	id       string
	terminus Terminus
	input    *channel
}

// run func to start the Machine
func (m *root) run(ctx context.Context) error {
	if m.next == nil {
		return fmt.Errorf("non-terminated machine")
	}

	return m.next.cascade(ctx, m.begin(ctx), m)
}

// inject func to inject the logs into the machine
func (m *root) inject(ctx context.Context, logs map[string][]*Packet) {
	if list, ok := logs[m.id]; ok {
		if *m.option.Span {
			tracer := global.Tracer("initium.inject")
			for _, packet := range list {
				packet.newSpan(ctx, tracer, "initium.inject", m.id, "initium")
			}
		}
		m.output.channel <- list
	}

	for node, payload := range logs {
		if node, ok := m.nodes[node]; ok {
			if *m.option.Span {
				tracer := global.Tracer("processus.inject")
				for _, packet := range payload {
					packet.newSpan(ctx, tracer, "processus.inject", node.id, "processus")
				}
			}
			node.inject(payload)
		}
	}
}

func (m *root) begin(ctx context.Context) *channel {
	meter := global.Meter("initium.begin")
	tracer := global.Tracer("initium.begin")
	labels := []label.KeyValue{label.String("vertex_id", m.id), label.String("vertex_type", "initium")}

	inCounter := metric.Must(meter).NewInt64ValueRecorder(m.id + ".incoming")
	outCounter := metric.Must(meter).NewInt64ValueRecorder(m.id + ".outgoing")
	inTotalCounter := metric.Must(meter).NewFloat64Counter(m.id + ".total.incoming")
	outTotalCounter := metric.Must(meter).NewFloat64Counter(m.id + ".total.outgoing")
	batchDuration := metric.Must(meter).NewInt64ValueRecorder(m.id + ".duration")

	m.output = newChannel(*m.option.BufferSize)

	input := m.initium(ctx)

	fn := func(data []typed.Typed) {
		if *m.option.Metrics {
			inCounter.Record(ctx, int64(len(data)), labels...)
			inTotalCounter.Add(ctx, float64(len(data)), labels...)
		}
		start := time.Now()

		payload := []*Packet{}
		for _, item := range data {
			packet := &Packet{
				ID:   uuid.New().String(),
				Data: item,
			}
			if *m.option.Span {
				packet.newSpan(ctx, tracer, "initium.begin", m.id, "initium")
			}
			payload = append(payload, packet)
		}

		duration := time.Since(start)
		m.recorder(m.id, "initium", "start", payload)
		m.output.channel <- payload
		m.recorder(m.id, "initium", "done", payload)

		if *m.option.Metrics {
			outCounter.Record(ctx, int64(len(payload)), labels...)
			outTotalCounter.Add(ctx, float64(len(payload)), labels...)
			batchDuration.Record(ctx, int64(duration), labels...)
		}
	}

	go func() {
	Loop:
		for {
			select {
			case <-ctx.Done():
				break Loop
			case data := <-input:
				if len(data) < 1 {
					continue
				}

				if *m.option.FIFO {
					fn(data)
				} else {
					go fn(data)
				}
			}
		}
	}()

	return m.output
}

func (pn *node) inject(payload []*Packet) {
	pn.input.channel <- payload
}

func (pn *node) cascade(ctx context.Context, output *channel, m *root) error {
	if pn.input != nil {
		output.sendTo(ctx, pn.input)
		return nil
	}

	m.nodes[pn.id] = pn
	pn.input = output

	out := newChannel(*m.option.BufferSize)

	fn := func(payload []*Packet) {
		for _, packet := range payload {
			packet.apply(pn.id, pn.processus)
		}

		out.channel <- payload
	}

	run(ctx, pn.id, "processus", fn, m, output)

	if pn.next == nil {
		return fmt.Errorf("non-terminated node")
	}

	return pn.next.cascade(ctx, out, m)
}

func (r *router) cascade(ctx context.Context, output *channel, m *root) error {
	if r.input != nil {
		output.sendTo(ctx, r.input)
		return nil
	}

	r.input = output

	left := newChannel(*m.option.BufferSize)
	right := newChannel(*m.option.BufferSize)

	fn := func(payload []*Packet) {
		lpayload, rpayload := r.handler(payload)

		if *m.option.Span {
			for _, packet := range lpayload {
				packet.span.AddEvent(ctx, "router-left",
					label.String("vertex_id", r.id),
					label.String("vertex_type", "router"),
					label.String("packet_id", packet.ID),
				)
			}

			for _, packet := range rpayload {
				packet.span.AddEvent(ctx, "router-right",
					label.String("vertex_id", r.id),
					label.String("vertex_type", "router"),
					label.String("packet_id", packet.ID),
				)
			}
		}

		left.channel <- lpayload
		right.channel <- rpayload
	}

	run(ctx, r.id, "router", fn, m, output)

	if r.left == nil || r.right == nil {
		return fmt.Errorf("non-terminated router")
	} else if err := r.left.cascade(ctx, left, m); err != nil {
		return err
	} else if err := r.right.cascade(ctx, right, m); err != nil {
		return err
	}

	return nil
}

func (c *termination) cascade(ctx context.Context, output *channel, m *root) error {
	if c.input != nil {
		output.sendTo(ctx, c.input)
		return nil
	}

	c.input = output

	runner := func(payload []*Packet) {
		data := []typed.Typed{}
		for _, packet := range payload {
			data = append(data, packet.Data)
		}

		err := c.terminus(data)

		for _, packet := range payload {
			if err != nil {
				packet.handleError(c.id, err)
			}
		}
	}

	run(ctx, c.id, "terminus", runner, m, c.input)

	return nil
}

func run(ctx context.Context, id, vertexType string, handler func([]*Packet), m *root, input *channel) {
	meter := global.Meter(vertexType)
	labels := []label.KeyValue{
		label.String("vertex_id", id),
		label.String("vertex_type", vertexType),
	}

	inCounter := metric.Must(meter).NewInt64ValueRecorder(id + ".incoming")
	outCounter := metric.Must(meter).NewInt64ValueRecorder(id + ".outgoing")
	errorsCounter := metric.Must(meter).NewInt64ValueRecorder(id + ".errors")
	inTotalCounter := metric.Must(meter).NewFloat64Counter(id + ".total.incoming")
	outTotalCounter := metric.Must(meter).NewFloat64Counter(id + ".total.outgoing")
	errorsTotalCounter := metric.Must(meter).NewFloat64Counter(id + ".total.errors")
	batchDuration := metric.Must(meter).NewInt64ValueRecorder(id + ".duration")

	runner := func(payload []*Packet) {
		if *m.option.Metrics {
			inCounter.Record(ctx, int64(len(payload)), labels...)
			inTotalCounter.Add(ctx, float64(len(payload)), labels...)
		}
		start := time.Now()

		if *m.option.Span {
			for _, packet := range payload {
				packet.span.AddEvent(ctx, "vertex",
					label.String("vertex_id", id),
					label.String("vertex_type", vertexType),
					label.String("packet_id", packet.ID),
					label.Int64("when", start.UnixNano()),
				)
			}
		}

		m.recorder(id, vertexType, "start", payload)
		handler(payload)
		m.recorder(id, vertexType, "done", payload)

		duration := time.Since(start)

		failures := 0

		if *m.option.Span {
			for _, packet := range payload {
				if packet.Error != nil {
					failures++
					packet.span.AddEvent(ctx, "error",
						label.String("vertex_id", id),
						label.String("vertex_type", vertexType),
						label.String("packet_id", packet.ID),
						label.Bool("error", packet.Error != nil),
					)
				}
			}
		}

		if *m.option.Metrics {
			outCounter.Record(ctx, int64(len(payload)), labels...)
			outTotalCounter.Add(ctx, float64(len(payload)), labels...)
			errorsCounter.Record(ctx, int64(failures), labels...)
			errorsTotalCounter.Add(ctx, float64(failures), labels...)
			batchDuration.Record(ctx, int64(duration), labels...)
		}
	}

	go do(ctx, *m.option.FIFO, runner, input.channel)
}

func do(ctx context.Context, fifo bool, runner func([]*Packet), channel chan []*Packet) {
	func() {
	Loop:
		for {
			select {
			case <-ctx.Done():
				break Loop
			case data := <-channel:
				if len(data) < 1 {
					continue
				}

				if fifo {
					runner(data)
				} else {
					go runner(data)
				}
			}
		}
	}()
}
