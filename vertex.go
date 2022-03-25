// Package machine - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.
package machine

import (
	"context"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/trace"
)

var (
	meter         = global.Meter("machine")
	inCounter     = metric.Must(meter).NewInt64Counter("/machine/vertex/count")
	batchDuration = metric.Must(meter).NewInt64Histogram("/machine/vertex/duration")
	panicCounter  = metric.Must(meter).NewInt64Counter("/machine/vertex/panics")
)

type vertex[T Identifiable] struct {
	id         string
	vertexType string
	input      chan []T
	handler[T]
	connector func(ctx context.Context, streamID, currentID string, option *Option[T]) error
}

type handler[T Identifiable] func(payload []T)

func (v *vertex[T]) cascade(ctx context.Context, streamID, currentID string, option *Option[T], incoming Edge[T]) error {
	if v.input != nil {
		incoming.OutputTo(ctx, v.input)
		return nil
	}

	v.input = make(chan []T, *option.BufferSize)
	incoming.OutputTo(ctx, v.input)

	if err := v.connector(ctx, streamID, currentID, option); err != nil {
		return err
	}

	v.wrap(ctx, streamID, option)
	v.run(ctx, option)

	return nil
}

func (v *vertex[T]) wrap(ctx context.Context, streamID string, option *Option[T]) {
	h := v.handler
	tracer := otel.GetTracerProvider().Tracer(*option.Telemetry.TracerName)
	attributes := append(
		[]attribute.KeyValue{
			attribute.String(*option.Telemetry.LabelPrefix+"/machine/stream/id", streamID),
			attribute.String(*option.Telemetry.LabelPrefix+"/machine/vertex/id", v.id),
			attribute.String(*option.Telemetry.LabelPrefix+"/machine/vertex/type", v.vertexType),
		},
		option.Telemetry.AdditionalLabels...,
	)

	v.handler = func(payload []T) {
		var span trace.Span
		if option.Telemetry.Enabled != nil && *option.Telemetry.Enabled {
			ids := make([]string, len(payload))
			for i, packet := range payload {
				ids[i] = packet.ID()
			}

			_, span = tracer.Start(ctx, v.id,
				trace.WithAttributes(
					append(attributes,
						attribute.StringSlice("ids", ids),
					)...,
				),
			)

			inCounter.Add(ctx, int64(len(payload)), attributes...)
		}

		start := time.Now()

		defer func() {
			if r := recover(); r != nil {
				if err, ok := r.(error); !ok {
					if option.Telemetry.Enabled != nil && *option.Telemetry.Enabled {
						panicCounter.Add(ctx, 1, attributes...)
						span.RecordError(err)
					}
					option.PanicHandler(streamID, v.id, err, payload...)
				}
			}

			if option.Telemetry.Enabled != nil && *option.Telemetry.Enabled {
				duration := time.Since(start)
				batchDuration.Record(ctx, int64(duration), attributes...)

				span.End()
			}
		}()

		if *option.DeepCopy {
			h(deepcopy(payload))
		} else {
			h(payload)
		}
	}
}

func (v *vertex[T]) run(ctx context.Context, option *Option[T]) {
	go func() {
	Loop:
		for {
			select {
			case <-ctx.Done():
				break Loop
			case data := <-v.input:
				if len(data) < 1 {
					continue
				}

				if *option.FIFO {
					v.handler(data)
				} else {
					go v.handler(data)
				}
			}
		}
	}()
}
