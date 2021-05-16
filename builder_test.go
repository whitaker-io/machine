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
	"net/http"
	"testing"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/whitaker-io/data"
)

type testType struct {
	Name  string `mapstructure:"name"`
	Value int    `mapstructure:"value"`
}

var testListBase = []data.Data{
	{
		"__traceID": "test_trace_id",
		"name":      "data0",
		"value":     0,
	},
	{
		"name":  "data1",
		"value": 1,
	},
	{
		"name":  "data2",
		"value": 2,
	},
	{
		"name":  "data3",
		"value": 3,
	},
	{
		"name":  "data4",
		"value": 4,
	},
	{
		"name":  "data5",
		"value": 5,
	},
	{
		"name":  "data6",
		"value": 6,
	},
	{
		"name":  "data7",
		"value": 7,
	},
	{
		"name":  "data8",
		"value": 8,
	},
	{
		"name":  "data9",
		"value": 9,
	},
}

var testPayloadBase = []*Packet{
	{
		ID: "ID_0",
		Data: data.Data{
			"name":  "data0",
			"value": 0,
		},
	},
	{
		ID: "ID_1",
		Data: data.Data{
			"name":  "data1",
			"value": 1,
		},
	},
	{
		ID: "ID_2",
		Data: data.Data{
			"name":  "data2",
			"value": 2,
		},
	},
	{
		ID: "ID_3",
		Data: data.Data{
			"name":  "data3",
			"value": 3,
		},
	},
	{
		ID: "ID_4",
		Data: data.Data{
			"name":  "data4",
			"value": 4,
		},
	},
	{
		ID: "ID_5",
		Data: data.Data{
			"name":  "data5",
			"value": 5,
		},
	},
	{
		ID: "ID_6",
		Data: data.Data{
			"name":  "data6",
			"value": 6,
		},
	},
	{
		ID: "ID_7",
		Data: data.Data{
			"name":  "data7",
			"value": 7,
		},
	},
	{
		ID: "ID_8",
		Data: data.Data{
			"name":  "data8",
			"value": 8,
		},
	},
	{
		ID: "ID_9",
		Data: data.Data{
			"name":  "data9",
			"value": 9,
		},
	},
}

var bufferSize = 0

func deepCopyList(data []*Packet) []*Packet {
	out := []*Packet{}
	buf := &bytes.Buffer{}
	enc, dec := gob.NewEncoder(buf), gob.NewDecoder(buf)

	_ = enc.Encode(data)
	_ = dec.Decode(&out)

	return out
}

func Benchmark_Test_New(b *testing.B) {
	out := make(chan []data.Data)
	channel := make(chan []data.Data)
	m := NewStream("machine_id", func(c context.Context) chan []data.Data {
		return channel
	},
		&Option{FIFO: boolP(false)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(false)},
		&Option{BufferSize: intP(0)},
	)

	m.Builder().
		Map("map_id", func(m data.Data) error {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
				return fmt.Errorf("incorrect data have %v want %v", m, "name field")
			}
			return nil
		}).
		Publish("sender_id",
			publishFN(func(d []data.Data) error {
				out <- d
				return nil
			}),
		)

	if err := m.Run(context.Background(), time.Second); err != nil {
		b.Error(err)
	}

	for n := 0; n < b.N; n++ {
		go func() {
			channel <- testListBase
		}()

		list := <-out

		if len(list) != len(testListBase) {
			b.Errorf("incorrect data have %v want %v", list, testListBase)
		}
	}
}

