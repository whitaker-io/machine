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
	"testing"
	"time"

	"github.com/karlseguin/typed"
)

var testList = []Data{
	{
		"name":  "data0",
		"value": 0,
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

var testPayload = []*Packet{
	{
		ID: "ID_0",
		Data: Data{
			"name":  "data0",
			"value": 0,
		},
	},
	{
		ID: "ID_1",
		Data: Data{
			"name":  "data1",
			"value": 1,
		},
	},
	{
		ID: "ID_2",
		Data: Data{
			"name":  "data2",
			"value": 2,
		},
	},
	{
		ID: "ID_3",
		Data: Data{
			"name":  "data3",
			"value": 3,
		},
	},
	{
		ID: "ID_4",
		Data: Data{
			"name":  "data4",
			"value": 4,
		},
	},
	{
		ID: "ID_5",
		Data: Data{
			"name":  "data5",
			"value": 5,
		},
	},
	{
		ID: "ID_6",
		Data: Data{
			"name":  "data6",
			"value": 6,
		},
	},
	{
		ID: "ID_7",
		Data: Data{
			"name":  "data7",
			"value": 7,
		},
	},
	{
		ID: "ID_8",
		Data: Data{
			"name":  "data8",
			"value": 8,
		},
	},
	{
		ID: "ID_9",
		Data: Data{
			"name":  "data9",
			"value": 9,
		},
	},
}

var bufferSize = 0

func Benchmark_Test_New(b *testing.B) {
	out := make(chan []Data)
	channel := make(chan []Data)
	m := NewStream("machine_id", func(c context.Context) chan []Data {
		return channel
	},
		&Option{FIFO: boolP(false)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(false)},
		&Option{BufferSize: intP(0)},
	)

	m.Builder().
		Map("map_id", func(m Data) error {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
				return fmt.Errorf("incorrect data have %v want %v", m, "name field")
			}
			return nil
		}).
		Transmit("sender_id", func(d []Data) error {
			out <- d
			return nil
		})

	if err := m.Run(context.Background()); err != nil {
		b.Error(err)
	}

	for n := 0; n < b.N; n++ {
		go func() {
			channel <- testList
		}()

		list := <-out

		if len(list) != len(testList) {
			b.Errorf("incorrect data have %v want %v", list, testList)
		}
	}
}

func Test_New(b *testing.T) {
	count := 100000
	out := make(chan []Data)
	m := NewStream("machine_id", func(c context.Context) chan []Data {
		channel := make(chan []Data)
		go func() {
			for n := 0; n < count; n++ {
				channel <- testList
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
		Map("map_id", func(m Data) error {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
				return fmt.Errorf("incorrect data have %v want %v", m, "name field")
			}
			return nil
		}).
		FoldLeft("fold_id1", func(d1, d2 Data) Data {
			return d1
		}).
		FoldLeft("fold_id1", func(d1, d2 Data) Data {
			return d1
		}).
		FoldRight("fold_id2", func(d1, d2 Data) Data {
			return d1
		}).
		Fork("fork_id", ForkError)

	left.Transmit("sender_id", func(d []Data) error {
		out <- d
		return nil
	})

	right.Transmit("sender_id", func(d []Data) error {
		b.Error("unexpected")
		return nil
	})

	if err := m.Run(context.Background()); err != nil {
		b.Error(err)
	}

	for n := 0; n < count; n++ {
		list := <-out

		if len(list) != 1 {
			b.Errorf("incorrect data have %v want %v", list, testList[0])
		}
	}
}

func Test_New2(b *testing.T) {
	count := 1000
	out := make(chan []Data)
	m := NewStream("machine_id", func(c context.Context) chan []Data {
		channel := make(chan []Data)
		go func() {
			for n := 0; n < count; n++ {
				channel <- testList
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
		Map("map_id", func(m Data) error {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
				return fmt.Errorf("incorrect data have %v want %v", m, "name field")
			}
			return nil
		}).
		FoldRight("fold_id2", func(d1, d2 Data) Data {
			return d1
		}).
		Fork("fork_id", ForkDuplicate)

	left.Transmit("sender_id", func(d []Data) error {
		out <- d
		return nil
	})

	l2, r2 := right.Fork("fork_id2", ForkRule(func(d Data) bool {
		return true
	}).Handler)

	l3, r3 := l2.Fork("fork_id3", ForkRule(func(d Data) bool {
		return false
	}).Handler)

	r2.Transmit("sender_id", func(d []Data) error {
		b.Error("unexpected")
		return nil
	})

	l3.Transmit("sender_id", func(d []Data) error {
		b.Error("unexpected")
		return nil
	})

	r3.Transmit("sender_id", func(d []Data) error {
		out <- d
		return fmt.Errorf("error")
	})

	if err := m.Run(context.Background(), func(s1, s2, s3 string, p []*Packet) {}); err != nil {
		b.Error(err)
	}

	if m.ID() != "machine_id" {
		b.Errorf("incorrect id have %v want %v", m.ID(), "machine_id")
	}

	for n := 0; n < 2*count; n++ {
		list := <-out

		if len(list) != 1 {
			b.Errorf("incorrect data have %v want %v", list, testList[9])
		}
	}
}

func Test_Missing_Leaves(b *testing.T) {
	m := NewStream("machine_id", func(c context.Context) chan []Data {
		channel := make(chan []Data)
		return channel
	})

	left, _ := m.Builder().Fork("fork_id", ForkDuplicate)

	left.Transmit("sender_id", func(d []Data) error {
		b.Error("unexpected")
		return nil
	})

	m2 := NewStream("machine_id", func(c context.Context) chan []Data {
		channel := make(chan []Data)
		return channel
	},
		&Option{FIFO: boolP(true)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(true)},
		&Option{BufferSize: intP(1000)},
	)

	m2.Builder().Fork("fork_id", ForkDuplicate)

	m3 := NewStream("machine_id", func(c context.Context) chan []Data {
		channel := make(chan []Data)
		return channel
	},
		&Option{FIFO: boolP(true)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(true)},
		&Option{BufferSize: intP(1000)},
	)

	m4 := NewStream("machine_id", func(c context.Context) chan []Data {
		channel := make(chan []Data)
		return channel
	},
		&Option{FIFO: boolP(true)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(true)},
		&Option{BufferSize: intP(1000)},
	)

	m4.Builder().Map("map_id", func(m Data) error {
		if _, ok := m["name"]; !ok {
			b.Errorf("packet missing name %v", m)
			return fmt.Errorf("incorrect data have %v want %v", m, "name field")
		}
		return nil
	})

	m5 := NewStream("machine_id", func(c context.Context) chan []Data {
		channel := make(chan []Data)
		return channel
	},
		&Option{FIFO: boolP(true)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(true)},
		&Option{BufferSize: intP(1000)},
	)

	m5.Builder().FoldRight("fold_id2", func(d1, d2 Data) Data {
		return d1
	})

	m6 := NewStream("machine_id", func(c context.Context) chan []Data {
		channel := make(chan []Data)
		return channel
	},
		&Option{FIFO: boolP(true)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(true)},
		&Option{BufferSize: intP(1000)},
	)

	l6, r6 := m6.Builder().Fork("fork_id", ForkDuplicate)

	l6.Map("map_id", func(m Data) error {
		if _, ok := m["name"]; !ok {
			b.Errorf("packet missing name %v", m)
			return fmt.Errorf("incorrect data have %v want %v", m, "name field")
		}
		return nil
	})

	r6.Map("map_id", func(m Data) error {
		if _, ok := m["name"]; !ok {
			b.Errorf("packet missing name %v", m)
			return fmt.Errorf("incorrect data have %v want %v", m, "name field")
		}
		return nil
	})

	m7 := NewStream("machine_id", func(c context.Context) chan []Data {
		channel := make(chan []Data)
		return channel
	},
		&Option{FIFO: boolP(true)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(true)},
		&Option{BufferSize: intP(1000)},
	)

	l7, r7 := m7.Builder().Fork("fork_id", ForkDuplicate)

	l7.Map("map_id", func(m Data) error {
		if _, ok := m["name"]; !ok {
			b.Errorf("packet missing name %v", m)
			return fmt.Errorf("incorrect data have %v want %v", m, "name field")
		}
		return nil
	}).Transmit("sender_id", func(d []Data) error {
		b.Error("unexpected")
		return nil
	})

	r7.FoldLeft("fold_id2", func(d1, d2 Data) Data {
		return d1
	})

	if err := m.Run(context.Background()); err == nil {
		b.Error("expected error m")
	}

	if err := m2.Run(context.Background()); err == nil {
		b.Error("expected error m2")
	}

	if err := m3.Run(context.Background()); err == nil {
		b.Error("expected error m3")
	}

	if err := m4.Run(context.Background()); err == nil {
		b.Error("expected error m4")
	}

	if err := m5.Run(context.Background()); err == nil {
		b.Error("expected error m5")
	}

	if err := m6.Run(context.Background()); err == nil {
		b.Error("expected error m6")
	}

	if err := m7.Run(context.Background()); err == nil {
		b.Error("expected error m7")
	}
}

func Test_Inject(b *testing.T) {
	count := 1000
	channel := make(chan []Data)
	out := make(chan []Data)
	m := NewStream("machine_id", func(c context.Context) chan []Data {
		go func() {
			for n := 0; n < count; n++ {
				channel <- testList
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
		Map("map_id", func(m Data) error {
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
		FoldLeft("fold_id1", func(d1, d2 Data) Data {
			return d1
		}).
		FoldLeft("fold_id1", func(d1, d2 Data) Data {
			return d1
		},
			&Option{FIFO: boolP(false)},
			&Option{Injectable: boolP(true)},
			&Option{Metrics: boolP(true)},
			&Option{Span: boolP(false)},
			&Option{BufferSize: intP(0)}).
		FoldRight("fold_id2", func(d1, d2 Data) Data {
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

	left.Transmit("sender_id", func(d []Data) error {
		b.Error("unexpected")
		return nil
	},
		&Option{FIFO: boolP(false)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(false)},
		&Option{BufferSize: intP(0)},
	)

	right.Transmit("sender_id", func(d []Data) error {
		out <- d
		return nil
	})

	if err := m.Run(context.Background()); err != nil {
		b.Error(err)
	}

	go func() {
		for n := 0; n < count; n++ {
			m.Inject(context.Background(), map[string][]*Packet{
				"map_id": testPayload,
			})
		}
	}()

	for n := 0; n < 2*count; n++ {
		list := <-out

		if len(list) != 1 {
			b.Errorf("incorrect data have %v want %v", list, testList[0])
		}
	}
}

func Test_Inject_Cancel(b *testing.T) {
	count := 1000
	channel := make(chan []Data)
	out := make(chan []Data)
	m := NewStream("machine_id", func(c context.Context) chan []Data {
		go func() {
			for n := 0; n < count; n++ {
				channel <- testList
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
		Map("map_id", func(m Data) error {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
				return fmt.Errorf("incorrect data have %v want %v", m, "name field")
			}
			return fmt.Errorf("error")
		}).
		FoldLeft("fold_id1", func(d1, d2 Data) Data {
			return d1
		}).
		FoldLeft("fold_id1", func(d1, d2 Data) Data {
			return d1
		}).
		FoldRight("fold_id2", func(d1, d2 Data) Data {
			return d1
		}).
		Fork("fork_id", ForkError)

	left.Transmit("sender_id", func(d []Data) error {
		b.Error("unexpected")
		return nil
	})

	right.Transmit("sender_id", func(d []Data) error {
		out <- d
		return nil
	})

	ctx, cancel := context.WithCancel(context.Background())

	if err := m.Run(ctx); err != nil {
		b.Error(err)
	}

	go func() {
		for n := 0; n < count; n++ {
			m.Inject(context.Background(), map[string][]*Packet{
				"map_id": testPayload,
			})
		}
	}()

	<-time.After(time.Second)
	cancel()
	<-time.After(3 * time.Second)
}

func Test_Link(t *testing.T) {
	count := 10000
	out := make(chan []Data)
	m := NewStream("machine_id", func(c context.Context) chan []Data {
		channel := make(chan []Data)
		go func() {
			for n := 0; n < count; n++ {
				out := []Data{}
				buf := &bytes.Buffer{}
				enc, dec := gob.NewEncoder(buf), gob.NewDecoder(buf)

				_ = enc.Encode(testList)
				_ = dec.Decode(&out)
				channel <- out
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
		Map("map_id", func(m Data) error {
			if _, ok := m["name"]; !ok {
				t.Errorf("packet missing name %v", m)
				return fmt.Errorf("incorrect data have %v want %v", m, "name field")
			}
			return nil
		}).
		Fork("fork_id", ForkRule(func(d Data) bool {
			if val := typed.Typed(d).IntOr("loops", 0); val > 5 {
				return true
			} else {
				d["loops"] = val + 1
			}

			return false
		}).Handler,
			&Option{Injectable: boolP(false)},
			&Option{BufferSize: intP(10)},
		)

	left.Transmit("sender_id", func(d []Data) error {
		out <- d
		return nil
	})

	right.Link("link_id", "map_id")

	ctx, cancel := context.WithCancel(context.Background())

	if err := m.Run(ctx); err != nil {
		t.Error(err)
	}

	for n := 0; n < count; n++ {
		list := <-out

		if len(list) != 10 || list[0]["loops"] != 6 {
			t.Errorf("incorrect data have %v want %v", list, testList)
		}
	}

	go func() {
		for n := 0; n < count; n++ {
			out := []*Packet{}
			buf := &bytes.Buffer{}
			enc, dec := gob.NewEncoder(buf), gob.NewDecoder(buf)

			_ = enc.Encode(testPayload)
			_ = dec.Decode(&out)
			m.Inject(context.Background(), map[string][]*Packet{
				"map_id":  out,
				"fork_id": out,
			})
		}
	}()

	<-time.After(time.Second)
	cancel()
	<-time.After(time.Second)
}

func Test_Link_not_ancestor(t *testing.T) {
	count := 100000
	out := make(chan []Data)
	m := NewStream("machine_id", func(c context.Context) chan []Data {
		channel := make(chan []Data)
		go func() {
			for n := 0; n < count; n++ {
				channel <- testList
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
		Map("map_id", func(m Data) error {
			if _, ok := m["name"]; !ok {
				t.Errorf("packet missing name %v", m)
				return fmt.Errorf("incorrect data have %v want %v", m, "name field")
			}
			return nil
		}).Fork("fork_id", ForkRule(func(d Data) bool {
		if val := typed.Typed(d).IntOr("loops", 0); val > 5 {
			return false
		} else {
			d["loops"] = val + 1
		}

		return true
	}).Handler)

	left.Link("link_id", "sender_id",
		&Option{FIFO: boolP(false)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(true)},
		&Option{BufferSize: intP(0)},
	)

	right.Transmit("sender_id", func(d []Data) error {
		out <- d
		return nil
	})

	if err := m.Run(context.Background()); err == nil {
		t.Error("expecting error")
	}
}
