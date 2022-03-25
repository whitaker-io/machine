// Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package machine

import (
	"context"
	"encoding/gob"
	"strings"
	"testing"
)

type idMap map[string]any

func (i idMap) ID() string {
	v := i["name"]

	return v.(string)
}

var testListInvalidBase = []idMap{
	{
		"name":  "data0",
		"value": 0,
	},
}

var testPayloadBase = []idMap{
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

type tester struct {
	close error
}

func (t *tester) Read(ctx context.Context) []idMap {
	return deepcopy(testPayloadBase)
}

func (t *tester) Close() error {
	return t.close
}

func (t *tester) Error(...interface{}) {}
func (t *tester) Info(...interface{})  {}

type publishFN func([]idMap) error

func (p publishFN) Send(payload []idMap) error {
	return p(payload)
}

func (p publishFN) HandleError(payload []idMap, err error) {}

func Benchmark_Test_New(b *testing.B) {
	channel := make(chan []idMap)
	m := New("machine_id",
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(false),
			BufferSize: intP(0),
		},
	)

	out := m.Builder().
		Map(
			func(m idMap) idMap {
				if m.ID() == "" {
					b.Errorf("packet missing name %v", m)
				}
				return m
			},
		).Channel()

	if err := m.Consume(context.Background(), channel); err != nil {
		b.Error(err)
		b.FailNow()
	}

	for n := 0; n < b.N; n++ {
		go func() {
			channel <- testPayloadBase
		}()

		list := <-out

		if len(list) != len(testPayloadBase) {
			b.Errorf("incorrect data have %v want %v", list, testPayloadBase)
			b.FailNow()
		}
	}
}

func Test_New(b *testing.T) {
	count := 10
	channel := make(chan []idMap)
	go func() {
		channel <- deepcopy(testListInvalidBase)
		for n := 0; n < count; n++ {
			channel <- deepcopy(testPayloadBase)
		}
	}()

	m := New("machine_id",
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(false),
			BufferSize: intP(0),
		},
	)

	left, right := m.Builder().
		Map(
			func(m idMap) idMap {
				if _, ok := m["name"]; !ok {
					b.Errorf("packet missing name %v", m)
				}
				return m
			},
		).
		Window(
			func(payload []idMap) []idMap {
				return payload
			},
		).
		Sort(
			func(a, b idMap) int {
				return strings.Compare(a["name"].(string), b["name"].(string))
			},
		).
		Remove(
			func(index int, d idMap) bool {
				return false
			},
		).
		FoldLeft(
			func(d1, d2 idMap) idMap {
				return d1
			},
		).
		FoldLeft(
			func(d1, d2 idMap) idMap {
				return d1
			},
		).
		FoldRight(
			func(d1, d2 idMap) idMap {
				return d1
			},
		).
		Filter(
			func(d idMap) bool { return true },
		)

	out := left.Channel()
	right.Channel()

	if err := m.Consume(context.Background(), channel); err != nil {
		b.Error(err)
		b.FailNow()
	}

	for n := 0; n < count; n++ {
		list := <-out

		if len(list) != 1 {
			b.Errorf("incorrect data have %v want %v", list, testPayloadBase[0])
			b.FailNow()
		}
	}
}

func Test_New2(b *testing.T) {
	count := 10
	channel := make(chan []idMap)
	go func() {
		for n := 0; n < count; n++ {
			channel <- deepcopy(testPayloadBase)
		}
	}()
	m := New("machine_id",
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(true),
			BufferSize: intP(1000),
			Telemetry: &Telemetry{
				Enabled:     boolP(true),
				TracerName:  stringP("test"),
				LabelPrefix: stringP("test"),
			},
		},
	)

	left, right := m.Builder().
		Map(
			func(m idMap) idMap {
				if _, ok := m["name"]; !ok {
					b.Errorf("packet missing name %v", m)
					b.FailNow()
				}
				return m
			},
		).
		FoldRight(
			func(d1, d2 idMap) idMap {
				return d1
			},
		).
		Duplicate()

	out := left.Channel()

	l2, r2 := right.Filter(
		func(d idMap) bool {
			return true
		},
	)

	r2.Channel()

	l3, r3 := l2.Filter(
		func(d idMap) bool {
			return false
		},
	)

	l3.Channel()

	out2 := r3.Channel()

	if err := m.Consume(context.Background(), channel); err != nil {
		b.Error(err)
		b.FailNow()
	}

	if m.ID() != "machine_id" {
		b.Errorf("incorrect id have %v want %v", m.ID(), "machine_id")
		b.FailNow()
	}

	for n := 0; n < 2*count; n++ {
		select {
		case list := <-out:
			if len(list) != 1 {
				b.Errorf("incorrect data have %v want %v", list, testPayloadBase[9])
				b.FailNow()
			}
		case list := <-out2:
			if len(list) != 1 {
				b.Errorf("incorrect data have %v want %v", list, testPayloadBase[9])
				b.FailNow()
			}
		}
	}
}

