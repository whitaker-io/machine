// Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package machine

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"time"

	"github.com/whitaker-io/data"
	"go.opentelemetry.io/otel/trace"
)

var (
	// ForkDuplicate is a Fork that creates a deep copy of the
	// payload and sends it down both branches.
	ForkDuplicate Fork = func(payload []*Packet) (a, b []*Packet) {
		payload2 := []*Packet{}
		buf := &bytes.Buffer{}
		enc, dec := gob.NewEncoder(buf), gob.NewDecoder(buf)

		_ = enc.Encode(payload)
		_ = dec.Decode(&payload2)

		for i, packet := range payload {
			payload2[i].span = packet.span
		}

		return payload, payload2
	}

	// ForkError is a Fork that splits machine.Packets based on
	// the presence of an error. Errors are sent down the
	// right side path.
	ForkError Fork = func(payload []*Packet) (s, f []*Packet) {
		s = []*Packet{}
		f = []*Packet{}

		for _, packet := range payload {
			if len(packet.Errors) > 0 {
				f = append(f, packet)
			} else {
				s = append(s, packet)
			}
		}

		return s, f
	}

	defaultOptions = &Option{
		DeepCopy:   boolP(false),
		FIFO:       boolP(false),
		Injectable: boolP(true),
		Metrics:    boolP(true),
		Span:       boolP(true),
		BufferSize: intP(0),
	}
)

// Cluster is an interface for allowing a distributed cluster of workers
// It requires 3 methods Join, Write, and Leave.
//
// Join is called during the run method of the Stream and it is used for
// accouncing membership to the cluster with an identifier and a callback
// for injection
//
// Injection is how work can be restarted in the system and is the responsibility
// of the implementation to decide when and how it is reinitiated.
//
// Write is called at the beginning of every vertex and provides the current
// payload about to be run. The implementation is considered the owner of that
// payload and may modify the data in special known circumstances if need be.
//
// Leave is called during a graceful termination and any errors
// are logged.
type Cluster interface {
	Join(id string, callback InjectionCallback) error
	Write(logs ...*Log)
	Leave(id string) error
}

// InjectionCallback is a function provided to the LogStore Join method so that
// the cluster may restart work that has been dropped by one of the workers.
// Injections will only be processed for vertices that have the Injectable option
// set to true, which is the default.
type InjectionCallback func(logs ...*Log)

// Log type for holding the data that is recorded from the streams and sent to
// the LogStore instance
type Log struct {
	StreamID   string    `json:"stream_id"`
	VertexID   string    `json:"vertex_id"`
	VertexType string    `json:"vertex_type"`
	State      string    `json:"state"`
	Packet     *Packet   `json:"packet"`
	When       time.Time `json:"when"`
}

// Subscription is an interface for creating a pull based stream.
// It requires 2 methods Read and Close.
//
// Read is called when the interval passes and the resulting
// payload is sent down the Stream.
//
// Close is called during a graceful termination and any errors
// are logged.
type Subscription interface {
	Read(ctx context.Context) []data.Data
	Close() error
}

// Publisher is an interface for sending data out of the Stream
type Publisher interface {
	Send([]data.Data) error
}

// Packet type that holds information traveling through the machine.
type Packet struct {
	ID      string           `json:"id"`
	Data    data.Data        `json:"data"`
	Errors  map[string]error `json:"errors"`
	span    trace.Span
	spanCtx context.Context
}

// Option type for holding machine settings.
type Option struct {
	// DeepCopy uses encoding/gob to create a deep copy of the payload
	// before the processing to ensure concurrent map exceptions cannot
	// happen. Is fairly resource intensive so use with caution.
	// Default: false
	DeepCopy *bool
	// FIFO controls the processing order of the payloads
	// If set to true the system will wait for one payload
	// to be processed before starting the next.
	// Default: false
	FIFO *bool
	// Injectable controls whether the vertex accepts injection calls
	// if set to false the data will be logged and processing will not
	// take place.
	// Default: true
	Injectable *bool
	// BufferSize sets the buffer size on the edge channels between the
	// vertices, this setting can be useful when processing large amounts
	// of data with FIFO turned on.
	// Default: 0
	BufferSize *int
	// Span controls whether opentelemetry spans are created for tracing
	// Packets processed by the system.
	// Default: true
	Span *bool
	// Metrics controls whether opentelemetry metrics are recorded for
	// Packets processed by the system.
	// Default: true
	Metrics *bool
}