func Test_New(b *testing.T) {
	count := 1000
	out := make(chan []data.Data)
	m := NewStream("machine_id", func(c context.Context) chan []data.Data {
		channel := make(chan []data.Data)
		go func() {
			for n := 0; n < count; n++ {
				channel <- deepCopy(testListBase)
			}
		}()
		return channel
	},
		&Option{FIFO: boolP(false)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(false)},
		&Option{BufferSize: intP(0)},
	)

	left, right := m.Builder().
		Map("map_id", func(m data.Data) error {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
				return fmt.Errorf("incorrect data have %v want %v", m, "name field")
			}
			return nil
		}).
		FoldLeft("fold_id1", func(d1, d2 data.Data) data.Data {
			return d1
		}).
		FoldLeft("fold_id1", func(d1, d2 data.Data) data.Data {
			return d1
		}).
		FoldRight("fold_id2", func(d1, d2 data.Data) data.Data {
			return d1
		}).
		Fork("fork_id", ForkError)

	left.Publish("sender_id",
		publishFN(func(d []data.Data) error {
			out <- d
			return nil
		}),
	)

	right.Publish("sender_id",
		publishFN(func(d []data.Data) error {
			b.Error("unexpected")
			return nil
		}),
	)

	if err := m.Run(context.Background(), time.Second); err != nil {
		b.Error(err)
	}

	for n := 0; n < count; n++ {
		list := <-out

		if len(list) != 1 {
			b.Errorf("incorrect data have %v want %v", list, testListBase[0])
		}
	}
}

func Test_New2(b *testing.T) {
	count := 1000
	out := make(chan []data.Data)
	m := NewStream("machine_id", func(c context.Context) chan []data.Data {
		channel := make(chan []data.Data)
		go func() {
			for n := 0; n < count; n++ {
				channel <- deepCopy(testListBase)
			}
		}()
		return channel
	},
		&Option{FIFO: boolP(true)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(true)},
		&Option{BufferSize: intP(1000)},
	)

	left, right := m.Builder().
		Map("map_id", func(m data.Data) error {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
				return fmt.Errorf("incorrect data have %v want %v", m, "name field")
			}
			return nil
		}).
		FoldRight("fold_id2", func(d1, d2 data.Data) data.Data {
			return d1
		}).
		Fork("fork_id", ForkDuplicate)

	left.Publish("sender_id",
		publishFN(func(d []data.Data) error {
			out <- d
			return nil
		}),
	)

	l2, r2 := right.Fork("fork_id2", ForkRule(func(d data.Data) bool {
		return true
	}).Handler)

	l3, r3 := l2.Fork("fork_id3", ForkRule(func(d data.Data) bool {
		return false
	}).Handler)

	r2.Publish("sender_id",
		publishFN(func(d []data.Data) error {
			b.Error("unexpected")
			return nil
		}),
	)

	l3.Publish("sender_id",
		publishFN(func(d []data.Data) error {
			b.Error("unexpected")
			return nil
		}),
	)

	r3.Publish("sender_id",
		publishFN(func(d []data.Data) error {
			out <- d
			return fmt.Errorf("error")
		}),
	)

	if err := m.Run(context.Background(), time.Second); err != nil {
		b.Error(err)
	}

	if m.ID() != "machine_id" {
		b.Errorf("incorrect id have %v want %v", m.ID(), "machine_id")
	}

	for n := 0; n < 2*count; n++ {
		list := <-out

		if len(list) != 1 {
			b.Errorf("incorrect data have %v want %v", list, testListBase[9])
		}
	}
}

func Test_Panic(b *testing.T) {
	count := 1000
	panicCount := 0
	out := make(chan []data.Data)
	m := NewStream("machine_id", func(c context.Context) chan []data.Data {
		channel := make(chan []data.Data)
		go func() {
			for n := 0; n < count; n++ {
				channel <- deepCopy(testListBase)
			}
		}()
		return channel
	},
		&Option{FIFO: boolP(true)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(true)},
		&Option{BufferSize: intP(1000)},
	)

	left, right := m.Builder().
		Map("map_id", func(m data.Data) error {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
				return fmt.Errorf("incorrect data have %v want %v", m, "name field")
			}
			return nil
		}).
		FoldRight("fold_id2", func(d1, d2 data.Data) data.Data {
			return d1
		}).
		Fork("fork_id", ForkDuplicate)

	left.Publish("sender_id",
		publishFN(func(d []data.Data) error {
			out <- d
			return nil
		}),
	)

	l2, r2 := right.Fork("fork_id2", ForkRule(func(d data.Data) bool {
		return true
	}).Handler)

	l3, r3 := l2.Fork("fork_id3", ForkRule(func(d data.Data) bool {
		return false
	}).Handler)

	r2.Publish("sender_id",
		publishFN(func(d []data.Data) error {
			b.Error("unexpected")
			return nil
		}),
	)

	l3.Publish("sender_id",
		publishFN(func(d []data.Data) error {
			b.Error("unexpected")
			return nil
		}),
	)

	r3.Publish("sender_id", publishFN(func(d []data.Data) error {
		panicCount++

		if panicCount%2 == 0 {
			panic("test panic")
		}

		out <- d
		return fmt.Errorf("error")
	}),
	)

	if err := m.Run(context.Background(), time.Second); err != nil {
		b.Error(err)
	}

	if m.ID() != "machine_id" {
		b.Errorf("incorrect id have %v want %v", m.ID(), "machine_id")
	}

	for n := 0; n < count; n++ {
		list := <-out

		if len(list) != 1 {
			b.Errorf("incorrect data have %v want %v", list, testListBase[9])
		}
	}
}

