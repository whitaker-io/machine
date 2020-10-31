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
	"go.opentelemetry.io/otel/api/trace"
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
		meterName := fmt.Sprintf("machine.%s", m.id)
		tracer := global.Tracer(meterName)
		for _, packet := range list {
			_, span := tracer.Start(
				ctx,
				packet.ID,
				trace.WithAttributes(
					label.String("machine_id", m.id),
					label.String("packet_id", packet.ID),
				),
			)
			packet.span = span
			packet.span.AddEvent(ctx, m.id+"-inject",
				label.String("machine_id", m.id),
				label.String("packet_id", packet.ID),
				label.Bool("error", packet.Error != nil),
			)
		}
		m.output.channel <- list
	}

	for node, list := range logs {
		if node, ok := m.nodes[node]; ok {
			node.inject(ctx, list)
		}
	}
}

func (m *root) begin(ctx context.Context) *channel {
	meterName := fmt.Sprintf("machine.%s", m.id)
	meter := global.Meter(meterName)
	tracer := global.Tracer(meterName)
	labels := []label.KeyValue{label.String("id", m.id), label.String("type", "initium")}

	inCounter := metric.Must(meter).NewInt64ValueRecorder(meterName + ".incoming")
	outCounter := metric.Must(meter).NewInt64ValueRecorder(meterName + ".outgoing")
	inTotalCounter := metric.Must(meter).NewFloat64Counter(meterName + ".total.incoming")
	outTotalCounter := metric.Must(meter).NewFloat64Counter(meterName + ".total.outgoing")
	batchDuration := metric.Must(meter).NewInt64ValueRecorder(meterName + ".duration")

	m.output = newChannel(*m.option.BufferSize)

	input := m.initium(ctx)

	fn := func(data []typed.Typed) {
		inCounter.Record(ctx, int64(len(data)), labels...)
		inTotalCounter.Add(ctx, float64(len(data)), labels...)

		start := time.Now()

		payload := []*Packet{}
		for _, item := range data {
			id := uuid.New().String()
			_, span := tracer.Start(
				ctx,
				id,
				trace.WithAttributes(
					label.String("machine_id", m.id),
					label.String("packet_id", id),
				),
			)
			packet := &Packet{
				ID:   id,
				Data: item,
				span: span,
			}
			packet.span.AddEvent(ctx, m.id+"-start",
				label.String("machine_id", m.id),
				label.String("packet_id", packet.ID),
				label.Bool("error", packet.Error != nil),
			)
			payload = append(payload, packet)
		}

		duration := time.Since(start)
		m.recorder(m.id, "initium", "start", payload)
		m.output.channel <- payload
		m.recorder(m.id, "initium", "done", payload)

		outCounter.Record(ctx, int64(len(payload)), labels...)
		outTotalCounter.Add(ctx, float64(len(payload)), labels...)
		batchDuration.Record(ctx, int64(duration), labels...)
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

func (pn *node) inject(ctx context.Context, payload []*Packet) {
	meterName := fmt.Sprintf("node.inject.%s", pn.id)
	tracer := global.Tracer(meterName)
	for _, packet := range payload {
		_, span := tracer.Start(
			ctx,
			packet.ID,
			trace.WithAttributes(
				label.String("node_id", pn.id),
				label.String("packet_id", packet.ID),
			),
		)
		packet.span = span
		packet.span.AddEvent(ctx, pn.id+"-inject",
			label.String("node_id", pn.id),
			label.String("packet_id", packet.ID),
			label.Bool("error", packet.Error != nil),
		)
	}
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

			packet.span.AddEvent(ctx, packet.ID,
				label.String("node_id", pn.id),
				label.String("packet_id", packet.ID),
				label.Bool("error", packet.Error != nil),
			)
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

		for _, packet := range lpayload {
			packet.span.AddEvent(ctx, "router-left",
				label.String("router_id", r.id),
				label.String("packet_id", packet.ID),
				label.Bool("error", packet.Error != nil),
			)
		}

		for _, packet := range rpayload {
			packet.span.AddEvent(ctx, "router-right",
				label.String("router_id", r.id),
				label.String("packet_id", packet.ID),
				label.Bool("error", packet.Error != nil),
			)
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
		m.recorder(c.id, "terminus", "start", payload)

		if len(payload) < 1 {
			return
		}

		data := []typed.Typed{}
		for _, packet := range payload {
			data = append(data, packet.Data)
		}

		err := c.terminus(data)
		m.recorder(c.id, "terminus", "end", payload)

		for _, packet := range payload {
			if err != nil {
				packet.handleError(c.id, err)
				packet.span.AddEvent(ctx, "terminus-error",
					label.String("terminus_id", c.id),
					label.String("packet_id", packet.ID),
					label.Bool("error", packet.Error != nil),
				)
			}
			packet.span.End()
		}
	}

	go func() {
	Loop:
		for {
			select {
			case <-ctx.Done():
				break Loop
			case list := <-c.input.channel:
				if *m.option.FIFO {
					runner(list)
				} else {
					go runner(list)
				}
			}
		}
	}()

	output.sendTo(ctx, c.input)

	return nil
}

func run(ctx context.Context, id, vertexType string, r func([]*Packet), m *root, output *channel) {
	meterName := fmt.Sprintf("machine.%s", id)
	meter := global.Meter(meterName)
	labels := []label.KeyValue{label.String("id", id), label.String("type", vertexType)}

	inCounter := metric.Must(meter).NewInt64ValueRecorder(meterName + ".incoming")
	outCounter := metric.Must(meter).NewInt64ValueRecorder(meterName + ".outgoing")
	errorsCounter := metric.Must(meter).NewInt64ValueRecorder(meterName + ".errors")
	inTotalCounter := metric.Must(meter).NewFloat64Counter(meterName + ".total.incoming")
	outTotalCounter := metric.Must(meter).NewFloat64Counter(meterName + ".total.outgoing")
	errorsTotalCounter := metric.Must(meter).NewFloat64Counter(meterName + ".total.errors")
	batchDuration := metric.Must(meter).NewInt64ValueRecorder(meterName + ".duration")

	runner := func(payload []*Packet) {
		if len(payload) < 1 {
			return
		}

		inCounter.Record(ctx, int64(len(payload)), labels...)
		inTotalCounter.Add(ctx, float64(len(payload)), labels...)

		start := time.Now()

		m.recorder(id, vertexType, "start", payload)
		r(payload)
		m.recorder(id, vertexType, "done", payload)

		duration := time.Since(start)

		failures := 0

		for _, packet := range payload {
			if packet.Error != nil {
				failures++
			}
		}

		outCounter.Record(ctx, int64(len(payload)), labels...)
		outTotalCounter.Add(ctx, float64(len(payload)), labels...)
		errorsCounter.Record(ctx, int64(failures), labels...)
		errorsTotalCounter.Add(ctx, float64(failures), labels...)
		batchDuration.Record(ctx, int64(duration), labels...)
	}

	go func() {
	Loop:
		for {
			select {
			case <-ctx.Done():
				break Loop
			case list := <-output.channel:
				if *m.option.FIFO {
					runner(list)
				} else {
					go runner(list)
				}
			}
		}
	}()
}
