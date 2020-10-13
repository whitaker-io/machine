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

const end = "end"

// Recorder type for providing a state recorder
type Recorder interface {
	Record(string, string, Payload)
}

// Vertex interface for defining a child node
type vertex interface {
	cascade(ctx context.Context, output *outChannel, machine *Machine) error
}

type labels struct {
	id       string
	name     string
	fifo     bool
	recorder Recorder
}

// Machine execution graph for a system
type Machine struct {
	labels
	initium Initium
	nodes   map[string]*node
	child   vertex
}

// node graph node for the Machine
type node struct {
	labels
	processus Processus
	child     vertex
	input     *inChannel
}

// router graph node for the Machine
type router struct {
	labels
	handler func(list Payload) (Payload, Payload)
	left    vertex
	right   vertex
	input   *inChannel
}

// cap graph leaf for the Machine
type cap struct {
	labels
	terminus Terminus
	input    *inChannel
}

// ID func to return the ID
func (m *Machine) ID() string {
	return m.id
}

// Run func to start the Machine
func (m *Machine) Run(ctx context.Context) error {
	return m.child.cascade(ctx, m.begin(ctx), m)
}

// Inject func to inject the logs into the machine
func (m *Machine) Inject(logs map[string]Payload) {
	for node, list := range logs {
		m.nodes[node].inject(list)
	}
}

func (m *Machine) begin(c context.Context) *outChannel {
	channel := newOutChannel()
	input := m.initium(c)
	go func() {
	Loop:
		for {
			select {
			case <-c.Done():
				break Loop
			case data := <-input:
				list := Payload{}
				for _, item := range data {
					list = append(list, &Packet{
						ID:   uuid.New().String(),
						Data: item,
					})
				}
				m.recorder.Record(m.id, "start", list)
				channel.channel <- list
			}
		}
	}()

	return channel
}

func (pn *node) inject(payload Payload) {
	pn.input.channel <- payload
}

func (pn *node) cascade(ctx context.Context, output *outChannel, m *Machine) error {
	if pn.input != nil {
		output.sendTo(pn.input)
		return nil
	}

	m.nodes[pn.id] = pn
	pn.input = output.convert()

	out := newOutChannel()

	fn := func(list Payload) {
		for _, log := range list {
			log.apply(pn.id, pn.processus)
		}
		out.channel <- list
	}

	run(ctx, pn.id, pn.name, pn.fifo, fn, pn.recorder, output)

	return pn.child.cascade(ctx, out, m)
}

func (r *router) cascade(ctx context.Context, output *outChannel, m *Machine) error {
	if r.input != nil {
		output.sendTo(r.input)
		return nil
	}

	r.input = output.convert()

	left := newOutChannel()
	right := newOutChannel()

	fn := func(list Payload) {
		l, r := r.handler(list)
		left.channel <- l
		right.channel <- r
	}

	run(ctx, r.id, "route", r.fifo, fn, r.recorder, output)

	if err := r.left.cascade(ctx, left, m); err != nil {
		return err
	} else if err := r.right.cascade(ctx, right, m); err != nil {
		return err
	}

	return nil
}

func (c *cap) cascade(ctx context.Context, output *outChannel, m *Machine) error {
	if m == nil {
		return fmt.Errorf("missing machine")
	} else if c.input != nil {
		output.sendTo(c.input)
		return nil
	}

	c.input = output.convert()

	runner := func(list Payload) {
		if len(list) < 1 {
			return
		}

		dataList := []map[string]interface{}{}
		for _, v := range list {
			dataList = append(dataList, v.Data)
		}
		list.handleError(c.id, c.terminus(dataList))

		c.recorder.Record(c.id, end, list)
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

	output.sendTo(c.input)

	return nil
}

func run(ctx context.Context, id, name string, fifo bool, r func(list Payload), recorder Recorder, output *outChannel) {
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

	runner := func(list Payload) {
		if len(list) < 1 {
			return
		}

		metricsCtx := otel.ContextWithBaggageValues(ctx, label.String("node_id", id))
		inCounter.Record(metricsCtx, int64(len(list)), labels...)
		inTotalCounter.Add(metricsCtx, float64(len(list)), labels...)

		_, span := tracer.Start(
			metricsCtx,
			name,
			trace.WithAttributes(labels...),
		)
		t := time.Now()
		for _, pkt := range list {
			span.AddEventWithTimestamp(
				metricsCtx,
				t,
				pkt.ID,
				append(
					labels,
					[]label.KeyValue{
						label.String("pkt_id", pkt.ID),
						label.String("error", pkt.error()),
					}...,
				)...)
		}

		start := time.Now()

		r(list)

		duration := time.Since(start)

		recorder.Record(id, name, list)

		span.End()

		failures := list.errorCount()
		outCounter.Record(metricsCtx, int64(len(list)), labels...)
		outTotalCounter.Add(metricsCtx, float64(len(list)), labels...)
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
