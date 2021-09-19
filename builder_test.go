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
	"strings"
	"testing"
	"time"

	"github.com/fasthttp/websocket"
	"github.com/gofiber/fiber/v2"

	"github.com/whitaker-io/data"
)

var testListInvalidBase = []data.Data{
	{
		"__traceID": "test_trace_id",
		"name":      "data0",
	},
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
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(false)},
		&Option{BufferSize: intP(0)},
	)

	m.Builder().
		Map("map_id", func(m data.Data) data.Data {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
			}
			return m
		}).
		Publish("sender_id",
			publishFN(func(d []data.Data) error {
				out <- d
				return nil
			}),
		)

	if err := m.Run(context.Background()); err != nil {
		b.Error(err)
		b.FailNow()
	}

	for n := 0; n < b.N; n++ {
		go func() {
			channel <- testListBase
		}()

		list := <-out

		if len(list) != len(testListBase) {
			b.Errorf("incorrect data have %v want %v", list, testListBase)
			b.FailNow()
		}
	}
}

func Test_New(b *testing.T) {
	count := 1000
	out := make(chan []data.Data)
	m := NewStream("machine_id", func(c context.Context) chan []data.Data {
		channel := make(chan []data.Data)
		go func() {
			channel <- deepCopy(testListInvalidBase)
			for n := 0; n < count; n++ {
				channel <- deepCopy(testListBase)
			}
		}()
		return channel
	},
		&Option{FIFO: boolP(false)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(false)},
		&Option{BufferSize: intP(0)},
		&Option{Validators: map[string]ForkRule{
			"name-test": func(d data.Data) bool {
				if _, err := d.String("name"); err != nil {
					return false
				}

				if _, err := d.Int("value"); err != nil {
					return false
				}

				return true
			},
		}},
	)

	left, right := m.Builder().
		Window("window_id", func(list ...*Packet) []*Packet {
			return list
		}).
		Map("map_id", func(m data.Data) data.Data {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
			}
			return m
		}).
		Sort("sort_id1", func(a, b data.Data) int {
			return strings.Compare(a.MustString("name"), b.MustString("name"))
		}).
		Remove("remove_id1", func(index int, d data.Data) bool {
			return false
		}).
		FoldLeft("fold_id1", func(d1, d2 data.Data) data.Data {
			return d1
		}).
		FoldLeft("fold_id2", func(d1, d2 data.Data) data.Data {
			return d1
		}).
		FoldRight("fold_id3", func(d1, d2 data.Data) data.Data {
			return d1
		}).
		Fork("fork_id", ForkError)

	left.Publish("sender_id",
		publishFN(func(d []data.Data) error {
			out <- d
			return nil
		}),
	)

	right.Publish("sender_id2",
		publishFN(func(d []data.Data) error {
			b.Error("unexpected")
			b.FailNow()
			return nil
		}),
	)

	if err := m.Run(context.Background()); err != nil {
		b.Error(err)
		b.FailNow()
	}

	for n := 0; n < count; n++ {
		list := <-out

		if len(list) != 1 {
			b.Errorf("incorrect data have %v want %v", list, testListBase[0])
			b.FailNow()
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
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(true)},
		&Option{BufferSize: intP(1000)},
	)

	left, right := m.Builder().
		Map("map_id", func(m data.Data) data.Data {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
			}
			return m
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

	l2, r2 := right.Fork("fork_id1", ForkRule(func(d data.Data) bool {
		return true
	}).Handler)

	l3, r3 := l2.Fork("fork_id2", ForkRule(func(d data.Data) bool {
		return false
	}).Handler)

	r2.Publish("sender_id2",
		publishFN(func(d []data.Data) error {
			b.Error("unexpected")
			b.FailNow()
			return nil
		}),
	)

	l3.Publish("sender_id3",
		publishFN(func(d []data.Data) error {
			b.Error("unexpected")
			b.FailNow()
			return nil
		}),
	)

	r3.Publish("sender_id4",
		publishFN(func(d []data.Data) error {
			out <- d
			return fmt.Errorf("error")
		}),
	)

	if err := m.Run(context.Background()); err != nil {
		b.Error(err)
		b.FailNow()
	}

	if m.ID() != "machine_id" {
		b.Errorf("incorrect id have %v want %v", m.ID(), "machine_id")
		b.FailNow()
	}

	for n := 0; n < 2*count; n++ {
		list := <-out

		if len(list) != 1 {
			b.Errorf("incorrect data have %v want %v", list, testListBase[9])
			b.FailNow()
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
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(true)},
		&Option{BufferSize: intP(1000)},
	)

	left, right := m.Builder().
		Map("map_id", func(m data.Data) data.Data {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
			}
			return m
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

	l2, r2 := right.Fork("fork_id1", ForkRule(func(d data.Data) bool {
		return true
	}).Handler)

	l3, r3 := l2.Fork("fork_id2", ForkRule(func(d data.Data) bool {
		return false
	}).Handler)

	r2.Publish("sender_id2",
		publishFN(func(d []data.Data) error {
			b.Error("unexpected")
			return nil
		}),
	)

	l3.Publish("sender_id3",
		publishFN(func(d []data.Data) error {
			b.Error("unexpected")
			return nil
		}),
	)

	r3.Publish("sender_id4", publishFN(func(d []data.Data) error {
		panicCount++

		if panicCount%2 == 0 {
			panic("test panic")
		}

		out <- d
		return fmt.Errorf("error")
	}),
	)

	if err := m.Run(context.Background()); err != nil {
		b.Error(err)
		b.FailNow()
	}

	if m.ID() != "machine_id" {
		b.Errorf("incorrect id have %v want %v", m.ID(), "machine_id")
		b.FailNow()
	}

	for n := 0; n < count; n++ {
		list := <-out

		if len(list) != 1 {
			b.Errorf("incorrect data have %v want %v", list, testListBase[9])
			b.FailNow()
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
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(true)},
		&Option{BufferSize: intP(1000)},
	)

	m4 := NewStream("machine_id", func(c context.Context) chan []data.Data {
		channel := make(chan []data.Data)
		return channel
	},
		&Option{FIFO: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(true)},
		&Option{BufferSize: intP(1000)},
	)

	m4.Builder().Map("map_id", func(m data.Data) data.Data {
		if _, ok := m["name"]; !ok {
			b.Errorf("packet missing name %v", m)
		}
		return m
	})

	m5 := NewStream("machine_id", func(c context.Context) chan []data.Data {
		channel := make(chan []data.Data)
		return channel
	},
		&Option{FIFO: boolP(true)},
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
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(true)},
		&Option{BufferSize: intP(1000)},
	)

	l6, r6 := m6.Builder().Fork("fork_id", ForkDuplicate)

	l6.Map("map_id", func(m data.Data) data.Data {
		if _, ok := m["name"]; !ok {
			b.Errorf("packet missing name %v", m)
		}
		return m
	})

	r6.Map("map_id", func(m data.Data) data.Data {
		if _, ok := m["name"]; !ok {
			b.Errorf("packet missing name %v", m)
		}
		return m
	})

	m7 := NewStream("machine_id", func(c context.Context) chan []data.Data {
		channel := make(chan []data.Data)
		return channel
	},
		&Option{FIFO: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(true)},
		&Option{BufferSize: intP(1000)},
	)

	l7, r7 := m7.Builder().Fork("fork_id", ForkDuplicate)

	l7.Map("map_id", func(m data.Data) data.Data {
		if _, ok := m["name"]; !ok {
			b.Errorf("packet missing name %v", m)
		}
		return m
	}).
		Publish("sender_id",
			publishFN(func(d []data.Data) error {
				b.Error("unexpected")
				return nil
			}),
		)

	r7.FoldLeft("fold_id2", func(d1, d2 data.Data) data.Data {
		return d1
	})

	m8 := NewStream("machine_id", func(c context.Context) chan []data.Data {
		channel := make(chan []data.Data)
		return channel
	},
		&Option{FIFO: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(true)},
		&Option{BufferSize: intP(1000)},
	)

	m8.Builder().Loop("loop_id", func(list []*Packet) (a []*Packet, b []*Packet) {
		return []*Packet{}, list
	})

	m9 := NewStream("machine_id", func(c context.Context) chan []data.Data {
		channel := make(chan []data.Data)
		return channel
	},
		&Option{FIFO: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(true)},
		&Option{BufferSize: intP(1000)},
	)

	m9.Builder().Sort("sort_id", func(a, b data.Data) int {
		return 0
	})

	m10 := NewStream("machine_id", func(c context.Context) chan []data.Data {
		channel := make(chan []data.Data)
		return channel
	},
		&Option{FIFO: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(true)},
		&Option{BufferSize: intP(1000)},
	)

	m10.Builder().Remove("remove_id", func(index int, d data.Data) bool {
		return true
	})

	if err := m.Run(context.Background()); err == nil {
		b.Error("expected error m")
		b.FailNow()
	}

	if err := m2.Run(context.Background()); err == nil {
		b.Error("expected error m2")
		b.FailNow()
	}

	if err := m3.Run(context.Background()); err == nil {
		b.Error("expected error m3")
		b.FailNow()
	}

	if err := m4.Run(context.Background()); err == nil {
		b.Error("expected error m4")
		b.FailNow()
	}

	if err := m5.Run(context.Background()); err == nil {
		b.Error("expected error m5")
		b.FailNow()
	}

	if err := m6.Run(context.Background()); err == nil {
		b.Error("expected error m6")
		b.FailNow()
	}

	if err := m7.Run(context.Background()); err == nil {
		b.Error("expected error m7")
		b.FailNow()
	}

	if err := m8.Run(context.Background()); err == nil {
		b.Error("expected error m8")
		b.FailNow()
	}

	if err := m9.Run(context.Background()); err == nil {
		b.Error("expected error m9")
		b.FailNow()
	}

	if err := m10.Run(context.Background()); err == nil {
		b.Error("expected error m10")
		b.FailNow()
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
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(true)},
		&Option{BufferSize: intP(0)},
	)

	left, right := m.Builder().
		Map("map_id", func(m data.Data) data.Data {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
			}
			return m
		}).
		FoldLeft("fold_idx", func(d1, d2 data.Data) data.Data {
			return d1
		}).
		FoldLeft("fold_id1", func(d1, d2 data.Data) data.Data {
			return d1
		}).
		FoldRight("fold_id2", func(d1, d2 data.Data) data.Data {
			return d1
		}).
		Fork("fork_id", func(list []*Packet) (a []*Packet, b []*Packet) {
			return []*Packet{}, list
		})

	left.Publish("sender_id",
		publishFN(func(d []data.Data) error {
			b.Error("unexpected")
			return nil
		}))

	right.Publish("sender_id2",
		publishFN(func(d []data.Data) error {
			out <- d
			return nil
		}),
	)

	if err := m.Run(context.Background()); err != nil {
		b.Error(err)
		b.FailNow()
	}

	go func() {
		for n := 0; n < count; n++ {
			m.Inject("map_id", deepCopyList(testPayloadBase)[0])
		}
	}()

	for n := 0; n < 2*count; n++ {
		list := <-out

		if len(list) != 1 {
			b.Errorf("incorrect data have %v want %v", list, testListBase[0])
			b.FailNow()
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
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(false)},
		&Option{BufferSize: intP(0)},
	)

	left, right := m.Builder().
		Map("map_id", func(m data.Data) data.Data {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
				b.FailNow()
			}
			return m
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
			})

	inside, _ := left.Loop("loop_id2",
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

	inside.Map("map_id2", func(m data.Data) data.Data {
		if _, ok := m["name"]; !ok {
			b.Errorf("packet missing name %v", m)
			b.FailNow()
		}
		return m
	})

	right.Publish("sender_id",
		publishFN(func(d []data.Data) error {
			out <- d
			return nil
		}),
	)

	if err := m.Run(context.Background()); err != nil {
		b.Error(err)
		b.FailNow()
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
		if err := s.Run(ctx); err != nil {
			b.Error(err)
		}
	}()

	for n := 0; n < count; n++ {
		list := <-out

		if len(list) != 10 && len(list) != 1 {
			b.Errorf("incorrect data have %v want %v", list, testListBase[0])
			b.FailNow()
		}
	}

	o := []data.Data{}
	buf := &bytes.Buffer{}
	enc, dec := gob.NewEncoder(buf), gob.NewDecoder(buf)

	_ = enc.Encode(&testListBase)
	_ = dec.Decode(&o)

	if len(o) != 10 {
		b.Error("len of injection wrong")
		b.FailNow()
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
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(false)},
		&Option{BufferSize: intP(0)},
	)

	s.Builder().
		Map("map_id", func(m data.Data) data.Data {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
				b.FailNow()
			}
			return m
		}).
		Publish("publish_id",
			publishFN(func(d []data.Data) error {
				out <- d
				return nil
			}),
		)

	ctx, cancel := context.WithCancel(context.Background())

	if err := s.Run(ctx); err != nil {
		b.Error(err)
		b.FailNow()
	}

	go func() {
		app.Listen("localhost:5000")
	}()

	app.Post("/test", s.Handler())
	app.Post("/test/map_id", s.InjectionHandlers()["map_id"])

	bytez, _ := json.Marshal(deepCopy(testListBase))
	req, _ := http.NewRequest(http.MethodPost, "http://localhost:5000/test", bytes.NewReader(bytez))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req, -1)

	if resp.StatusCode != http.StatusAccepted || err != nil {
		b.Error(resp.StatusCode, err)
		b.FailNow()
	}

	list := <-out

	if len(list) != 10 {
		b.Errorf("incorrect data have %v want %v", list, testListBase)
		b.FailNow()
	}

	bytez, _ = json.Marshal(deepCopy(testListBase)[0])
	req, _ = http.NewRequest(http.MethodPost, "http://localhost:5000/test", bytes.NewReader(bytez))
	req.Header.Set("Content-Type", "application/json")

	resp, err = app.Test(req, -1)

	if resp.StatusCode != http.StatusAccepted || err != nil {
		b.Error(resp.StatusCode, err)
		b.FailNow()
	}

	list = <-out

	if len(list) != 1 {
		b.Errorf("incorrect data have %v want %v", list, testListBase)
		b.FailNow()
	}

	bytez, _ = json.Marshal(deepCopyPayload(testPayloadBase))
	req, _ = http.NewRequest(http.MethodPost, "http://localhost:5000/test/map_id", bytes.NewReader(bytez))
	req.Header.Set("Content-Type", "application/json")

	resp, err = app.Test(req, -1)

	if resp.StatusCode != http.StatusAccepted || err != nil {
		b.Error(resp.StatusCode, err)
		b.FailNow()
	}

	list = <-out

	if len(list) != 10 {
		b.Errorf("incorrect data have %v want %v", list, testListBase)
		b.FailNow()
	}

	bytez, _ = json.Marshal(deepCopyPayload(testPayloadBase)[0])
	req, _ = http.NewRequest(http.MethodPost, "http://localhost:5000/test/map_id", bytes.NewReader(bytez))
	req.Header.Set("Content-Type", "application/json")

	resp, err = app.Test(req, -1)

	if resp.StatusCode != http.StatusAccepted || err != nil {
		b.Error(resp.StatusCode, err)
		b.FailNow()
	}

	list = <-out

	if len(list) != 1 {
		b.Errorf("incorrect data have %v want %v", list, testListBase)
		b.FailNow()
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
		if err := s.Run(ctx); err != nil {
			b.Error(err)
		}
		app.Listen("localhost:5001")
	}()

	conn, resp, err := websocket.DefaultDialer.Dial("ws://localhost:5001/test", http.Header{})
	defer func() {
		if err := conn.Close(); err != nil {
			fmt.Printf("error closing websocket - %v", err)
		}
	}()

	if err != nil || resp.StatusCode != fiber.StatusSwitchingProtocols {
		b.Error(err)
		b.FailNow()
	}

	if err := conn.WriteJSON(deepCopy(testListBase)); err != nil {
		b.Error(err)
		b.FailNow()
	}

	if err := conn.WriteJSON(deepCopy(testListBase)); err != nil {
		b.Error(err)
		b.FailNow()
	}

	list := <-out
	if len(list) != 10 {
		b.Errorf("incorrect data have %v want %v", list, testListBase)
		b.FailNow()
	}

	list = <-out
	if len(list) != 10 {
		b.Errorf("incorrect data have %v want %v", list, testListBase)
		b.FailNow()
	}

	if err := conn.WriteJSON(deepCopy(testListBase)); err != nil {
		b.Error(err)
		b.FailNow()
	}

	list = <-out
	if len(list) != 10 {
		b.Errorf("incorrect data have %v want %v", list, testListBase)
		b.FailNow()
	}

	cancel()
	<-time.After(time.Second)
}
