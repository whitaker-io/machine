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
	input      *edge
	handler
	errorHandler func(*Error)
	connector    func(ctx context.Context, b *builder) error
	option       *Option
}

func (v *vertex) cascade(ctx context.Context, b *builder, input *edge) error {
	v.errorHandler = func(err *Error) {
		err.StreamID = b.id
		b.errorChannel <- err
	}

	if v.input != nil && v.vertexType != "stream" {
		input.sendTo(ctx, v.input)
		return nil
	}

	v.option = b.option.merge(v.option)
	v.input = input

	b.vertacies[v.id] = v

	if err := v.connector(ctx, b); err != nil {
		return err
	}

	v.record(b)
	v.metrics(ctx)
	v.span()
	v.deepCopy()
	v.recover(b)
	v.run(ctx)

	return nil
}

func (v *vertex) span() {
	h := v.handler

	vType := trace.WithAttributes(attribute.String("vertex_type", v.vertexType))

	if *v.option.Span {
		v.handler = func(payload []*Packet) {
			spans := map[string]trace.Span{}

			for _, packet := range payload {
				_, spans[packet.ID] = tracer.Start(packet.spanCtx, v.id, vType)
			}

			h(payload)

			for _, packet := range payload {
				if _, ok := packet.Errors[v.id]; ok {
					spans[packet.ID].AddEvent("error")
					spans[packet.ID].End()
				}
			}
		}
	}
}

func (v *vertex) metrics(ctx context.Context) {
	if *v.option.Metrics {
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

func (v *vertex) record(b *builder) {
	h := v.handler

	v.handler = func(payload []*Packet) {
		b.record(v.id, v.vertexType, "start", payload)
		h(payload)
		b.record(v.id, v.vertexType, "end", payload)
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