func Test_Panic(b *testing.T) {
	count := 10
	channel := make(chan []idMap)
	go func() {
		for n := 0; n < count; n++ {
			channel <- deepcopy(testPayloadBase)
		}
	}()
	m := New("machine_id",
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(true),
			BufferSize: intP(1000),
		},
	)

	m.Builder().
		Map(
			func(m idMap) idMap {
				if _, ok := m["name"]; !ok {
					b.Errorf("packet missing name %v", m)
				}
				return m
			},
		).
		FoldRight(
			func(d1, d2 idMap) idMap {
				panic("panic")
			},
		).Channel()

	if err := m.Consume(context.Background(), channel); err != nil {
		b.Error(err)
		b.FailNow()
	}
}

func Test_Missing_Leaves(b *testing.T) {
	m := New("machine_id", &Option[idMap]{})

	m.Builder().Duplicate()

	m2 := New("machine_id",
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(true),
			BufferSize: intP(1000),
		},
	)

	m2.Builder().Duplicate()

	m3 := New("machine_id",
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(true),
			BufferSize: intP(1000),
		},
	)

	m4 := New("machine_id",
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(true),
			BufferSize: intP(1000),
		},
	)

	m4.Builder().
		Map(
			func(m idMap) idMap {
				if _, ok := m["name"]; !ok {
					b.Errorf("packet missing name %v", m)
				}
				return m
			},
		)

	m5 := New("machine_id",
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(true),
			BufferSize: intP(1000),
		},
	)

	m5.Builder().
		FoldRight(
			func(d1, d2 idMap) idMap {
				return d1
			},
		)

	m6 := New("machine_id",
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(true),
			BufferSize: intP(1000),
		},
	)

	l6, r6 := m6.Builder().Duplicate()

	l6.Map(
		func(m idMap) idMap {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
			}
			return m
		},
	)

	r6.Map(
		func(m idMap) idMap {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
			}
			return m
		},
	)

	m7 := New("machine_id",
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(true),
			BufferSize: intP(1000),
		},
	)

	l7, r7 := m7.Builder().Duplicate()

	l7.Map(
		func(m idMap) idMap {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
			}
			return m
		},
	)

	r7.FoldLeft(
		func(d1, d2 idMap) idMap {
			return d1
		},
	)

	m8 := New("machine_id",
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(true),
			BufferSize: intP(1000),
		},
	)

	m8.Builder().
		Loop(
			func(a idMap) bool {
				return false
			},
		)

	m9 := New("machine_id",
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(true),
			BufferSize: intP(1000),
		},
	)

	m9.Builder().
		Sort(
			func(a, b idMap) int {
				return 0
			},
		)

	m10 := New("machine_id",
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(true),
			BufferSize: intP(0),
		},
	)

	m10.Builder().Remove(
		func(index int, d idMap) bool {
			return true
		},
	)

	if err := m.Consume(context.Background(), make(chan []idMap)); err == nil {
		b.Error("expected error m")
		b.FailNow()
	}

	if err := m2.Consume(context.Background(), make(chan []idMap)); err == nil {
		b.Error("expected error m2")
		b.FailNow()
	}

	if err := m3.Consume(context.Background(), make(chan []idMap)); err == nil {
		b.Error("expected error m3")
		b.FailNow()
	}

	if err := m4.Consume(context.Background(), make(chan []idMap)); err == nil {
		b.Error("expected error m4")
		b.FailNow()
	}

	if err := m5.Consume(context.Background(), make(chan []idMap)); err == nil {
		b.Error("expected error m5")
		b.FailNow()
	}

	if err := m6.Consume(context.Background(), make(chan []idMap)); err == nil {
		b.Error("expected error m6")
		b.FailNow()
	}

	if err := m7.Consume(context.Background(), make(chan []idMap)); err == nil {
		b.Error("expected error m7")
		b.FailNow()
	}

	if err := m8.Consume(context.Background(), make(chan []idMap)); err == nil {
		b.Error("expected error m8")
		b.FailNow()
	}

	if err := m9.Consume(context.Background(), make(chan []idMap)); err == nil {
		b.Error("expected error m9")
		b.FailNow()
	}

	if err := m10.Consume(context.Background(), make(chan []idMap)); err == nil {
		b.Error("expected error m10")
		b.FailNow()
	}
}

func Test_Loop(b *testing.T) {
	count := 10
	channel := make(chan []idMap)
	go func() {
		for n := 0; n < count; n++ {
			channel <- deepcopy(testPayloadBase)
		}
	}()
	m := New("machine_id",
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(false),
			BufferSize: intP(0),
		},
	)

	counter := 1
	left, right := m.Builder().
		Map(
			func(m idMap) idMap {
				if _, ok := m["name"]; !ok {
					b.Errorf("packet missing name %v", m)
					b.FailNow()
				}
				return m
			},
		).
		Loop(
			func(a idMap) bool {
				counter++
				return counter%2 == 0
			},
		)

	counter2 := 1
	inside, _ := left.Loop(
		func(a idMap) bool {
			counter2++
			return counter2%2 == 0
		},
	)

	inside.Map(
		func(m idMap) idMap {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
				b.FailNow()
			}
			return m
		},
	)

	out := right.Channel()

	if err := m.Consume(context.Background(), channel); err != nil {
		b.Error(err)
		b.FailNow()
	}

	for n := 0; n < count; n++ {
		list := <-out

		if len(list) == 10 {
			b.Errorf("incorrect data have %v want %v", list, testPayloadBase[0])
		}
	}
}

func init() {
	gob.Register(idMap{})
	gob.Register([]idMap{})
}
