// Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package machine

import (
	"context"
	"encoding/gob"
	"fmt"
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
	m := New("machine_id", func(c context.Context) chan []idMap {
		return channel
	},
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(false),
			Metrics:    boolP(true),
			Span:       boolP(false),
			BufferSize: intP(0),
		},
	)

	out := m.Builder().
		Map("map_id", func(m idMap) idMap {
			if m.ID() == "" {
				b.Errorf("packet missing name %v", m)
			}
			return m
		}).Channel()

	if err := m.Run(context.Background()); err != nil {
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
	m := New("machine_id", func(c context.Context) chan []idMap {
		channel := make(chan []idMap)
		go func() {
			channel <- deepcopy(testListInvalidBase)
			for n := 0; n < count; n++ {
				channel <- deepcopy(testPayloadBase)
			}
		}()
		return channel
	},
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(false),
			Metrics:    boolP(true),
			Span:       boolP(false),
			BufferSize: intP(0),
		},
	)

	left, right := m.Builder().
		Map("map_id", func(m idMap) idMap {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
			}
			return m
		}).
		Sort("sort_id1", func(a, b idMap) int {
			return strings.Compare(a["name"].(string), b["name"].(string))
		}).
		Remove("remove_id1", func(index int, d idMap) bool {
			return false
		}).
		FoldLeft("fold_id1", func(d1, d2 idMap) idMap {
			return d1
		}).
		FoldLeft("fold_id2", func(d1, d2 idMap) idMap {
			return d1
		}).
		FoldRight("fold_id3", func(d1, d2 idMap) idMap {
			return d1
		}).
		Filter("fork_id", func(d idMap) bool { return true })

	out := left.Channel()

	right.Finally("sender_id2",
		func(m idMap) idMap {
			b.Error("unexpected")
			b.FailNow()
			return nil
		},
	)

	if err := m.Run(context.Background()); err != nil {
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
	m := New("machine_id", func(c context.Context) chan []idMap {
		channel := make(chan []idMap)
		go func() {
			for n := 0; n < count; n++ {
				channel <- deepcopy(testPayloadBase)
			}
		}()
		return channel
	},
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(true),
			Metrics:    boolP(true),
			Span:       boolP(true),
			BufferSize: intP(1000),
		},
	)

	left, right := m.Builder().
		Map("map_id", func(m idMap) idMap {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
				b.FailNow()
			}
			return m
		}).
		FoldRight("fold_id2", func(d1, d2 idMap) idMap {
			return d1
		}).
		Duplicate("fork_id")

	out := left.Channel()

	l2, r2 := right.Filter("fork_id1", func(d idMap) bool {
		return true
	})

	l3, r3 := l2.Filter("fork_id2", func(d idMap) bool {
		return false
	})

	r2.Finally("sender_id2",
		func(m idMap) idMap {
			fmt.Println("fail")
			b.Error("unexpected")
			b.FailNow()
			return nil
		},
	)

	l3.Finally("sender_id3",
		func(m idMap) idMap {
			fmt.Println("fail")
			b.Error("unexpected")
			b.FailNow()
			return nil
		},
	)

	out2 := r3.Channel()

	if err := m.Run(context.Background()); err != nil {
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
	m := New("machine_id", func(c context.Context) chan []idMap {
		channel := make(chan []idMap)
		go func() {
			for n := 0; n < count; n++ {
				channel <- deepcopy(testPayloadBase)
			}
		}()
		return channel
	},
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(true),
			Metrics:    boolP(true),
			Span:       boolP(true),
			BufferSize: intP(1000),
		},
	)

	m.Builder().
		Map("map_id", func(m idMap) idMap {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
			}
			return m
		}).
		FoldRight("fold_id2", func(d1, d2 idMap) idMap {
			panic("panic")
		}).Channel()

	if err := m.Run(context.Background()); err != nil {
		b.Error(err)
		b.FailNow()
	}
}