func Test_Missing_Leaves(b *testing.T) {
	m := NewStream("machine_id", func(c context.Context) chan []data.Data {
		channel := make(chan []data.Data)
		return channel
	})

	left, _ := m.Builder().Fork("fork_id", ForkDuplicate)

	left.Publish("sender_id",
		publishFN(func(d []data.Data) error {
			b.Error("unexpected")
			return nil
		}),
	)

	m2 := NewStream("machine_id", func(c context.Context) chan []data.Data {
		channel := make(chan []data.Data)
		return channel
	},
		&Option{FIFO: boolP(true)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(true)},
		&Option{BufferSize: intP(1000)},
	)

	m2.Builder().Fork("fork_id", ForkDuplicate)

	m3 := NewStream("machine_id", func(c context.Context) chan []data.Data {
		channel := make(chan []data.Data)
		return channel
	},
		&Option{FIFO: boolP(true)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(true)},
		&Option{BufferSize: intP(1000)},
	)

	m4 := NewStream("machine_id", func(c context.Context) chan []data.Data {
		channel := make(chan []data.Data)
		return channel
	},
		&Option{FIFO: boolP(true)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(true)},
		&Option{BufferSize: intP(1000)},
	)

	m4.Builder().Map("map_id", func(m data.Data) error {
		if _, ok := m["name"]; !ok {
			b.Errorf("packet missing name %v", m)
			return fmt.Errorf("incorrect data have %v want %v", m, "name field")
		}
		return nil
	})

	m5 := NewStream("machine_id", func(c context.Context) chan []data.Data {
		channel := make(chan []data.Data)
		return channel
	},
		&Option{FIFO: boolP(true)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(true)},
		&Option{BufferSize: intP(1000)},
	)

	m5.Builder().FoldRight("fold_id2", func(d1, d2 data.Data) data.Data {
		return d1
	})

	m6 := NewStream("machine_id", func(c context.Context) chan []data.Data {
		channel := make(chan []data.Data)
		return channel
	},
		&Option{FIFO: boolP(true)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(true)},
		&Option{BufferSize: intP(1000)},
	)

	l6, r6 := m6.Builder().Fork("fork_id", ForkDuplicate)

	l6.Map("map_id", func(m data.Data) error {
		if _, ok := m["name"]; !ok {
			b.Errorf("packet missing name %v", m)
			return fmt.Errorf("incorrect data have %v want %v", m, "name field")
		}
		return nil
	})

	r6.Map("map_id", func(m data.Data) error {
		if _, ok := m["name"]; !ok {
			b.Errorf("packet missing name %v", m)
			return fmt.Errorf("incorrect data have %v want %v", m, "name field")
		}
		return nil
	})

	m7 := NewStream("machine_id", func(c context.Context) chan []data.Data {
		channel := make(chan []data.Data)
		return channel
	},
		&Option{FIFO: boolP(true)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(true)},
		&Option{BufferSize: intP(1000)},
	)

	l7, r7 := m7.Builder().Fork("fork_id", ForkDuplicate)

	l7.Map("map_id", func(m data.Data) error {
		if _, ok := m["name"]; !ok {
			b.Errorf("packet missing name %v", m)
			return fmt.Errorf("incorrect data have %v want %v", m, "name field")
		}
		return nil
	}).Publish("sender_id",
		publishFN(func(d []data.Data) error {
			b.Error("unexpected")
			return nil
		}),
	)

	r7.FoldLeft("fold_id2", func(d1, d2 data.Data) data.Data {
		return d1
	})

	if err := m.Run(context.Background(), time.Second); err == nil {
		b.Error("expected error m")
	}

	if err := m2.Run(context.Background(), time.Second); err == nil {
		b.Error("expected error m2")
	}

	if err := m3.Run(context.Background(), time.Second); err == nil {
		b.Error("expected error m3")
	}

	if err := m4.Run(context.Background(), time.Second); err == nil {
		b.Error("expected error m4")
	}

	if err := m5.Run(context.Background(), time.Second); err == nil {
		b.Error("expected error m5")
	}

	if err := m6.Run(context.Background(), time.Second); err == nil {
		b.Error("expected error m6")
	}

	if err := m7.Run(context.Background(), time.Second); err == nil {
		b.Error("expected error m7")
	}
}

