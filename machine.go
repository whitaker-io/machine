package machine

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/api/global"
	"go.opentelemetry.io/otel/api/metric"
	"go.opentelemetry.io/otel/api/trace"
	"go.opentelemetry.io/otel/label"
)

// Vertex interface for defining a child node
type vertex interface {
	cascade(ctx context.Context, output *outChannel, machine *Machine) error
}

// Machine execution graph for a system
type Machine struct {
	info
	initium Initium
	nodes   map[string]*node
	child   vertex
}

// node graph node for the Machine
type node struct {
	info
	processus Processus
	child     vertex
	input     *inChannel
}

// router graph node for the Machine
type router struct {
	info
	handler func([]*Packet) ([]*Packet, []*Packet)
	left    vertex
	right   vertex
	input   *inChannel
}

// termination graph leaf for the Machine
type termination struct {
	info
	terminus Terminus
	input    *inChannel
}

type info struct {
	id       string
	name     string
	fifo     bool
	recorder func(string, string, []*Packet)
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
func (m *Machine) Inject(logs map[string][]*Packet) {
	for node, list := range logs {
		m.nodes[node].inject(list)
	}
}

func (m *Machine) begin(ctx context.Context) *outChannel {
	meterName := fmt.Sprintf("machine.%s", m.id)
	meter := global.Meter(meterName)
	labels := []label.KeyValue{label.String("id", m.id), label.String("type", m.name)}

	inCounter := metric.Must(meter).NewInt64ValueRecorder(meterName + ".incoming")
	outCounter := metric.Must(meter).NewInt64ValueRecorder(meterName + ".outgoing")
	inTotalCounter := metric.Must(meter).NewFloat64Counter(meterName + ".total.incoming")
	outTotalCounter := metric.Must(meter).NewFloat64Counter(meterName + ".total.outgoing")
	batchDuration := metric.Must(meter).NewInt64ValueRecorder(meterName + ".duration")

	channel := newOutChannel()

	input := m.initium(ctx)

	fn := func(data []map[string]interface{}) {
		metricsCtx := otel.ContextWithBaggageValues(ctx, label.String("node_id", m.id))
		inCounter.Record(metricsCtx, int64(len(data)), labels...)
		inTotalCounter.Add(metricsCtx, float64(len(data)), labels...)

		start := time.Now()

		payload := []*Packet{}
		for _, item := range data {
			payload = append(payload, &Packet{
				ID:   uuid.New().String(),
				Data: item,
			})
		}

		duration := time.Since(start)
		m.recorder(m.id, "start", payload)
		channel.channel <- payload

		outCounter.Record(metricsCtx, int64(len(payload)), labels...)
		outTotalCounter.Add(metricsCtx, float64(len(payload)), labels...)
		batchDuration.Record(metricsCtx, int64(duration), labels...)
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

	return channel
}

func (pn *node) inject(payload []*Packet) {
	pn.input.channel <- payload
}

func (pn *node) cascade(ctx context.Context, output *outChannel, m *Machine) error {
	if pn.input != nil {
		output.sendTo(ctx, pn.input)
		return nil
	}

	m.nodes[pn.id] = pn
	pn.input = output.convert()
	pn.recorder = m.recorder

	out := newOutChannel()

	fn := func(payload []*Packet) {
		for _, packet := range payload {
			packet.apply(pn.id, pn.processus)
		}
		out.channel <- payload
	}

	run(ctx, pn.id, pn.name, pn.fifo, fn, pn.recorder, output)

	if pn.child == nil {
		return fmt.Errorf("non-terminated node")
	}

	return pn.child.cascade(ctx, out, m)
}

func (r *router) cascade(ctx context.Context, output *outChannel, m *Machine) error {
	if r.input != nil {
		output.sendTo(ctx, r.input)
		return nil
	}

	r.input = output.convert()
	r.recorder = m.recorder

	left := newOutChannel()
	right := newOutChannel()

	fn := func(payload []*Packet) {
		l, r := r.handler(payload)
		left.channel <- l
		right.channel <- r
	}

	run(ctx, r.id, "route", r.fifo, fn, r.recorder, output)

	if r.left == nil || r.right == nil {
		return fmt.Errorf("non-terminated router")
	} else if err := r.left.cascade(ctx, left, m); err != nil {
		return err
	} else if err := r.right.cascade(ctx, right, m); err != nil {
		return err
	}

	return nil
}

func (c *termination) cascade(ctx context.Context, output *outChannel, m *Machine) error {
	c.input = output.convert()
	c.recorder = m.recorder

	runner := func(payload []*Packet) {
		if len(payload) < 1 {
			return
		}

		data := []map[string]interface{}{}
		for _, packet := range payload {
			data = append(data, packet.Data)
		}

		err := c.terminus(data)

		if err != nil {
			for _, packet := range payload {
				packet.handleError(c.id, err)
			}
		}

		c.recorder(c.id, "end", payload)
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

func run(ctx context.Context, id, name string, fifo bool, r func([]*Packet), recorder func(string, string, []*Packet), output *outChannel) {
	meterName := fmt.Sprintf("machine.%s", id)
	meter := global.Meter(meterName)
	tracer := global.Tracer(meterName)
	labels := []label.KeyValue{label.String("id", id), label.String("type", name)}

	inCounter := metric.Must(meter).NewInt64ValueRecorder(meterName + ".incoming")
	outCounter := metric.Must(meter).NewInt64ValueRecorder(meterName + ".outgoing")
	errorsCounter := metric.Must(meter).NewInt64ValueRecorder(meterName + ".errors")
	inTotalCounter := metric.Must(meter).NewFloat64Counter(meterName + ".total.incoming")
	outTotalCounter := metric.Must(meter).NewFloat64Counter(meterName + ".total.outgoing")
	errorsTotalCounter := metric.Must(meter).NewFloat64Counter(meterName + ".total.errors")
	batchDuration := metric.Must(meter).NewInt64ValueRecorder(meterName + ".duration")

	metricsCtx := otel.ContextWithBaggageValues(ctx, label.String("node_id", id))

	runner := func(payload []*Packet) {
		if len(payload) < 1 {
			return
		}

		inCounter.Record(metricsCtx, int64(len(payload)), labels...)
		inTotalCounter.Add(metricsCtx, float64(len(payload)), labels...)

		_, span := tracer.Start(
			metricsCtx,
			name,
			trace.WithAttributes(labels...),
		)
		t := time.Now()
		for _, pkt := range payload {
			span.AddEventWithTimestamp(
				metricsCtx,
				t,
				pkt.ID,
				append(
					labels,
					[]label.KeyValue{
						label.String("pkt_id", pkt.ID),
					}...,
				)...)
		}

		recorder(id, name, payload)

		start := time.Now()

		r(payload)

		duration := time.Since(start)

		span.End()

		failures := 0

		for _, packet := range payload {
			if packet.Error != nil {
				failures++
			}
		}

		outCounter.Record(metricsCtx, int64(len(payload)), labels...)
		outTotalCounter.Add(metricsCtx, float64(len(payload)), labels...)
		errorsCounter.Record(metricsCtx, int64(failures), labels...)
		errorsTotalCounter.Add(metricsCtx, float64(failures), labels...)
		batchDuration.Record(metricsCtx, int64(duration), labels...)
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