func Test_Missing_Leaves(b *testing.T) {
	m := New("machine_id", func(c context.Context) chan []idMap {
		channel := make(chan []idMap)
		return channel
	})

	left, _ := m.Builder().Duplicate("fork_id")

	left.Finally("sender_id",
		func(m idMap) idMap {
			b.Error("unexpected")
			b.FailNow()
			return nil
		},
	)

	m2 := New("machine_id", func(c context.Context) chan []idMap {
		channel := make(chan []idMap)
		return channel
	},
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(true),
			Metrics:    boolP(true),
			Span:       boolP(true),
			BufferSize: intP(1000),
		},
	)

	m2.Builder().Duplicate("fork_id")

	m3 := New("machine_id", func(c context.Context) chan []idMap {
		channel := make(chan []idMap)
		return channel
	},
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(true),
			Metrics:    boolP(true),
			Span:       boolP(true),
			BufferSize: intP(1000),
		},
	)

	m4 := New("machine_id", func(c context.Context) chan []idMap {
		channel := make(chan []idMap)
		return channel
	},
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(true),
			Metrics:    boolP(true),
			Span:       boolP(true),
			BufferSize: intP(1000),
		},
	)

	m4.Builder().Map("map_id", func(m idMap) idMap {
		if _, ok := m["name"]; !ok {
			b.Errorf("packet missing name %v", m)
		}
		return m
	})

	m5 := New("machine_id", func(c context.Context) chan []idMap {
		channel := make(chan []idMap)
		return channel
	},
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(true),
			Metrics:    boolP(true),
			Span:       boolP(true),
			BufferSize: intP(1000),
		},
	)

	m5.Builder().FoldRight("fold_id2", func(d1, d2 idMap) idMap {
		return d1
	})

	m6 := New("machine_id", func(c context.Context) chan []idMap {
		channel := make(chan []idMap)
		return channel
	},
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(true),
			Metrics:    boolP(true),
			Span:       boolP(true),
			BufferSize: intP(1000),
		},
	)

	l6, r6 := m6.Builder().Duplicate("fork_id")

	l6.Map("map_id", func(m idMap) idMap {
		if _, ok := m["name"]; !ok {
			b.Errorf("packet missing name %v", m)
		}
		return m
	})

	r6.Map("map_id", func(m idMap) idMap {
		if _, ok := m["name"]; !ok {
			b.Errorf("packet missing name %v", m)
		}
		return m
	})

	m7 := New("machine_id", func(c context.Context) chan []idMap {
		channel := make(chan []idMap)
		return channel
	},
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(true),
			Metrics:    boolP(true),
			Span:       boolP(true),
			BufferSize: intP(1000),
		},
	)

	l7, r7 := m7.Builder().Duplicate("fork_id")

	l7.Map("map_id", func(m idMap) idMap {
		if _, ok := m["name"]; !ok {
			b.Errorf("packet missing name %v", m)
		}
		return m
	}).Finally("sender_id",
		func(m idMap) idMap {
			b.Error("unexpected")
			b.FailNow()
			return nil
		},
	)

	r7.FoldLeft("fold_id2", func(d1, d2 idMap) idMap {
		return d1
	})

	m8 := New("machine_id", func(c context.Context) chan []idMap {
		channel := make(chan []idMap)
		return channel
	},
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(true),
			Metrics:    boolP(true),
			Span:       boolP(true),
			BufferSize: intP(1000),
		},
	)

	m8.Builder().Loop("loop_id", func(a idMap) bool {
		return false
	})

	m9 := New("machine_id", func(c context.Context) chan []idMap {
		channel := make(chan []idMap)
		return channel
	},
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(true),
			Metrics:    boolP(true),
			Span:       boolP(true),
			BufferSize: intP(1000),
		},
	)

	m9.Builder().Sort("sort_id", func(a, b idMap) int {
		return 0
	})

	m10 := New("machine_id", func(c context.Context) chan []idMap {
		channel := make(chan []idMap)
		return channel
	},
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(true),
			Metrics:    boolP(true),
			Span:       boolP(true),
			BufferSize: intP(0),
		},
	)

	m10.Builder().Remove("remove_id", func(index int, d idMap) bool {
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
	count := 10
	channel := make(chan []idMap)
	m := New("machine_id", func(c context.Context) chan []idMap {
		go func() {
			for n := 0; n < count; n++ {
				channel <- deepcopy(testPayloadBase)
				channel <- nil
			}
		}()
		return channel
	},
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(false),
			Metrics:    boolP(true),
			Span:       boolP(true),
			BufferSize: intP(0),
		},
	)

	left, right := m.Builder().
		Map("map_id", func(m idMap) idMap {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
			}
			return m
		}).
		FoldLeft("fold_idx", func(d1, d2 idMap) idMap {
			return d1
		}).
		FoldLeft("fold_id1", func(d1, d2 idMap) idMap {
			return d1
		}).
		FoldRight("fold_id2", func(d1, d2 idMap) idMap {
			return d1
		}).
		Filter("fork_id", func(a idMap) bool {
			return false
		})

	left.Finally("sender_id",
		func(m idMap) idMap {
			b.Error("unexpected")
			b.FailNow()
			return nil
		},
	)

	out := right.Channel()

	if err := m.Run(context.Background()); err != nil {
		b.Error(err)
		b.FailNow()
	}

	go func() {
		for n := 0; n < count; n++ {
			m.Inject("map_id", deepcopy(testPayloadBase)[0])
		}
	}()

	for n := 0; n < 2*count; n++ {
		list := <-out

		if len(list) != 1 {
			b.Errorf("incorrect data have %v want %v", list, testPayloadBase[0])
			b.FailNow()
		}
	}
}

func Test_Loop(b *testing.T) {
	count := 10
	m := New("machine_id", func(c context.Context) chan []idMap {
		channel := make(chan []idMap)
		go func() {
			for n := 0; n < count; n++ {
				channel <- deepcopy(testPayloadBase)
			}
		}()
		return channel
	},
		&Option[idMap]{
			DeepCopy:   boolP(true),
			FIFO:       boolP(false),
			Metrics:    boolP(true),
			Span:       boolP(false),
			BufferSize: intP(0),
		},
	)

	counter := 1
	left, right := m.Builder().
		Map("map_id", func(m idMap) idMap {
			if _, ok := m["name"]; !ok {
				b.Errorf("packet missing name %v", m)
				b.FailNow()
			}
			return m
		}).
		Loop("loop_id",
			func(a idMap) bool {
				counter++
				return counter%2 == 0
			},
		)

	counter2 := 1
	inside, _ := left.Loop("loop_id2",
		func(a idMap) bool {
			counter2++
			return counter2%2 == 0
		},
	)

	inside.Map("map_id2", func(m idMap) idMap {
		if _, ok := m["name"]; !ok {
			b.Errorf("packet missing name %v", m)
			b.FailNow()
		}
		return m
	})

	out := right.Channel()

	if err := m.Run(context.Background()); err != nil {
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
