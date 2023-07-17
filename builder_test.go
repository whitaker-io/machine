// Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package machine

import (
	"context"
	"fmt"
	"strconv"
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

type channelEdge[T any] chan T

func (t channelEdge[T]) Output() chan T {
	return t
}
func (t channelEdge[T]) Send(payload T) {
	t <- payload
}

type noopTelemetry[T any] struct{}

func (t *noopTelemetry[T]) IncrementPayloadCount(string)   {}
func (t *noopTelemetry[T]) IncrementErrorCount(string)     {}
func (t *noopTelemetry[T]) Duration(string, time.Duration) {}
func (t *noopTelemetry[T]) RecordPayload(string, T)        {}
func (t *noopTelemetry[T]) RecordError(string, T, error)   {}

func Benchmark_Test_New(b *testing.B) {
	channel := make(chan *kv)
	startFn, m := New("machine_id",
		channel,
		&Option[*kv]{
			// DeepCopy:   deepcopyKV,
			FIFO:       false,
			BufferSize: 0,
		},
	)

	out := m.
		Then(
			func(m *kv) *kv {
				if m.ID() == "" {
					b.Errorf("packet missing name %v", m)
				}
				return m
			},
		).Output()

	startFn(context.Background())

	for n := 0; n < b.N; n++ {
		go func() {
			channel <- testPayloadBase
		}()

		<-out
	}
}

func Test_New(b *testing.T) {
	count := 10000
	channel := make(chan *kv)
	go func() {
		for n := 0; n < count; n++ {
			channel <- &kv{
				name:  fmt.Sprintf("name%d", n),
				value: 11,
			}
		}
	}()

	startFn, m := New("machine_id",
		channel,
		&Option[*kv]{
			FIFO:         false,
			BufferSize:   0,
			DeepCopy:     deepcopyKV,
			PanicHandler: func(err error, payload *kv) {},
		},
	)

	m.Name()

	list := m.
		Then(
			func(m *kv) *kv {
				return m
			},
		).
		Recurse(func(f Monad[*kv]) Monad[*kv] {
			return func(x *kv) *kv {
				if x.value < 3 {
					return &kv{x.name, 1}
				} else {
					return &kv{x.name, f(&kv{x.name, x.value - 1}).value + f(&kv{x.name, x.value - 2}).value}
				}
			}
		}).
		Memoize(
			func(f Monad[*kv]) Monad[*kv] {
				return func(x *kv) *kv {
					if x.value < 3 {
						return &kv{x.name, 1}
					} else {
						return &kv{x.name, f(&kv{x.name, x.value - 1}).value + f(&kv{x.name, x.value - 2}).value}
					}
				}
			},
			func(k *kv) string {
				return strconv.Itoa(k.value)
			},
		).
		Select(
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

	left, right := list[2].If(
		func(d *kv) bool {
			return true
		},
	)

	l2, r2 := left.Or(func(d *kv) bool {
		return true
	}, func(d *kv) bool {
		return true
	})

	l3, r3 := l2.Or(func(d *kv) bool {
		return false
	}, func(d *kv) bool {
		return false
	})

	l4, r4 := r3.And(func(d *kv) bool {
		return true
	}, func(d *kv) bool {
		return true
	})

	l5, r5 := l4.And(func(d *kv) bool {
		return true
	}, func(d *kv) bool {
		return false
	}, func(d *kv) bool {
		return true
	})

	outGood1 := r5.Output()

	outBad1 := right.Output()
	outBad2 := r2.Output()
	outBad3 := l3.Output()
	outBad4 := r4.Output()
	outBad5 := l5.Output()

	ctx, cancel := context.WithCancel(context.Background())

	startFn(ctx)

	right.AsEdge().Send(
		&kv{
			name:  fmt.Sprintf("name%d", 10001),
			value: 11,
		},
	)

	for n := 0; n < count+1; n++ {
		select {
		case x := <-outGood1:
			if x.value != 1779979416004714189 {
				b.Errorf("unexpected value %v", x.value)
			}
		case <-outBad1:
			b.Errorf("should never reach this")
			b.FailNow()
		case <-outBad2:
			b.Errorf("should never reach this")
			b.FailNow()
		case <-outBad3:
			b.Errorf("should never reach this")
			b.FailNow()
		case <-outBad4:
			b.Errorf("should never reach this")
			b.FailNow()
		case <-outBad5:
			b.Errorf("should never reach this")
			b.FailNow()
		}
	}

	cancel()

	<-time.After(10 * time.Millisecond)
}

func Test_New2(b *testing.T) {
	count := 100000
	channel := make(chan *kv)
	go func() {
		for n := 0; n < count; n++ {
			channel <- deepcopy(testPayloadBase)
		}
	}()
	startFn, m := New("machine_id",
		channel,
		&Option[*kv]{
			FIFO:                     true,
			BufferSize:               1000,
			DeepCopy:                 deepcopyKV,
			DeepCopyBetweenVerticies: true,
		},
	)

	left, right := m.
		Then(
			func(m *kv) *kv {
				return m
			},
		).
		Distribute(channelEdge[*kv](make(chan *kv))).
		Duplicate()

	outGood1 := left.Output()

	l2, r2 := right.If(
		func(d *kv) bool {
			return true
		},
	)

	outBad1 := r2.Output()

	l3, r3 := l2.If(
		func(d *kv) bool {
			return false
		},
	)

	outBad2 := l3.Output()
	l4, r4 := r3.Duplicate()

	outGood2 := l4.Output()
	r4.Drop()

	ctx, cancel := context.WithCancel(context.Background())

	startFn(ctx)

	for n := 0; n < 2*count; n++ {
		select {
		case <-outGood1:
		case <-outGood2:
		case <-outBad1:
			b.Errorf("should never reach this")
			b.FailNow()
		case <-outBad2:
			b.Errorf("should never reach this")
			b.FailNow()
		}
	}

	cancel()

	<-time.After(10 * time.Millisecond)
}

func Test_Panic(b *testing.T) {
	count := 100000
	channel := make(chan *kv)
	go func() {
		for n := 0; n < count; n++ {
			channel <- deepcopy(testPayloadBase)
		}
	}()
	startFn, m := New("machine_id",
		channel,
		&Option[*kv]{
			DeepCopy:   deepcopyKV,
			Telemetry:  &noopTelemetry[*kv]{},
			FIFO:       true,
			BufferSize: 1000,
		},
	)

	m.Then(
		func(m *kv) *kv {
			panic(fmt.Errorf("error"))
		},
	).Output()

	startFn(context.Background())

	<-time.After(300 * time.Millisecond)
}

// func Test_Missing_Leaves(b *testing.T) {
// 	m := New("machine_id", &Option[*kv]{})

// 	m.Builder().
// 		Then(
// 			func(m *kv) *kv {
// 				return m
// 			},
// 		)

// 	m2 := New("machine_id", &Option[*kv]{})

// 	m2.Builder().
// 		Filter(
// 			func(d *kv) bool {
// 				return true
// 			},
// 		)

// 	m3 := New("machine_id", &Option[*kv]{})

// 	m3.Builder().
// 		Then(
// 			func(m *kv) *kv {
// 				return m
// 			},
// 		).
// 		Then(
// 			func(m *kv) *kv {
// 				return m
// 			},
// 		)

// 	m4 := New("machine_id",
// 		&Option[*kv]{
// 			FIFO:       false,
// 			BufferSize: 0,
// 		},
// 	)

// 	counter := 1
// 	left, right := m4.Builder().
// 		Then(
// 			func(m *kv) *kv {
// 				return m
// 			},
// 		).
// 		Loop(
// 			func(a *kv) bool {
// 				counter++
// 				return counter%2 == 0
// 			},
// 		)

// 	left.
// 		Filter(
// 			func(d *kv) bool {
// 				return true
// 			},
// 		)

// 	right.
// 		Filter(
// 			func(d *kv) bool {
// 				return true
// 			},
// 		)

// 	m5 := New("machine_id", &Option[*kv]{})

// 	left, _ = m5.Builder().
// 		Filter(
// 			func(d *kv) bool {
// 				return true
// 			},
// 		)

// 	left.
// 		Then(
// 			func(m *kv) *kv {
// 				return m
// 			},
// 		)

// 	m6 := New("machine_id", &Option[*kv]{})

// 	m7 := New("machine_id", &Option[*kv]{})

// 	m7.Builder().
// 		Distribute(&channelEdge[*kv]{make(chan *kv)})

// 	if err := m.Start(context.Background(), make(chan *kv)); err == nil {
// 		b.Error("expected error m")
// 		b.FailNow()
// 	}

// 	if err := m2.Start(context.Background(), make(chan *kv)); err == nil {
// 		b.Error("expected error m2")
// 		b.FailNow()
// 	}

// 	if err := m3.Start(context.Background(), make(chan *kv)); err == nil {
// 		b.Error("expected error m3")
// 		b.FailNow()
// 	}

// 	if err := m4.Start(context.Background(), make(chan *kv)); err == nil {
// 		b.Error("expected error m4")
// 		b.FailNow()
// 	}

// 	if err := m5.Start(context.Background(), make(chan *kv)); err == nil {
// 		b.Error("expected error m4")
// 		b.FailNow()
// 	}

// 	if err := m6.Start(context.Background(), make(chan *kv)); err == nil {
// 		b.Error("expected error m4")
// 		b.FailNow()
// 	}

// 	if err := m7.Start(context.Background(), make(chan *kv)); err == nil {
// 		b.Error("expected error m4")
// 		b.FailNow()
// 	}
// }

func Test_Loop(b *testing.T) {
	count := 10000
	channel := make(chan *kv)
	go func() {
		for n := 0; n < count; n++ {
			channel <- deepcopy(testPayloadBase)
		}
	}()
	startFn, m := New("machine_id",
		channel,
		&Option[*kv]{},
	)

	counter := 1
	left, right := m.
		Then(
			func(m *kv) *kv {
				return m
			},
		).
		While(
			func(a *kv) bool {
				counter++
				return counter%2 == 0
			},
		)

	counter2 := 1
	inside, _ := left.While(
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

	out := right.Output()

	ctx, cancel := context.WithCancel(context.Background())

	startFn(ctx)

	for n := 0; n < count; n++ {
		<-out
	}

	cancel()

	<-time.After(100 * time.Millisecond)
}