// Retriever is a function that provides data to a generic Stream
// must stop when the context receives a done signal.
type Retriever func(ctx context.Context) chan []data.Data

// Applicative is a function that is applied on an individual
// basis for each Packet in the payload. The data may be modified
// or checked for correctness. Any resulting error is combined
// with current errors in the wrapping Packet.
type Applicative func(d data.Data) error

// Fold is a function used to combine a payload into a single Packet.
// It may be used with either a Fold Left or Fold Right operation,
// which starts at the corresponding side and moves through the payload.
// The returned instance of data.Data is used as the aggregate in the subsequent
// call.
type Fold func(aggregate, next data.Data) data.Data

// Fork is a function for splitting the payload into 2 separate paths.
// Default Forks for duplication and error checking are provided by
// ForkDuplicate and ForkError respectively.
type Fork func(list []*Packet) (a, b []*Packet)

// ForkRule is a function that can be converted to a Fork via the Handler
// method allowing for Forking based on the contents of the data.
type ForkRule func(d data.Data) bool

// Error type for wrapping errors coming from the Stream
type Error struct {
	Err        error
	StreamID   string
	VertexID   string
	VertexType string
	Packets    []*Packet
	Time       time.Time
}

type handler func(payload []*Packet)

type edge struct {
	channel chan []*Packet
}

func (p *Packet) apply(id string, a Applicative) {
	p.handleError(id, a(p.Data))
}

func (p *Packet) handleError(id string, err error) {
	if err != nil {
		p.Errors[id] = err
	}
}

func (o *Option) merge(options ...*Option) *Option {
	if len(options) < 1 {
		return o
	} else if len(options) == 1 {
		return o.join(options[0])
	}

	return o.join(options[0]).merge(options[1:]...)
}

func (o *Option) join(option *Option) *Option {
	out := &Option{
		DeepCopy:   o.DeepCopy,
		FIFO:       o.FIFO,
		BufferSize: o.BufferSize,
		Injectable: o.Injectable,
		Metrics:    o.Metrics,
		Span:       o.Span,
	}

	if option.DeepCopy != nil {
		out.DeepCopy = option.DeepCopy
	}

	if option.FIFO != nil {
		out.FIFO = option.FIFO
	}

	if option.BufferSize != nil {
		out.BufferSize = option.BufferSize
	}

	if option.Injectable != nil {
		out.Injectable = option.Injectable
	}

	if option.Metrics != nil {
		out.Metrics = option.Metrics
	}

	if option.Span != nil {
		out.Span = option.Span
	}

	return out
}

// Handler is a method for turning the ForkRule into an instance of Fork
func (r ForkRule) Handler(payload []*Packet) (t, f []*Packet) {
	t = []*Packet{}
	f = []*Packet{}

	for _, packet := range payload {
		if r(packet.Data) {
			t = append(t, packet)
		} else {
			f = append(f, packet)
		}
	}

	return t, f
}

func (e *Error) Error() string {
	bytez, err := json.Marshal(e.Packets)

	if err != nil {
		return fmt.Sprintf(
			`{"error": "%s", "stream_id":"%s", "vertex_id": "%s", "vertex_type":"%s", "packets": "%s", "time":"%v"}`,
			e.Err.Error(),
			e.StreamID,
			e.VertexID,
			e.VertexType,
			err.Error(),
			e.Time,
		)
	}

	return fmt.Sprintf(
		`{"error": "%s", "stream_id":"%s", "vertex_id": "%s", "vertex_type":"%s", "packets": %s, "time":"%v"}`,
		e.Err.Error(),
		e.StreamID,
		e.VertexID,
		e.VertexType,
		string(bytez),
		e.Time,
	)
}

func (out *edge) sendTo(ctx context.Context, in *edge) {
	go func() {
	Loop:
		for {
			select {
			case <-ctx.Done():
				break Loop
			case list := <-out.channel:
				if len(list) > 0 {
					in.channel <- list
				}
			}
		}
	}()
}

func newEdge(bufferSize *int) *edge {
	b := 0

	if bufferSize != nil {
		b = *bufferSize
	}

	return &edge{
		make(chan []*Packet, b),
	}
}

func boolP(v bool) *bool {
	return &v
}

func intP(v int) *int {
	return &v
}

func deepCopy(d []data.Data) []data.Data {
	out := []data.Data{}
	buf := &bytes.Buffer{}
	enc, dec := gob.NewEncoder(buf), gob.NewDecoder(buf)

	_ = enc.Encode(d)
	_ = dec.Decode(&out)

	return out
}