func Test_Inject(b *testing.T) {
	count := 1000
	channel := make(chan []data.Data)
	out := make(chan []data.Data)
	m := NewStream("machine_id", func(c context.Context) chan []data.Data {
		go func() {
			for n := 0; n < count; n++ {
				channel <- deepCopy(testListBase)
				channel <- nil
			}
		}()
		return channel
	},
		&Option{FIFO: boolP(false)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(true)},
		&Option{BufferSize: intP(0)},
	)

	left, right := m.Builder().
		Map("map_id", func(m data.Data) error {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
				return fmt.Errorf("incorrect data have %v want %v", m, "name field")
			}
			return fmt.Errorf("error")
		},
			&Option{FIFO: boolP(false)},
			&Option{Injectable: boolP(true)},
			&Option{Metrics: boolP(true)},
			&Option{Span: boolP(false)},
			&Option{BufferSize: intP(0)},
		).
		FoldLeft("fold_idx", func(d1, d2 data.Data) data.Data {
			return d1
		}, &Option{Injectable: boolP(false)}).
		FoldLeft("fold_id1", func(d1, d2 data.Data) data.Data {
			return d1
		},
			&Option{FIFO: boolP(false)},
			&Option{Injectable: boolP(true)},
			&Option{Metrics: boolP(true)},
			&Option{Span: boolP(false)},
			&Option{BufferSize: intP(0)}).
		FoldRight("fold_id2", func(d1, d2 data.Data) data.Data {
			return d1
		},
			&Option{FIFO: boolP(false)},
			&Option{Injectable: boolP(true)},
			&Option{Metrics: boolP(true)},
			&Option{Span: boolP(false)},
			&Option{BufferSize: intP(0)},
		).
		Fork("fork_id", ForkError,
			&Option{FIFO: boolP(false)},
			&Option{Injectable: boolP(true)},
			&Option{Metrics: boolP(true)},
			&Option{Span: boolP(false)},
			&Option{BufferSize: intP(0)},
		)

	left.Publish("sender_id",
		publishFN(func(d []data.Data) error {
			b.Error("unexpected")
			return nil
		}),
		&Option{FIFO: boolP(false)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(false)},
		&Option{BufferSize: intP(0)},
	)

	right.Publish("sender_id",
		publishFN(func(d []data.Data) error {
			out <- d
			return nil
		}),
	)

	if err := m.Run(context.Background(), time.Second); err != nil {
		b.Error(err)
	}

	cb := m.InjectionCallback(context.Background())

	go func() {
		for n := 0; n < count; n++ {
			cb(&Log{
				StreamID:   "machine_id",
				VertexID:   "map_id",
				VertexType: "map",
				State:      "start",
				When:       time.Now(),
				Packet:     deepCopyList(testPayloadBase)[0],
			}, &Log{
				StreamID:   "machine_id",
				VertexID:   "fold_idx",
				VertexType: "fold",
				State:      "start",
				When:       time.Now(),
				Packet:     deepCopyList(testPayloadBase)[0],
			})
		}
	}()

	for n := 0; n < 2*count; n++ {
		list := <-out

		if len(list) != 1 {
			b.Errorf("incorrect data have %v want %v", list, testListBase[0])
		}
	}
}

