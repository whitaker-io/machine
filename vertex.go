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
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/trace"
)

type vertex struct {
	id         string
	vertexType string
	input      *edge
	handler
	connector func(ctx context.Context, b *builder) error
	option    *Option
}

func (v *vertex) cascade(ctx context.Context, b *builder, input *edge) error {
	if v.input != nil && v.vertexType != "stream" {
		input.sendTo(ctx, v.input)
		return nil
	}

	v.option = b.option.merge(v.option)
	v.input = input

	v.record(b.record)
	v.metrics(ctx)
	v.span(ctx)
	v.deepCopy()
	v.recover()
	v.run(ctx)

	b.vertacies[v.id] = v

	return v.connector(ctx, b)
}

func (v *vertex) span(ctx context.Context) {
	h := v.handler

	tracer := otel.GetTracerProvider().Tracer(v.vertexType + "." + v.id)

	vertexAttributes := trace.WithAttributes(
		attribute.String("vertex_id", v.id),
		attribute.String("vertex_type", v.vertexType),
	)

	errorName := v.vertexType + "-error"

	v.handler = func(payload []*Packet) {
		if *v.option.Span {
			now := time.Now()

			for _, packet := range payload {
				if packet.span == nil {
					packet.newSpan(ctx, tracer, "stream.inject", vertexAttributes)
				}

				packet.span.AddEvent(v.vertexType, vertexAttributes, trace.WithTimestamp(now))
			}
		}

		h(payload)

		if *v.option.Span {
			now := time.Now()

			for _, packet := range payload {
				if packet.Error != nil {
					packet.span.AddEvent(errorName, vertexAttributes, trace.WithTimestamp(now))
				}
			}
		}

		if v.vertexType == "publish" {
			for _, packet := range payload {
				if packet.span != nil {
					packet.span.End()
				}
			}
		}
	}
}

func (v *vertex) metrics(ctx context.Context) {
	if *v.option.Metrics {
		h := v.handler

		meter := global.Meter(v.id)

		labels := []attribute.KeyValue{
			attribute.String("vertex_id", v.id),
			attribute.String("vertex_type", v.vertexType),
		}

		inTotalCounter := metric.Must(meter).NewFloat64Counter(v.vertexType + "." + v.id + ".total.incoming")
		outTotalCounter := metric.Must(meter).NewFloat64Counter(v.vertexType + "." + v.id + ".total.outgoing")
		errorsTotalCounter := metric.Must(meter).NewFloat64Counter(v.vertexType + "." + v.id + ".total.errors")
		inCounter := metric.Must(meter).NewInt64ValueRecorder(v.vertexType + "." + v.id + ".incoming")
		outCounter := metric.Must(meter).NewInt64ValueRecorder(v.vertexType + "." + v.id + ".outgoing")
		errorsCounter := metric.Must(meter).NewInt64ValueRecorder(v.vertexType + "." + v.id + ".errors")
		batchDuration := metric.Must(meter).NewInt64ValueRecorder(v.vertexType + "." + v.id + ".duration")

		v.handler = func(payload []*Packet) {
			inCounter.Record(ctx, int64(len(payload)), labels...)
			inTotalCounter.Add(ctx, float64(len(payload)), labels...)
			start := time.Now()
			h(payload)
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
	}
}

func (v *vertex) recover() {
	h := v.handler

	v.handler = func(payload []*Packet) {
		defer func() {
			if r := recover(); r != nil {
				var err error
				var ok bool

				if err, ok = r.(error); !ok {
					err = fmt.Errorf("%v", r)
				}

				ids := make([]string, len(payload))

				for i, packet := range payload {
					ids[i] = packet.ID
				}

				logger.Error(fmt.Sprintf("panic-recovery [id: %s type: %s error: %v packets: %v]", v.id, v.vertexType, err, ids))
			}
		}()

		h(payload)
	}
}

func (v *vertex) deepCopy() {
	if *v.option.DeepCopy {
		h := v.handler

		v.handler = func(payload []*Packet) {
			out := []*Packet{}
			buf := &bytes.Buffer{}
			enc, dec := gob.NewEncoder(buf), gob.NewDecoder(buf)

			_ = enc.Encode(payload)
			_ = dec.Decode(&out)

			for i, val := range payload {
				out[i].span = val.span
			}

			h(out)
		}
	}
}

func (v *vertex) record(r recorder) {
	if r != nil {
		h := v.handler

		v.handler = func(payload []*Packet) {
			r(v.id, v.vertexType, "start", payload)
			h(payload)
		}
	}
}

func (v *vertex) run(ctx context.Context) {
	go func() {
	Loop:
		for {
			select {
			case <-ctx.Done():
				break Loop
			case data := <-v.input.channel:
				if len(data) < 1 {
					continue
				}

				if *v.option.FIFO {
					v.handler(data)
				} else {
					go v.handler(data)
				}
			}
		}
	}()
}
