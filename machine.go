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

	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/metric"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"
)

// Vertex interface for defining a child node
type vertex interface {
	cascade(ctx context.Context, output *channel, machine *Machine) error
}

// Machine execution graph for a system
type Machine struct {
	info
	initium    Initium
	nodes      map[string]*node
	child      vertex
	output     *channel
	bufferSize int
	recorder   func(string, string, string, []*Packet)
}

// node graph node for the Machine
type node struct {
	info
	processus Processus
	child     vertex
	input     *channel
}

// router graph node for the Machine
type router struct {
	info
	handler func([]*Packet) ([]*Packet, []*Packet)
	left    vertex
	right   vertex
	input   *channel
}

// termination graph leaf for the Machine
type termination struct {
	info
	terminus Terminus
	input    *channel
}

type info struct {
	id   string
	name string
	fifo bool
}

// ID func to return the ID
func (m *Machine) ID() string {
	return m.id
}

// Run func to start the Machine
func (m *Machine) Run(ctx context.Context) error {
	if m.child == nil {
		return fmt.Errorf("non-terminated machine")
	}

	return m.child.cascade(ctx, m.begin(ctx), m)
}

// Inject func to inject the logs into the machine
func (m *Machine) Inject(ctx context.Context, logs map[string][]*Packet) {
	if list, ok := logs[m.id]; ok {
		m.output.channel <- list
	}

	for node, list := range logs {
		if node, ok := m.nodes[node]; ok {
			node.inject(ctx, list)
		}
	}
}

func (m *Machine) begin(ctx context.Context) *channel {
	meterName := fmt.Sprintf("machine.%s", m.id)
	meter := global.Meter(meterName)
	tracer := global.Tracer(meterName)
	labels := []label.KeyValue{label.String("id", m.id), label.String("type", m.name)}

	inCounter := metric.Must(meter).NewInt64ValueRecorder(meterName + ".incoming")
	outCounter := metric.Must(meter).NewInt64ValueRecorder(meterName + ".outgoing")
	inTotalCounter := metric.Must(meter).NewFloat64Counter(meterName + ".total.incoming")
	outTotalCounter := metric.Must(meter).NewFloat64Counter(meterName + ".total.outgoing")
	batchDuration := metric.Must(meter).NewInt64ValueRecorder(meterName + ".duration")

	m.output = newChannel(m.bufferSize)

	input := m.initium(ctx)

	fn := func(data []map[string]interface{}) {
		inCounter.Record(ctx, int64(len(data)), labels...)
		inTotalCounter.Add(ctx, float64(len(data)), labels...)

		start := time.Now()

		payload := []*Packet{}
		for _, item := range data {
			_, span := tracer.Start(
				ctx,
				m.name,
				trace.WithAttributes(labels...),
			)
			packet := &Packet{
				ID:   uuid.New().String(),
				Data: item,
				span: span,
			}
			packet.span.AddEvent(ctx, m.name+"-start",
				label.String("id", m.id),
				label.String("name", m.name),
				label.Bool("error", packet.Error != nil),
			)
			payload = append(payload, packet)
		}

		duration := time.Since(start)
		m.recorder(m.id, m.name, "start", payload)
		m.output.channel <- payload
		m.recorder(m.id, m.name, "done", payload)

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

				if m.fifo {
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
			pn.name,
			trace.WithAttributes(
				label.String("id", pn.id),
				label.String("name", pn.name),
			),
		)
		packet.span = span
		packet.span.AddEvent(ctx, pn.name+"-inject",
			label.String("id", pn.id),
			label.String("name", pn.name),
			label.Bool("error", packet.Error != nil),
		)
	}
	pn.input.channel <- payload
}

func (pn *node) cascade(ctx context.Context, output *channel, m *Machine) error {
	if pn.input != nil {
		output.sendTo(ctx, pn.input)
		return nil
	}

	m.nodes[pn.id] = pn
	pn.input = output

	out := newChannel(m.bufferSize)

	fn := func(payload []*Packet) {
		for _, packet := range payload {
			packet.apply(pn.id, pn.processus)

			packet.span.AddEvent(ctx, pn.name,
				label.String("id", pn.id),
				label.String("name", pn.name),
				label.Bool("error", packet.Error != nil),
			)
		}

		out.channel <- payload
	}

	run(ctx, pn.id, pn.name, pn.fifo, fn, m, output)

	if pn.child == nil {
		return fmt.Errorf("non-terminated node")
	}

	return pn.child.cascade(ctx, out, m)
}

func (r *router) cascade(ctx context.Context, output *channel, m *Machine) error {
	if r.input != nil {
		output.sendTo(ctx, r.input)
		return nil
	}

	r.input = output

	left := newChannel(m.bufferSize)
	right := newChannel(m.bufferSize)

	fn := func(payload []*Packet) {
		lpayload, rpayload := r.handler(payload)

		for _, packet := range lpayload {
			packet.span.AddEvent(ctx, r.name+"-left",
				label.String("id", r.id),
				label.String("name", r.name),
				label.Bool("error", packet.Error != nil),
			)
		}

		for _, packet := range rpayload {
			packet.span.AddEvent(ctx, r.name+"-right",
				label.String("id", r.id),
				label.String("name", r.name),
				label.Bool("error", packet.Error != nil),
			)
		}

		left.channel <- lpayload
		right.channel <- rpayload
	}

	run(ctx, r.id, "route", r.fifo, fn, m, output)

	if r.left == nil || r.right == nil {
		return fmt.Errorf("non-terminated router")
	} else if err := r.left.cascade(ctx, left, m); err != nil {
		return err
	} else if err := r.right.cascade(ctx, right, m); err != nil {
		return err
	}

	return nil
}

func (c *termination) cascade(ctx context.Context, output *channel, m *Machine) error {
	if c.input != nil {
		output.sendTo(ctx, c.input)
		return nil
	}

	c.input = output

	runner := func(payload []*Packet) {
		m.recorder(c.id, c.name, "start", payload)

		if len(payload) < 1 {
			return
		}

		data := []map[string]interface{}{}
		for _, packet := range payload {
			data = append(data, packet.Data)
		}

		err := c.terminus(data)
		m.recorder(c.id, c.name, "end", payload)

		for _, packet := range payload {
			if err != nil {
				packet.handleError(c.id, err)
				packet.span.AddEvent(ctx, c.name+"-error",
					label.String("id", c.id),
					label.String("name", c.name),
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
				if c.fifo {
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

func run(ctx context.Context, id, name string, fifo bool, r func([]*Packet), m *Machine, output *channel) {
	meterName := fmt.Sprintf("machine.%s", id)
	meter := global.Meter(meterName)
	labels := []label.KeyValue{label.String("id", id), label.String("type", name)}

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

		m.recorder(id, name, "start", payload)
		r(payload)
		m.recorder(id, name, "done", payload)

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
				if fifo {
					runner(list)
				} else {
					go runner(list)
				}
			}
		}
	}()
}