func Test_Loop(b *testing.T) {
	count := 100000
	out := make(chan []data.Data)
	m := NewStream("machine_id", func(c context.Context) chan []data.Data {
		channel := make(chan []data.Data)
		go func() {
			for n := 0; n < count; n++ {
				channel <- deepCopy(testListBase)
			}
		}()
		return channel
	},
		&Option{FIFO: boolP(false)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(false)},
		&Option{BufferSize: intP(0)},
	)

	left, right := m.Builder().
		Map("map_id", func(m data.Data) error {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
				return fmt.Errorf("incorrect data have %v want %v", m, "name field")
			}
			return nil
		}).
		Loop("loop_id",
			func(list []*Packet) (a []*Packet, b []*Packet) {
				a = []*Packet{}
				b = []*Packet{}

				for i, item := range list {
					if i == 0 {
						b = append(b, item)
					} else {
						a = append(a, item)
					}
				}

				return
			},
			&Option{FIFO: boolP(false)},
			&Option{Injectable: boolP(true)},
			&Option{Metrics: boolP(true)},
			&Option{Span: boolP(true)},
			&Option{BufferSize: intP(0)},
		)

	left2, _ := left.
		Loop("loop2_id",
			func(list []*Packet) (a []*Packet, b []*Packet) {
				a = []*Packet{}
				b = []*Packet{}

				for i, item := range list {
					if i == 0 {
						b = append(b, item)
					} else {
						a = append(a, item)
					}
				}

				return
			},
		)

	left3, _ := left2.Fork("fork_id", ForkError)

	_, right2 := left3.Fork("fork_id",
		func(list []*Packet) (a []*Packet, b []*Packet) {
			return []*Packet{}, list
		},
	)

	left4, right3 := right2.Fork("fork_id",
		func(list []*Packet) (a []*Packet, b []*Packet) {
			return []*Packet{}, list
		},
	)

	left5, right4 := left4.Fork("fork_id", ForkError)
	left5.FoldLeft("fold_id1", func(d1, d2 data.Data) data.Data {
		return d1
	})

	right4.FoldRight("fold_id1", func(d1, d2 data.Data) data.Data {
		return d1
	})

	right3.Map("map_id2", func(m data.Data) error {
		if _, ok := m["name"]; !ok {
			b.Errorf("packet missing name %v", m)
			return fmt.Errorf("incorrect data have %v want %v", m, "name field")
		}
		return nil
	})

	right.Publish("sender_id",
		publishFN(func(d []data.Data) error {
			out <- d
			return nil
		}),
	)

	if err := m.Run(context.Background(), time.Second); err != nil {
		b.Error(err)
	}

	for n := 0; n < count; n++ {
		list := <-out

		if len(list) != 1 {
			b.Errorf("incorrect data have %v want %v", list, testListBase[0])
		}
	}
}

func Test_Pipe_Sub(b *testing.T) {
	count := 100
	out := make(chan []data.Data)

	t := &tester{}

	s := NewSubscriptionStream("stream_id", t, 5*time.Millisecond,
		&Option{DeepCopy: boolP(true)},
		&Option{FIFO: boolP(false)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(false)},
		&Option{BufferSize: intP(0)},
	)

	s.Builder().Publish("publish_id",
		publishFN(func(d []data.Data) error {
			out <- d
			return nil
		}),
	)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		if err := s.Run(ctx, time.Second); err != nil {
			b.Error(err)
		}
	}()

	for n := 0; n < count; n++ {
		list := <-out

		if len(list) != 10 && len(list) != 1 {
			b.Errorf("incorrect data have %v want %v", list, testListBase[0])
		}
	}

	o := []data.Data{}
	buf := &bytes.Buffer{}
	enc, dec := gob.NewEncoder(buf), gob.NewDecoder(buf)

	_ = enc.Encode(&testListBase)
	_ = dec.Decode(&o)

	if len(o) != 10 {
		b.Error("len of injection wrong")
	}

	cancel()
	<-time.After(time.Second)
}

