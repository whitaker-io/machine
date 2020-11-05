// Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package machine

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/metric"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"
)

type handler func([]*Packet)
type recorder func(string, string, string, []*Packet)

type vertex struct {
	id         string
	vertexType string
	input      *edge
	handler
	connector func(ctx context.Context, r recorder, vertacies map[string]*vertex, option *Option) error
	metrics   *metrics
}

type metrics struct {
	tracer             trace.Tracer
	labels             []label.KeyValue
	inCounter          metric.Int64ValueRecorder
	outCounter         metric.Int64ValueRecorder
	errorsCounter      metric.Int64ValueRecorder
	inTotalCounter     metric.Float64Counter
	outTotalCounter    metric.Float64Counter
	errorsTotalCounter metric.Float64Counter
	batchDuration      metric.Int64ValueRecorder
}

func (v *vertex) cascade(ctx context.Context, r recorder, vertacies map[string]*vertex, option *Option, input *edge) error {
	v.input = input

	h := v.handler

	if r != nil {
		h = r.wrap(v.id, v.vertexType, h)
	}

	if *option.Metrics {
		h = v.metrics.wrap(ctx, h)
	}

	if *option.Span {
		h = v.wrap(ctx, h)
	}

	do(ctx, *option.FIFO, h, input)

	vertacies[v.id] = v

	return v.connector(ctx, r, vertacies, option)
}

func (v *vertex) wrap(ctx context.Context, h handler) handler {
	return func(payload []*Packet) {
		start := time.Now()

		for _, packet := range payload {
			packet.span.AddEvent(ctx, "vertex",
				label.String("vertex_id", v.id),
				label.String("vertex_type", v.vertexType),
				label.String("packet_id", packet.ID),
				label.Int64("when", start.UnixNano()),
			)
		}

		h(payload)

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

func (mtrx *metrics) wrap(ctx context.Context, h handler) handler {
	return func(payload []*Packet) {
		mtrx.inCounter.Record(ctx, int64(len(payload)), mtrx.labels...)
		mtrx.inTotalCounter.Add(ctx, float64(len(payload)), mtrx.labels...)
		start := time.Now()
		h(payload)
		duration := time.Since(start)
		failures := 0
		for _, packet := range payload {
			if packet.Error != nil {
				failures++
			}
		}
		mtrx.outCounter.Record(ctx, int64(len(payload)), mtrx.labels...)
		mtrx.outTotalCounter.Add(ctx, float64(len(payload)), mtrx.labels...)
		mtrx.errorsCounter.Record(ctx, int64(failures), mtrx.labels...)
		mtrx.errorsTotalCounter.Add(ctx, float64(failures), mtrx.labels...)
		mtrx.batchDuration.Record(ctx, int64(duration), mtrx.labels...)
	}
}

func (r recorder) wrap(id, vertexType string, h handler) handler {
	return func(payload []*Packet) {
		r(id, vertexType, "start", payload)
		h(payload)
		r(id, vertexType, "done", payload)
	}
}

func createMetrics(id, vertexType string) *metrics {
	meter := global.Meter(id)
	return &metrics{
		tracer: global.Tracer(vertexType + "." + id),
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
