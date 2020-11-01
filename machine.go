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
	"go.opentelemetry.io/otel/label"
)

type handler func([]*Packet)
type recorder func(string, string, string, []*Packet)

// root execution graph for a system
type root struct {
	id        string
	retrieve  Retriever
	next      *vertex
	option    *Option
	vertacies map[string]*vertex
	recorder
}

type vertex struct {
	id         string
	vertexType string
	input      *edge
	handler
	connector func(ctx context.Context, m *root) error
	metrics   *metrics
}

type metrics struct {
	labels             []label.KeyValue
	inCounter          metric.Int64ValueRecorder
	outCounter         metric.Int64ValueRecorder
	errorsCounter      metric.Int64ValueRecorder
	inTotalCounter     metric.Float64Counter
	outTotalCounter    metric.Float64Counter
	errorsTotalCounter metric.Float64Counter
	batchDuration      metric.Int64ValueRecorder
}

// run func to start the Machine
func (m *root) run(ctx context.Context) error {
	if m.next == nil {
		return fmt.Errorf("non-terminated machine")
	}

	input := m.retrieve(ctx)
	edge := newEdge()

	v := &vertex{
		id:         m.id,
		vertexType: "retriever",
		metrics:    createMetrics(m.id, "retriever"),
		handler: func(payload []*Packet) {
			edge.channel <- payload
		},
		connector: func(ctx context.Context, m *root) error {
			return m.next.cascade(ctx, m, edge)
		},
		input: newEdge(),
	}

	tracer := global.Tracer(v.vertexType + ".begin")

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

				payload := []*Packet{}
				for _, item := range data {
					packet := &Packet{
						ID:   uuid.New().String(),
						Data: item,
					}
					if *m.option.Span {
						packet.newSpan(ctx, tracer, v.vertexType+"begin", m.id, v.vertexType)
					}
					payload = append(payload, packet)
				}

				v.input.channel <- payload
			}
		}
	}()

	h := m.recorder.wrap(v.id, v.vertexType, v.run(ctx, m.option))

	do(ctx, *m.option.FIFO, h, v.input)

	return v.connector(ctx, m)
}

// inject func to inject the logs into the machine
func (m *root) inject(ctx context.Context, logs map[string][]*Packet) {
	if payload, ok := logs[m.id]; ok {
		if *m.option.Span {
			tracer := global.Tracer("retriever.inject")
			for _, packet := range payload {
				packet.newSpan(ctx, tracer, "retriever.inject", m.id, "retriever")
			}
		}
		m.next.inject(payload)
	}

	for node, payload := range logs {
		if v, ok := m.vertacies[node]; ok {
			if *m.option.Span {
				tracer := global.Tracer(v.vertexType + ".inject")
				for _, packet := range payload {
					packet.newSpan(ctx, tracer, v.vertexType+".inject", v.id, v.vertexType)
				}
			}
			v.inject(payload)
		}
	}
}

func (v *vertex) inject(payload []*Packet) {
	v.input.channel <- payload
}

func (v *vertex) cascade(ctx context.Context, m *root, input *edge) error {
	if v.input != nil {
		input.sendTo(ctx, v.input)
		return nil
	}

	v.input = input

	h := m.recorder.wrap(v.id, v.vertexType, v.run(ctx, m.option))

	do(ctx, *m.option.FIFO, h, input)

	m.vertacies[v.id] = v

	return v.connector(ctx, m)
}

func (v *vertex) run(ctx context.Context, option *Option) handler {
	h := v.metrics.wrap(ctx, v.handler, option)
	return func(payload []*Packet) {
		start := time.Now()

		if *option.Span {
			for _, packet := range payload {
				packet.span.AddEvent(ctx, "vertex",
					label.String("vertex_id", v.id),
					label.String("vertex_type", v.vertexType),
					label.String("packet_id", packet.ID),
					label.Int64("when", start.UnixNano()),
				)
			}
		}

		h(payload)

		if *option.Span {
			for _, packet := range payload {
				if packet.Error != nil {
					packet.span.AddEvent(ctx, "error",
						label.String("vertex_id", v.id),
						label.String("vertex_type", v.vertexType),
						label.String("packet_id", packet.ID),
						label.Bool("error", packet.Error != nil),
					)
				}
				if v.vertexType == "sender" {
					packet.span.End()
				}
			}
		}
	}
}

func (mtrx *metrics) begin(ctx context.Context, payload []*Packet, option *Option) {
	if *option.Metrics {
		mtrx.inCounter.Record(ctx, int64(len(payload)), mtrx.labels...)
		mtrx.inTotalCounter.Add(ctx, float64(len(payload)), mtrx.labels...)
	}
}

func (mtrx *metrics) end(ctx context.Context, failures int, duration time.Duration, payload []*Packet, option *Option) {
	if *option.Metrics {
		mtrx.outCounter.Record(ctx, int64(len(payload)), mtrx.labels...)
		mtrx.outTotalCounter.Add(ctx, float64(len(payload)), mtrx.labels...)
		mtrx.errorsCounter.Record(ctx, int64(failures), mtrx.labels...)
		mtrx.errorsTotalCounter.Add(ctx, float64(failures), mtrx.labels...)
		mtrx.batchDuration.Record(ctx, int64(duration), mtrx.labels...)
	}
}

func (mtrx *metrics) wrap(ctx context.Context, h handler, option *Option) handler {
	return func(payload []*Packet) {
		mtrx.begin(ctx, payload, option)
		start := time.Now()
		h(payload)
		duration := time.Since(start)
		failures := 0
		for _, packet := range payload {
			if packet.Error != nil {
				failures++
			}
		}
		mtrx.end(ctx, failures, duration, payload, option)
	}
}

func createMetrics(id, vertexType string) *metrics {
	meter := global.Meter(id)
	return &metrics{
		labels: []label.KeyValue{
			label.String("vertex_id", id),
			label.String("vertex_type", vertexType),
		},
		inTotalCounter:     metric.Must(meter).NewFloat64Counter(vertexType + "." + id + ".total.incoming"),
		outTotalCounter:    metric.Must(meter).NewFloat64Counter(vertexType + "." + id + ".total.outgoing"),
		errorsTotalCounter: metric.Must(meter).NewFloat64Counter(vertexType + "." + id + ".total.errors"),
		inCounter:          metric.Must(meter).NewInt64ValueRecorder(vertexType + "." + id + ".incoming"),
		outCounter:         metric.Must(meter).NewInt64ValueRecorder(vertexType + "." + id + ".outgoing"),
		errorsCounter:      metric.Must(meter).NewInt64ValueRecorder(vertexType + "." + id + ".errors"),
		batchDuration:      metric.Must(meter).NewInt64ValueRecorder(vertexType + "." + id + ".duration"),
	}
}

func (r recorder) wrap(id, vertexType string, h handler) handler {
	return func(payload []*Packet) {
		r(id, vertexType, "start", payload)
		h(payload)
		r(id, vertexType, "done", payload)
	}
}

func do(ctx context.Context, fifo bool, h handler, input *edge) {
	go func() {
	Loop:
		for {
			select {
			case <-ctx.Done():
				break Loop
			case data := <-input.channel:
				if len(data) < 1 {
					continue
				}

				if fifo {
					h(data)
				} else {
					go h(data)
				}
			}
		}
	}()
}