func Test_Pipe_HTTP(b *testing.T) {
	out := make(chan []data.Data)

	app := fiber.New()
	defer app.Shutdown()

	s := NewHTTPStream("http_id",
		&Option{DeepCopy: boolP(true)},
		&Option{FIFO: boolP(true)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(false)},
		&Option{BufferSize: intP(0)},
	)

	s.Builder().
		Map("map_id",
			func(m data.Data) error {
				if _, ok := m["name"]; !ok {
					b.Errorf("packet missing name %v", m)
					return fmt.Errorf("incorrect data have %v want %v", m, "name field")
				}
				return nil
			},
		).Publish("publish_id",
		publishFN(func(d []data.Data) error {
			out <- d
			return nil
		}),
	)

	app.Post("/test", s.Handler())

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		if err := s.Run(ctx, time.Second, &tester{}); err != nil {
			b.Error(err)
		}

		app.Listen("localhost:5000")
	}()

	bytez, _ := json.Marshal(deepCopy(testListBase))
	req, _ := http.NewRequest(http.MethodPost, "http://localhost:5000/test", bytes.NewReader(bytez))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)

	if resp.StatusCode != http.StatusAccepted || err != nil {
		b.Error(resp.StatusCode, err)
	}

	list := <-out

	if len(list) != 10 {
		b.Errorf("incorrect data have %v want %v", list, testListBase)
	}

	bytez, _ = json.Marshal(deepCopy(testListBase)[0])
	req, _ = http.NewRequest(http.MethodPost, "http://localhost:5000/test", bytes.NewReader(bytez))
	req.Header.Set("Content-Type", "application/json")

	resp, err = app.Test(req, -1)

	if resp.StatusCode != http.StatusAccepted || err != nil {
		b.Error(resp.StatusCode, err)
	}

	list = <-out

	if len(list) != 1 {
		b.Errorf("incorrect data have %v want %v", list, testListBase)
	}

	cancel()
	<-time.After(time.Second)
}

func Test_Pipe_Websocket(b *testing.T) {
	out := make(chan []data.Data)

	app := fiber.New()
	defer app.Shutdown()

	s := NewWebsocketStream("websocket_id",
		&Option{DeepCopy: boolP(true)},
		&Option{FIFO: boolP(true)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(false)},
		&Option{BufferSize: intP(0)},
	)

	s.Builder().Publish("publish_id",
		publishFN(func(d []data.Data) error {
			out <- d
			return nil
		}),
	)

	app.Get("/test", s.Handler())

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		if err := s.Run(ctx, time.Second); err != nil {
			b.Error(err)
		}
		app.Listen("localhost:5000")
	}()

	conn, resp, err := websocket.DefaultDialer.Dial("ws://localhost:5000/test", http.Header{})
	defer conn.Close()

	if err != nil || resp.StatusCode != fiber.StatusSwitchingProtocols {
		b.Error(err)
	}

	if err := conn.WriteJSON(deepCopy(testListBase)); err != nil {
		b.Error(err)
	}

	if err := conn.WriteJSON(deepCopy(testListBase)); err != nil {
		b.Error(err)
	}

	list := <-out
	if len(list) != 10 {
		b.Errorf("incorrect data have %v want %v", list, testListBase)
	}

	list = <-out
	if len(list) != 10 {
		b.Errorf("incorrect data have %v want %v", list, testListBase)
	}

	if err := conn.WriteJSON(deepCopy(testListBase)); err != nil {
		b.Error(err)
	}

	list = <-out
	if len(list) != 10 {
		b.Errorf("incorrect data have %v want %v", list, testListBase)
	}

	cancel()
	<-time.After(time.Second)
}
