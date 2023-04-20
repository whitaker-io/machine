// Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package machine

import (
	"context"
	"fmt"
	"testing"
	"time"
)

type kv struct {
	name  string
	value int
}

func (i *kv) ID() string {
	return i.name
}

var testPayloadBase = &kv{
	name:  "data0",
	value: 5,
}

func deepcopy(item *kv) *kv {
	out := &kv{
		name:  item.name,
		value: item.value,
	}

	return out
}

func deepcopyKV(k *kv) *kv { return &kv{name: k.name, value: k.value} }

type channelEdge[T any] struct {
	channel chan T
}

func (t *channelEdge[T]) ReceiveOn(ctx context.Context, channel chan T) {
	t.channel = channel
}
func (t *channelEdge[T]) Send(payload T) {
	t.channel <- payload
}

type noopTelemetry[T any] struct{}

func (t *noopTelemetry[T]) IncrementPayloadCount(string)   {}
func (t *noopTelemetry[T]) IncrementErrorCount(string)     {}
func (t *noopTelemetry[T]) Duration(string, time.Duration) {}
func (t *noopTelemetry[T]) RecordPayload(string, T)        {}
func (t *noopTelemetry[T]) RecordError(string, T, error)   {}

func Benchmark_Test_New(b *testing.B) {
	out := make(chan *kv)
	channel := make(chan *kv)
	m := New("machine_id",
		&Option[*kv]{
			// DeepCopy:   deepcopyKV,
			FIFO:       false,
			BufferSize: 0,
		},
	)

	m.Builder().
		Then(
			func(m *kv) *kv {
				if m.ID() == "" {
					b.Errorf("packet missing name %v", m)
				}
				return m
			},
		).OutputTo(out)

	if err := m.Start(context.Background(), channel); err != nil {
		b.Error(err)
		b.FailNow()
	}

	for n := 0; n < b.N; n++ {
		go func() {
			channel <- testPayloadBase
		}()

		<-out
	}
}

func Test_New(b *testing.T) {
	count := 10000
	out := make(chan *kv)
	out2 := make(chan *kv)
	channel := make(chan *kv)
	go func() {
		for n := 0; n < count; n++ {
			channel <- &kv{
				name:  fmt.Sprintf("name%d", n),
				value: 5,
			}
		}
	}()

	m := New("machine_id",
		&Option[*kv]{
			DeepCopy:   deepcopyKV,
			Telemetry:  &noopTelemetry[*kv]{},
			FIFO:       false,
			BufferSize: 0,
		},
	)

	list := m.Builder().
		Then(
			func(m *kv) *kv {
				return m
			},
		).
		Y(func(f Applicative[*kv]) Applicative[*kv] {
			return func(x *kv) *kv {
				if x.value < 1 {
					return &kv{x.name, 1}
				} else {
					return &kv{x.name, x.value * f(&kv{x.name, x.value - 1}).value}
				}
			}
		}).
		When(
			func(d *kv) bool {
				return false
			},
			func(d *kv) bool {
				return false
			},
			func(d *kv) bool {
				return true
			},
		)

		list[0].Drop()
		list[1].Drop()
		list[3].Drop()


		left, right := list[2].Filter(
			func(d *kv) bool {
				return true
			},
		)

	l2, r2 := left.Or(func(d *kv) (*kv, error) {
		return d, nil
	}, func(d *kv) (*kv, error) {
		return d, nil
	})

	l3, r3 := l2.Or(func(d *kv) (*kv, error) {
		return d, fmt.Errorf("error")
	}, func(d *kv) (*kv, error) {
		return d, fmt.Errorf("error")
	})

	l4, r4 := r3.And(func(d *kv) (*kv, error) {
		return d, nil
	}, func(d *kv) (*kv, error) {
		return d, nil
	})

	l5, r5 := l4.And(func(d *kv) (*kv, error) {
		return d, nil
	}, func(d *kv) (*kv, error) {
		return d, fmt.Errorf("error")
	}, func(d *kv) (*kv, error) {
		return d, nil
	})

	r5.OutputTo(out)

	right.OutputTo(out2)
	r2.OutputTo(out2)
	l3.OutputTo(out2)
	r4.OutputTo(out2)
	l5.OutputTo(out2)

	ctx, cancel := context.WithCancel(context.Background())

	if err := m.Start(ctx, channel); err != nil {
		b.Error(err)
		b.FailNow()
	}

	for n := 0; n < count; n++ {
		select {
		case x := <-out:
			if x.value != 120 {
				b.Errorf("unexpected value %v", x.value)
			}
		case <-out2:
			b.Errorf("should never reach this")
			b.FailNow()
		}
	}

	cancel()

	<-time.After(10 * time.Millisecond)
}

