// Package machine - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.
package machine

import (
	"context"
	"fmt"
	"os"
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
	builder    *builder[T]
	handler[T]
	connector func(ctx context.Context, b *builder[T]) error
}

func (v *vertex[T]) cascade(ctx context.Context, b *builder[T], incoming Edge[T]) error {
	v.builder = b

	if v.input != nil {
		incoming.SetOutput(ctx, v.input)
		return nil
	}

	v.input = make(chan []T, *b.option.BufferSize)
	incoming.SetOutput(ctx, v.input)

	if _, ok := b.edges[v.id]; ok && v.id != "__channel" {
		return fmt.Errorf("duplicate vertex id %s", v.id)
	}

	if v.id != "__channel__" {
		b.edges[v.id] = incoming
	}

	if err := v.connector(ctx, b); err != nil {
		return err
	}

	v.wrap(ctx, b)
	v.deepCopy()
	v.run(ctx)

	return nil
}

func (v *vertex[T]) wrap(ctx context.Context, b *builder[T]) {
	h := v.handler
	tracer := otel.GetTracerProvider().Tracer("machine")
	attributes := []attribute.KeyValue{
		attribute.String("/machine/stream/id", b.id),
		attribute.String("/machine/vertex/id", v.id),
		attribute.String("/machine/vertex/type", v.vertexType),
		attribute.String("g.co/r/k8s_container/project_id", os.Getenv("GOOGLE_CLOUD_PROJECT")),
		attribute.String("g.co/r/k8s_container/cluster_name", os.Getenv("CLUSTER_NAME")),
		attribute.String("g.co/r/k8s_container/namespace", os.Getenv("NAMESPACE")),
		attribute.String("g.co/r/k8s_container/pod_name", os.Getenv("POD_NAME")),
		attribute.String("g.co/r/k8s_container/container_name", os.Getenv("CONTAINER_NAME")),
		attribute.String("service.name", os.Getenv("POD_NAME")),
		attribute.String("http.host", os.Getenv("HOSTNAME")),
	}

	v.handler = func(payload []T) {
		ids := make([]string, len(payload))
		for i, packet := range payload {
			ids[i] = packet.ID()
		}
		_, span := tracer.Start(ctx, v.id,
			trace.WithAttributes(
				append(attributes,
					attribute.StringSlice("ids", ids),
				)...,
			),
		)

		defer func() {
			if r := recover(); r != nil {
				if err, ok := r.(error); !ok {
					panicCounter.Add(ctx, 1, attributes...)
					span.RecordError(err)
					b.option.PanicHandler(b.ID(), v.id, err, payload...)
				}
			}
			span.End()
		}()

		inCounter.Add(ctx, int64(len(payload)), attributes...)
		start := time.Now()

		h(payload)

		duration := time.Since(start)
		batchDuration.Record(ctx, int64(duration), attributes...)
	}
}

func (v *vertex[T]) deepCopy() {
	if *v.builder.option.DeepCopy {
		h := v.handler

		v.handler = func(payload []T) {
			h(deepcopy(payload))
		}
	}
}

func (v *vertex[T]) run(ctx context.Context) {
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

				if *v.builder.option.FIFO {
					v.handler(data)
				} else {
					go v.handler(data)
				}
			}
		}
	}()
}
