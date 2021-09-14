// Package machine - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.
package machine

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/json"
	"time"

	"github.com/whitaker-io/data"
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
		Metrics:    boolP(true),
		Span:       boolP(true),
		BufferSize: intP(0),
		Provider:   &edgeProvider{},
	}
)

// EdgeProvider is an interface that is used for providing new instances
// of the Edge interface given the *Option set in the Stream
type EdgeProvider interface {
	New(ctx context.Context, id string, options *Option) Edge
}

// Edge is an inteface that is used for transferring data between vertices
type Edge interface {
	Send(ctx context.Context, channel chan []*Packet)
	Next(payload ...*Packet)
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
	ID     string           `json:"id"`
	Data   data.Data        `json:"data"`
	Errors map[string]error `json:"errors"`
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
	// Provider determines the edge type to be used, logic for what type of edge
	// for a given id is required if not using homogeneous edges
	// Default: nil
	Provider EdgeProvider
	// Validators are used to ensure the incoming Data is compliant
	// they are run at the start of the stream before creation of Packets
	// Default: nil
	Validators map[string]ForkRule
}

// Retriever is a function that provides data to a generic Stream
// must stop when the context receives a done signal.
type Retriever func(ctx context.Context) chan []data.Data

// Applicative is a function that is applied on an individual
// basis for each Packet in the payload. The resulting data replaces
// the old data
type Applicative func(d data.Data) data.Data

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

// Comparator is a function to compare 2 data.Data's
type Comparator func(a data.Data, b data.Data) int

// Remover func that is used to remove Data based on a true result
type Remover func(index int, d data.Data) bool

// Error type for wrapping errors coming from the Stream
type Error struct {
	Err        error     `json:"err"`
	StreamID   string    `json:"stream_id"`
	VertexID   string    `json:"vertex_id"`
	VertexType string    `json:"vertex_type"`
	Packets    []*Packet `json:"payload"`
	Time       time.Time `json:"time"`
}

type handler func(payload []*Packet)

type edgeProvider struct{}

type edge struct {
	channel chan []*Packet
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
		Metrics:    o.Metrics,
		Span:       o.Span,
		Provider:   o.Provider,
		Validators: o.Validators,
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

	if option.Metrics != nil {
		out.Metrics = option.Metrics
	}

	if option.Span != nil {
		out.Span = option.Span
	}

	if option.Provider != nil {
		out.Provider = option.Provider
	}

	if option.Validators != nil {
		out.Validators = option.Validators
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
	bytez, err := json.Marshal(e)

	if err != nil {
		return e.Error()
	}

	return string(bytez)
}

func (p *edgeProvider) New(ctx context.Context, id string, options *Option) Edge {
	b := 0

	if options.BufferSize != nil {
		b = *options.BufferSize
	}

	return &edge{
		channel: make(chan []*Packet, b),
	}
}

func (out *edge) Send(ctx context.Context, channel chan []*Packet) {
	go func() {
	Loop:
		for {
			select {
			case <-ctx.Done():
				break Loop
			case list := <-out.channel:
				if len(list) > 0 {
					channel <- list
				}
			}
		}
	}()
}

func (out *edge) Next(payload ...*Packet) {
	out.channel <- payload
}

func boolP(v bool) *bool {
	return &v
}

func intP(v int) *int {
	return &v
}

func (o *Option) validate(payload ...data.Data) [][]string {
	list := make([][]string, len(payload))
	failed := false
	if o.Validators != nil {
		for i, d := range payload {
			for k, v := range o.Validators {
				if !v(d) {
					if len(list[i]) < 1 {
						list[i] = []string{}
					}
					list[i] = append(list[i], k)
					failed = true
				}
			}
		}
	}

	if failed {
		return list
	}

	return [][]string{}
}

func deepCopy(d []data.Data) []data.Data {
	out := []data.Data{}
	buf := &bytes.Buffer{}
	enc, dec := gob.NewEncoder(buf), gob.NewDecoder(buf)

	_ = enc.Encode(d)
	_ = dec.Decode(&out)

	return out
}

func deepCopyPayload(d []*Packet) []*Packet {
	out := []*Packet{}
	buf := &bytes.Buffer{}
	enc, dec := gob.NewEncoder(buf), gob.NewDecoder(buf)

	_ = enc.Encode(d)
	_ = dec.Decode(&out)

	return out
}
