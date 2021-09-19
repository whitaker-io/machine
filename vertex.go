// Package machine - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.
package machine

import (
	"bytes"
	"context"
	"encoding/gob"
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
	errorsCounter = metric.Must(meter).NewInt64Counter("/machine/vertex/errors")
	batchDuration = metric.Must(meter).NewInt64Histogram("/machine/vertex/duration")
)

type vertex struct {
	id         string
	vertexType string
	input      chan []*Packet
	builder    *builder
	handler
	errorHandler func(*Error)
	connector    func(ctx context.Context, b *builder) error
}

func (v *vertex) cascade(ctx context.Context, b *builder, incoming Edge) error {
	v.builder = b
	v.errorHandler = func(err *Error) {
		err.StreamID = b.id
		b.errorChannel <- err
	}

	if v.input != nil {
		incoming.Send(ctx, v.input)
		return nil
	}

	v.input = make(chan []*Packet, *b.option.BufferSize)
	incoming.Send(ctx, v.input)

	if _, ok := b.edges[v.id]; ok {
		return fmt.Errorf("duplicate vertex id %s", v.id)
	}

	b.edges[v.id] = incoming

	if err := v.connector(ctx, b); err != nil {
		return err
	}

	v.deepCopy()
	v.recover(b)
	v.metrics(ctx)
	v.span(ctx)
	v.run(ctx)

	return nil
}

func (v *vertex) span(ctx context.Context) {
	tracer := otel.GetTracerProvider().Tracer("machine")
	h := v.handler

	vType := trace.WithAttributes(attribute.String("vertex_type", v.vertexType))

	if *v.builder.option.Span {
		v.handler = func(payload []*Packet) {
			spans := map[string]trace.Span{}

			for _, packet := range payload {
				_, spans[packet.ID] = tracer.Start(ctx, v.id, vType)
			}

			h(payload)

			for _, packet := range payload {
				if _, ok := packet.Errors[v.id]; ok {
					spans[packet.ID].RecordError(packet.Errors[v.id])
				}
				spans[packet.ID].End()
			}
		}
	}
}

func (v *vertex) metrics(ctx context.Context) {
	if *v.builder.option.Metrics {
		h := v.handler

		attributes := []attribute.KeyValue{
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

		v.handler = func(payload []*Packet) {
			inCounter.Add(ctx, int64(len(payload)), attributes...)
			start := time.Now()
			h(payload)
			duration := time.Since(start)
			failures := 0
			for _, packet := range payload {
				if _, ok := packet.Errors[v.id]; ok {
					failures++
				}
			}

			errorsCounter.Add(ctx, int64(failures), attributes...)
			batchDuration.Record(ctx, int64(duration), attributes...)
		}
	}
}

func (v *vertex) recover(b *builder) {
	h := v.handler

	v.handler = func(payload []*Packet) {
		defer func() {
			if r := recover(); r != nil {
				var err error
				var ok bool

				if err, ok = r.(error); !ok {
					err = fmt.Errorf("%v", r)
				}

				b.errorChannel <- &Error{
					Err:        fmt.Errorf("panic recovery %w", err),
					StreamID:   b.id,
					VertexID:   v.id,
					VertexType: v.vertexType,
					Packets:    payload,
					Time:       time.Now(),
				}
			}
		}()

		h(payload)
	}
}

func (v *vertex) deepCopy() {
	if *v.builder.option.DeepCopy {
		h := v.handler

		v.handler = func(payload []*Packet) {
			out := []*Packet{}
			buf := &bytes.Buffer{}
			enc, dec := gob.NewEncoder(buf), gob.NewDecoder(buf)

			_ = enc.Encode(payload)
			_ = dec.Decode(&out)

			h(out)
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
