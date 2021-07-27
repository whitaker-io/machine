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

	"github.com/google/uuid"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/trace"
)

var (
	meter         = global.Meter("machine")
	tracer        = otel.GetTracerProvider().Tracer("machine")
	inCounter     = metric.Must(meter).NewInt64ValueRecorder("incoming")
	outCounter    = metric.Must(meter).NewInt64ValueRecorder("outgoing")
	errorsCounter = metric.Must(meter).NewInt64ValueRecorder("errors")
	batchDuration = metric.Must(meter).NewInt64ValueRecorder("duration")
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

	v.metrics(ctx)
	v.span(ctx)
	v.deepCopy()
	v.recover(b)
	v.run(ctx)

	return nil
}

func (v *vertex) span(ctx context.Context) {
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
					spans[packet.ID].AddEvent("error")
				}
				spans[packet.ID].End()
			}
		}
	}
}

func (v *vertex) metrics(ctx context.Context) {
	if *v.builder.option.Metrics {
		h := v.handler

		id := attribute.String("vertex_id", v.id)
		vType := attribute.String("vertex_type", v.vertexType)

		v.handler = func(payload []*Packet) {
			runID := attribute.String("run_id", uuid.NewString())

			inCounter.Record(ctx, int64(len(payload)), id, vType, runID)
			start := time.Now()
			h(payload)
			duration := time.Since(start)
			failures := 0
			for _, packet := range payload {
				if _, ok := packet.Errors[v.id]; ok {
					failures++
				}
			}
			outCounter.Record(ctx, int64(len(payload)), id, vType, runID)
			errorsCounter.Record(ctx, int64(failures), id, vType, runID)
			batchDuration.Record(ctx, int64(duration), id, vType, runID)
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