func Test_New2(b *testing.T) {
	count := 100000
	out := make(chan *kv)
	out2 := make(chan *kv)
	channel := make(chan *kv)
	go func() {
		for n := 0; n < count; n++ {
			channel <- deepcopy(testPayloadBase)
		}
	}()
	m := New("machine_id",
		&Option[*kv]{
			FIFO:                     true,
			BufferSize:               1000,
			DeepCopy:                 deepcopyKV,
			DeepCopyBetweenVerticies: true,
		},
	)

	left, right := m.Builder().
		Then(
			func(m *kv) *kv {
				return m
			},
		).
		Distribute(&channelEdge[*kv]{}).
		Duplicate()

	left.OutputTo(out)

	l2, r2 := right.Filter(
		func(d *kv) bool {
			return true
		},
	)

	r2.OutputTo(out2)

	l3, r3 := l2.Filter(
		func(d *kv) bool {
			return false
		},
	)

	l3.OutputTo(out2)
	l4, r4 := r3.Duplicate()

	l4.OutputTo(out)
	r4.Drop()

	ctx, cancel := context.WithCancel(context.Background())

	if err := m.Start(ctx, channel); err != nil {
		b.Error(err)
		b.FailNow()
	}

	for n := 0; n < 2*count; n++ {
		select {
		case <-out:
		case <-out2:
			b.Errorf("should never reach this")
			b.FailNow()
		}
	}

	cancel()

	<-time.After(10 * time.Millisecond)
}

func Test_Panic(b *testing.T) {
	out := make(chan *kv)
	count := 100000
	channel := make(chan *kv)
	go func() {
		for n := 0; n < count; n++ {
			channel <- deepcopy(testPayloadBase)
		}
	}()
	m := New("machine_id",
		&Option[*kv]{
			DeepCopy:     deepcopyKV,
			Telemetry:    &noopTelemetry[*kv]{},
			FIFO:         true,
			BufferSize:   1000,
			PanicHandler: func(err error, payload *kv) {},
		},
	)

	m.Builder().
		Then(
			func(m *kv) *kv {
				panic(fmt.Errorf("error"))
			},
		).OutputTo(out)

	if err := m.Start(context.Background(), channel); err != nil {
		b.Error(err)
		b.FailNow()
	}

	<-time.After(10 * time.Millisecond)
}

func Test_Missing_Leaves(b *testing.T) {
	m := New("machine_id", &Option[*kv]{})

	m.Builder().
		Then(
			func(m *kv) *kv {
				return m
			},
		)

	m2 := New("machine_id", &Option[*kv]{})

	m2.Builder().
		Filter(
			func(d *kv) bool {
				return true
			},
		)

	m3 := New("machine_id", &Option[*kv]{})

	m3.Builder().
		Then(
			func(m *kv) *kv {
				return m
			},
		).
		Then(
			func(m *kv) *kv {
				return m
			},
		)

	m4 := New("machine_id",
		&Option[*kv]{
			FIFO:       false,
			BufferSize: 0,
		},
	)

	counter := 1
	left, right := m4.Builder().
		Then(
			func(m *kv) *kv {
				return m
			},
		).
		Loop(
			func(a *kv) bool {
				counter++
				return counter%2 == 0
			},
		)

	left.
		Filter(
			func(d *kv) bool {
				return true
			},
		)

	right.
		Filter(
			func(d *kv) bool {
				return true
			},
		)

	m5 := New("machine_id", &Option[*kv]{})

	left, _ = m5.Builder().
		Filter(
			func(d *kv) bool {
				return true
			},
		)

	left.
		Then(
			func(m *kv) *kv {
				return m
			},
		)

	m6 := New("machine_id", &Option[*kv]{})

	m7 := New("machine_id", &Option[*kv]{})

	m7.Builder().
		Distribute(&channelEdge[*kv]{make(chan *kv)})

	if err := m.Start(context.Background(), make(chan *kv)); err == nil {
		b.Error("expected error m")
		b.FailNow()
	}

	if err := m2.Start(context.Background(), make(chan *kv)); err == nil {
		b.Error("expected error m2")
		b.FailNow()
	}

	if err := m3.Start(context.Background(), make(chan *kv)); err == nil {
		b.Error("expected error m3")
		b.FailNow()
	}

	if err := m4.Start(context.Background(), make(chan *kv)); err == nil {
		b.Error("expected error m4")
		b.FailNow()
	}

	if err := m5.Start(context.Background(), make(chan *kv)); err == nil {
		b.Error("expected error m4")
		b.FailNow()
	}

	if err := m6.Start(context.Background(), make(chan *kv)); err == nil {
		b.Error("expected error m4")
		b.FailNow()
	}

	if err := m7.Start(context.Background(), make(chan *kv)); err == nil {
		b.Error("expected error m4")
		b.FailNow()
	}
}

func Test_Loop(b *testing.T) {
	out := make(chan *kv)
	count := 10000
	channel := make(chan *kv)
	go func() {
		for n := 0; n < count; n++ {
			channel <- deepcopy(testPayloadBase)
		}
	}()
	m := New("machine_id",
		&Option[*kv]{
			FIFO:       false,
			BufferSize: 0,
		},
	)

	counter := 1
	left, right := m.Builder().
		Then(
			func(m *kv) *kv {
				return m
			},
		).
		Loop(
			func(a *kv) bool {
				counter++
				return counter%2 == 0
			},
		)

	counter2 := 1
	inside, _ := left.Loop(
		func(a *kv) bool {
			counter2++
			return counter2%2 == 0
		},
	)

	inside.Then(
		func(m *kv) *kv {
			return m
		},
	)

	right.OutputTo(out)

	if err := m.Start(context.Background(), channel); err != nil {
		b.Error(err)
		b.FailNow()
	}

	for n := 0; n < count; n++ {
		<-out
	}
}
