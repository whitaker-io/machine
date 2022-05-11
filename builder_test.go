// Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package machine

import (
	"context"
	"strings"
	"testing"
)

type kv struct {
	name  string
	value int
}

func (i *kv) ID() string {
	return i.name
}

var testListInvalidBase = []*kv{
	{
		name:  "data0",
		value: 0,
	},
}

var testPayloadBase = []*kv{
	{
		name:  "data0",
		value: 0,
	},
	{
		name:  "data1",
		value: 1,
	},
	{
		name:  "data2",
		value: 2,
	},
	{
		name:  "data3",
		value: 3,
	},
	{
		name:  "data4",
		value: 4,
	},
	{
		name:  "data5",
		value: 5,
	},
	{
		name:  "data6",
		value: 6,
	},
	{
		name:  "data7",
		value: 7,
	},
	{
		name:  "data8",
		value: 8,
	},
	{
		name:  "data9",
		value: 9,
	},
}

func deepcopy(list ...*kv) []*kv {
	out := make([]*kv, len(list))

	for n, i := range list {
		out[n] = &kv{name: i.name, value: i.value}
	}

	return out
}

func deepcopyKV(k *kv) *kv { return &kv{name: k.name, value: k.value} }

func Benchmark_Test_New(b *testing.B) {
	out := make(chan []*kv)
	channel := make(chan []*kv)
	m := New[*kv]("machine_id",
		&edgeProvider[*kv]{},
		&Option[*kv]{
			DeepCopy:   deepcopyKV,
			FIFO:       false,
			BufferSize: 0,
		},
	)

	m.Builder().
		Map(
			func(m *kv) *kv {
				if m.ID() == "" {
					b.Errorf("packet missing name %v", m)
				}
				return m
			},
		).OutputTo(out)

	if err := m.StartWith(context.Background(), channel); err != nil {
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
	count := 1000
	out := make(chan []*kv)
	out2 := make(chan []*kv)
	channel := make(chan []*kv)
	go func() {
		channel <- deepcopy(testListInvalidBase...)
		for n := 0; n < count; n++ {
			channel <- deepcopy(testPayloadBase...)
		}
	}()

	m := NewWithChannels("machine_id",
		&Option[*kv]{
			DeepCopy:   deepcopyKV,
			FIFO:       false,
			BufferSize: 0,
		},
	)

	left, right := m.Builder().
		Map(
			func(m *kv) *kv {
				return m
			},
		).
		Window(
			func(payload []*kv) []*kv {
				return payload
			},
		).
		Sort(
			func(a, b *kv) int {
				return strings.Compare(a.name, b.name)
			},
		).
		Remove(
			func(index int, d *kv) bool {
				return false
			},
		).
		Combine(
			func(d []*kv) *kv {
				return d[0]
			},
		).
		Filter(
			func(d *kv) FilterResult {
				return FilterLeft
			},
		)

	left.OutputTo(out)
	right.OutputTo(out2)

	if err := m.StartWith(context.Background(), channel); err != nil {
		b.Error(err)
		b.FailNow()
	}

	for n := 0; n < count; n++ {
		select {
		case list := <-out:
			if len(list) != 1 {
				b.Errorf("incorrect data have %v want %v", list, testPayloadBase[0])
				b.FailNow()
			}
		case <-out2:
			b.Errorf("should never reach this")
			b.FailNow()
		}
	}
}

func Test_New2(b *testing.T) {
	count := 100000
	out := make(chan []*kv)
	out2 := make(chan []*kv)
	channel := make(chan []*kv)
	go func() {
		for n := 0; n < count; n++ {
			channel <- deepcopy(testPayloadBase...)
		}
	}()
	m := New[*kv]("machine_id",
		&edgeProvider[*kv]{},
		&Option[*kv]{
			DeepCopy:   deepcopyKV,
			FIFO:       true,
			BufferSize: 1000,
		},
	)

	left, right := m.Builder().
		Map(
			func(m *kv) *kv {
				return m
			},
		).
		Combine(
			func(d []*kv) *kv {
				return d[0]
			},
		).Filter(Duplicate[*kv])

	left.OutputTo(out)

	l2, r2 := right.Filter(
		func(d *kv) FilterResult {
			return FilterLeft
		},
	)

	r2.OutputTo(out2)

	l3, r3 := l2.Filter(
		func(d *kv) FilterResult {
			return FilterRight
		},
	)

	l3.OutputTo(out2)
	r3.OutputTo(out)

	if err := m.StartWith(context.Background(), channel); err != nil {
		b.Error(err)
		b.FailNow()
	}

	for n := 0; n < 2*count; n++ {
		select {
		case list := <-out:
			if len(list) != 1 {
				b.Errorf("incorrect data have %v want %v", list, testPayloadBase[0])
				b.FailNow()
			}
		case <-out2:
			b.Errorf("should never reach this")
			b.FailNow()
		}
	}
}

func Test_Panic(b *testing.T) {
	out := make(chan []*kv)
	count := 100000
	channel := make(chan []*kv)
	go func() {
		for n := 0; n < count; n++ {
			channel <- deepcopy(testPayloadBase...)
		}
	}()
	m := NewWithChannels("machine_id",
		&Option[*kv]{
			DeepCopy:   deepcopyKV,
			FIFO:       true,
			BufferSize: 1000,
		},
	)

	m.Builder().
		Map(
			func(m *kv) *kv {
				return m
			},
		).
		Combine(
			func(d []*kv) *kv {
				panic("not supposed to be here")
				return d[0]
			},
		).OutputTo(out)

	if err := m.StartWith(context.Background(), channel); err != nil {
		b.Error(err)
		b.FailNow()
	}
}

func Test_Missing_Leaves(b *testing.T) {
	m := NewWithChannels("machine_id", &Option[*kv]{})

	m.Builder().
		Map(
			func(m *kv) *kv {
				return m
			},
		)

	m2 := NewWithChannels("machine_id", &Option[*kv]{})

	m.Builder().
		Filter(
			func(d *kv) FilterResult {
				return FilterLeft
			},
		)

	if err := m.StartWith(context.Background(), make(chan []*kv)); err == nil {
		b.Error("expected error m")
		b.FailNow()
	}

	if err := m2.StartWith(context.Background(), make(chan []*kv)); err == nil {
		b.Error("expected error m2")
		b.FailNow()
	}
}

func Test_Loop(b *testing.T) {
	out := make(chan []*kv)
	count := 100000
	channel := make(chan []*kv)
	go func() {
		for n := 0; n < count; n++ {
			channel <- deepcopy(testPayloadBase...)
		}
	}()
	m := NewWithChannels("machine_id",
		&Option[*kv]{
			DeepCopy:   deepcopyKV,
			FIFO:       false,
			BufferSize: 0,
		},
	)

	counter := 1
	left, right := m.Builder().
		Map(
			func(m *kv) *kv {
				return m
			},
		).
		Loop(
			func(a *kv) FilterResult {
				counter++
				if counter%2 == 0 {
					return FilterLeft
				}
				return FilterRight
			},
		)

	counter2 := 1
	inside, _ := left.Loop(
		func(a *kv) FilterResult {
			counter2++
			if counter%2 == 0 {
				return FilterLeft
			}
			return FilterRight
		},
	)

	inside.Map(
		func(m *kv) *kv {
			return m
		},
	)

	right.OutputTo(out)

	m.Consume(channel)
	m.Clear()
	m.Consume(channel)

	if err := m.Start(context.Background()); err != nil {
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
