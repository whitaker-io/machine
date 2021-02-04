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

	"github.com/google/uuid"
)

type tester struct {
	close error
	join  error
	leave error
}

func (t *tester) Read(ctx context.Context) []Data {
	out := []Data{}
	buf := &bytes.Buffer{}
	enc, dec := gob.NewEncoder(buf), gob.NewDecoder(buf)

	_ = enc.Encode(testListBase)
	_ = dec.Decode(&out)
	return out
}

func (t *tester) Close() error {
	return t.close
}

func (t *tester) Error(...interface{}) {}
func (t *tester) Info(...interface{})  {}

func (t *tester) Join(id string, callback InjectionCallback, streamIDs ...string) error {
	return t.join
}

func (t *tester) Write(logs ...*Log) {}

func (t *tester) Leave(id string) error { return t.leave }

func Test_Pipe_Sub(b *testing.T) {
	count := 100
	out := make(chan []Data)

	t := &tester{}

	p := NewPipe("pipe_id", t, t)

	p.StreamSubscription("stream_id", t, 5*time.Millisecond,
		&Option{DeepCopy: boolP(true)},
		&Option{FIFO: boolP(false)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(false)},
		&Option{BufferSize: intP(0)},
	).Transmit("transmit_id",
		func(d []Data) error {
			out <- d
			return nil
		},
	)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		if err := p.Run(ctx, ":5000", time.Second); err != nil {
			b.Error(err)
		}
	}()

	for n := 0; n < count; n++ {
		list := <-out

		if len(list) != 10 && len(list) != 1 {
			b.Errorf("incorrect data have %v want %v", list, testListBase[0])
		}
	}

	o := []Data{}
	buf := &bytes.Buffer{}
	enc, dec := gob.NewEncoder(buf), gob.NewDecoder(buf)

	_ = enc.Encode(&testListBase)
	_ = dec.Decode(&o)

	if len(o) != 10 {
		b.Error("len of injection wrong")
	}

	logs := make([]*Log, len(o))
	logs2 := make([]*Log, len(o))
	for i, packet := range o {
		logs[i] = &Log{
			OwnerID:    "pipe_id",
			StreamID:   "stream_id",
			VertexID:   "transmit_id",
			VertexType: "transmit",
			State:      "start",
			Packet: &Packet{
				ID:   uuid.New().String(),
				Data: packet,
			},
			When: time.Now(),
		}
		logs2[i] = &Log{
			OwnerID:    "pipe_id",
			StreamID:   "bad_stream_id",
			VertexID:   "transmit_id",
			VertexType: "transmit",
			State:      "start",
			Packet: &Packet{
				ID:   uuid.New().String(),
				Data: packet,
			},
			When: time.Now(),
		}
	}

	p.injectionCallback(ctx)(logs...)
	p.injectionCallback(ctx)(logs2...)

	cancel()
	<-time.After(3 * time.Second)
}

func Test_Pipe_HTTP(b *testing.T) {
	out := make(chan []Data)

	t := &tester{}

	p := NewPipe("pipe_id", t, t)

	p.StreamHTTP("http_id",
		&Option{DeepCopy: boolP(true)},
		&Option{FIFO: boolP(true)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(false)},
		&Option{BufferSize: intP(0)},
	).Transmit("transmit_id",
		func(d []Data) error {
			out <- d
			return nil
		},
	)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		if err := p.Run(ctx, ":5000", time.Second); err != nil {
			b.Error(err)
		}
	}()

	bytez, _ := json.Marshal(deepCopy(testListBase))
	resp, err := p.app.Test(request(bytez), -1)

	if resp.StatusCode != http.StatusAccepted || err != nil {
		b.Error(resp.StatusCode, err)
	}

	bytez, _ = json.Marshal(testListBase[0])
	resp, err = p.app.Test(request(bytez), -1)

	if resp.StatusCode != http.StatusAccepted || err != nil {
		b.Error(resp.StatusCode, err)
	}

	bytez = []byte{}
	resp, err = p.app.Test(request(bytez), -1)

	if resp.StatusCode == http.StatusAccepted || err != nil {
		b.Error(resp.StatusCode, err)
	}

	list := <-out
	if len(list) != 10 {
		b.Errorf("incorrect data have %v want %v", list, testListBase)
	}

	list = <-out
	if len(list) != 1 {
		b.Errorf("incorrect data have %v want %v", list, testListBase[0])
	}

	cancel()
	<-time.After(3 * time.Second)
}

func Test_Pipe_No_Stream(b *testing.T) {
	t := &tester{
		join: fmt.Errorf("bad join"),
	}

	p := NewPipe("pipe_id", t, t)

	if err := p.Run(context.Background(), ":5000", time.Second); err == nil {
		b.Error("expected error")
	}
}

func Test_Pipe_Bad_Stream(b *testing.T) {
	t := &tester{}

	p := NewPipe("pipe_id", t, t)

	p.StreamSubscription("stream_id", t, 5*time.Millisecond,
		&Option{DeepCopy: boolP(true)},
		&Option{FIFO: boolP(false)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(false)},
		&Option{BufferSize: intP(0)},
	).Map("transmit_id",
		func(d Data) error {
			return nil
		},
	)

	if err := p.Run(context.Background(), ":5000", time.Second); err == nil {
		b.Error("expected error")
	}
}

func Test_Pipe_Bad_Join(b *testing.T) {
	out := make(chan []Data)

	t := &tester{
		join: fmt.Errorf("bad join"),
	}

	p := NewPipe("pipe_id", t, t)

	p.StreamHTTP("http_id",
		&Option{DeepCopy: boolP(true)},
		&Option{FIFO: boolP(true)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(false)},
		&Option{BufferSize: intP(0)},
	).Transmit("transmit_id",
		func(d []Data) error {
			out <- d
			return nil
		},
	)

	if err := p.Run(context.Background(), ":5000", time.Second); err == nil {
		b.Error("expected error")
	}
}

func Test_Pipe_Bad_Leave_Close(b *testing.T) {
	out := make(chan []Data)

	t := &tester{
		leave: fmt.Errorf("bad leave"),
		close: fmt.Errorf("bad close"),
	}

	p := NewPipe("pipe_id", t, t)

	p.StreamSubscription("stream_id", t, 5*time.Millisecond,
		&Option{DeepCopy: boolP(true)},
		&Option{FIFO: boolP(false)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(false)},
		&Option{BufferSize: intP(0)},
	).Transmit("transmit_id",
		func(d []Data) error {
			out <- d
			return nil
		},
	)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		if err := p.Run(ctx, ":5000", time.Second); err != nil {
			b.Error(err)
		}
	}()

	req, err := http.NewRequest(http.MethodGet, "http://localhost:5000/health", bytes.NewReader([]byte{}))

	if err != nil {
		panic(err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := p.app.Test(req, -1)

	if resp.StatusCode != http.StatusOK || err != nil {
		b.Error(resp.StatusCode, err)
	}

	cancel()
	<-time.After(3 * time.Second)
}

func request(bytez []byte) *http.Request {
	req, err := http.NewRequest(http.MethodPost, "http://localhost:5000/stream/http_id", bytes.NewReader(bytez))

	if err != nil {
		panic(err)
	}

	req.Header.Set("Content-Type", "application/json")

	return req
}

var subSerialization = &Serialization{
	ID:     "stream_id",
	Type:   "subscription",
	Symbol: "tester.Sub",
	Script: `package tester
	import (
		"context"

		"github.com/whitaker-io/machine"
	)

	var (
		Sub = func() machine.Subscription { return &t{} }
		testList = []machine.Data{
			{
				"name":  "data0",
				"value": 0,
			},
		}
	)

	type t struct {
		close error
	}

	func (t *t) Read(ctx context.Context) []machine.Data {
		return testList
	}

	func (t *t) Close() error {
		return t.close
	}`,
	Interval: 5 * time.Millisecond,
	Options: []*Option{
		{DeepCopy: boolP(true)},
		{FIFO: boolP(true)},
		{Injectable: boolP(true)},
		{Metrics: boolP(true)},
		{Span: boolP(false)},
		{BufferSize: intP(0)},
	},
	Next: &Serialization{
		ID:     "map_id",
		Type:   "map",
		Symbol: "tester.Map",
		Script: `package tester
		import "github.com/whitaker-io/machine"

		var Map = func(d machine.Data) error {
			return nil
		}
		`,
		Next: &Serialization{
			ID:     "fold_id",
			Type:   "fold_left",
			Symbol: "tester.Fold",
			Script: `package tester
			import "github.com/whitaker-io/machine"

			var Fold = func(aggragator machine.Data, next machine.Data) machine.Data {
				return next
			}
			`,
			Next: &Serialization{
				ID:     "fork_id",
				Type:   "fork",
				Symbol: "duplicate",
				Left: &Serialization{
					ID:     "fold_id2",
					Type:   "fold_right",
					Symbol: "tester.Fold",
					Script: `package tester
					import "github.com/whitaker-io/machine"

					var Fold = func(aggragator machine.Data, next machine.Data) machine.Data {
						return next
					}
					`,
					Next: &Serialization{
						ID:     "transmit_id",
						Type:   "transmit",
						Symbol: "tester.Sender",
						Script: `package tester
						import "github.com/whitaker-io/machine"

						var Sender = func(d []machine.Data) error {
							return nil
						}
						`,
					},
				},
				Right: &Serialization{
					ID:   "link_id",
					Type: "link",
					To:   "map_id",
				},
			},
		},
	},
}

func Test_Pipe_Load_Sub(b *testing.T) {
	t := &tester{}

	p := NewPipe("pipe_id", t, t)

	if err := p.Load(subSerialization); err != nil {
		b.Error(err)
	}
}

var httpSerialization = &Serialization{
	ID:   "http_id",
	Type: "http",
	Options: []*Option{
		{DeepCopy: boolP(true)},
		{FIFO: boolP(true)},
		{Injectable: boolP(true)},
		{Metrics: boolP(true)},
		{Span: boolP(false)},
		{BufferSize: intP(0)},
	},
	Next: &Serialization{
		ID:     "map_id",
		Type:   "map",
		Symbol: "tester.Map",
		Script: `package tester
		import "github.com/whitaker-io/machine"

		var Map = func(d machine.Data) error {
			return nil
		}
		`,
		Next: &Serialization{
			ID:     "fold_id",
			Type:   "fold_left",
			Symbol: "tester.Fold",
			Script: `package tester
			import "github.com/whitaker-io/machine"

			var Fold = func(aggragator machine.Data, next machine.Data) machine.Data {
				return next
			}
			`,
			Next: &Serialization{
				ID:     "fork_id",
				Type:   "fork",
				Symbol: "tester.Rule",
				Script: `package tester
				import "github.com/whitaker-io/machine"

				var Rule = func(machine.Data) bool {
					return true
				}
				`,
				Left: &Serialization{
					ID:     "fold_id2",
					Type:   "fold_right",
					Symbol: "tester.Fold",
					Script: `package tester
					import "github.com/whitaker-io/machine"

					var Fold = func(aggragator machine.Data, next machine.Data) machine.Data {
						return next
					}
					`,
					Next: &Serialization{
						ID:     "transmit_id",
						Type:   "transmit",
						Symbol: "tester.Sender",
						Script: `package tester
						import "github.com/whitaker-io/machine"

						var Sender = func(d []machine.Data) error {
							return nil
						}
						`,
					},
				},
				Right: &Serialization{
					ID:   "link_id",
					Type: "link",
					To:   "map_id",
				},
			},
		},
	},
}

func Test_Pipe_Load_HTTP(b *testing.T) {
	t := &tester{}

	p := NewPipe("pipe_id", t, t)

	if err := p.Load(httpSerialization); err != nil {
		b.Error(err)
	}
}

var streamSerialization = &Serialization{
	ID:     "stream_id",
	Type:   "stream",
	Symbol: "tester.Retriever",
	Script: `package tester
	import (
		"context"

		"github.com/whitaker-io/machine"
	)

	var (
		Retriever = func(context.Context) chan []machine.Data {
			channel := make(chan []machine.Data)

			return channel
		}
		testList = []machine.Data{
			{
				"name":  "data0",
				"value": 0,
			},
		}
	)
`,
	Interval: 5 * time.Millisecond,
	Options: []*Option{
		{DeepCopy: boolP(true)},
		{FIFO: boolP(true)},
		{Injectable: boolP(true)},
		{Metrics: boolP(true)},
		{Span: boolP(false)},
		{BufferSize: intP(0)},
	},
	Next: &Serialization{
		ID:     "map_id",
		Type:   "map",
		Symbol: "tester.Map",
		Script: `package tester
		import "github.com/whitaker-io/machine"

		var Map = func(d machine.Data) error {
			return nil
		}
		`,
		Next: &Serialization{
			ID:     "fold_id",
			Type:   "fold_left",
			Symbol: "tester.Fold",
			Script: `package tester
			import "github.com/whitaker-io/machine"

			var Fold = func(aggragator machine.Data, next machine.Data) machine.Data {
				return next
			}
			`,
			Next: &Serialization{
				ID:     "fork_id",
				Type:   "fork",
				Symbol: "error",
				Left: &Serialization{
					ID:     "fold_id2",
					Type:   "fold_right",
					Symbol: "tester.Fold",
					Script: `package tester
					import "github.com/whitaker-io/machine"

					var Fold = func(aggragator machine.Data, next machine.Data) machine.Data {
						return next
					}
					`,
					Next: &Serialization{
						ID:     "transmit_id",
						Type:   "transmit",
						Symbol: "tester.Sender",
						Script: `package tester
						import "github.com/whitaker-io/machine"

						var Sender = func(d []machine.Data) error {
							return nil
						}
						`,
					},
				},
				Right: &Serialization{
					ID:   "link_id",
					Type: "link",
					To:   "map_id",
				},
			},
		},
	},
}

func Test_Pipe_Load_Stream(b *testing.T) {
	t := &tester{}

	p := NewPipe("pipe_id", t, t)

	if err := p.Load(streamSerialization); err != nil {
		b.Error(err)
	}
}

var badTypeSerialization = &Serialization{
	ID:     "sub_id",
	Type:   "bad",
	Symbol: "tester.Retriever",
	Script: `package tester
	import (
		"context"

		"github.com/whitaker-io/machine"
	)

	var (
		Retriever = func(context.Context) chan []machine.Data {
			channel := make(chan []machine.Data)

			return channel
		}
		testList = []machine.Data{
			{
				"name":  "data0",
				"value": 0,
			},
		}
	)
`,
	Interval: 5 * time.Millisecond,
	Options: []*Option{
		{DeepCopy: boolP(true)},
		{FIFO: boolP(true)},
		{Injectable: boolP(true)},
		{Metrics: boolP(true)},
		{Span: boolP(false)},
		{BufferSize: intP(0)},
	},
}

var nonTerminatedSubscriptionSerialization = &Serialization{
	ID:     "sub_id",
	Type:   "subscription",
	Symbol: "tester.Retriever",
	Script: `package tester
	import (
		"context"

		"github.com/whitaker-io/machine"
	)

	var (
		Retriever = func(context.Context) chan []machine.Data {
			channel := make(chan []machine.Data)

			return channel
		}
		testList = []machine.Data{
			{
				"name":  "data0",
				"value": 0,
			},
		}
	)
`,
	Interval: 5 * time.Millisecond,
	Options: []*Option{
		{DeepCopy: boolP(true)},
		{FIFO: boolP(true)},
		{Injectable: boolP(true)},
		{Metrics: boolP(true)},
		{Span: boolP(false)},
		{BufferSize: intP(0)},
	},
}

var nonTerminatedHTTPSerialization = &Serialization{
	ID:     "stream_id",
	Type:   "http",
	Symbol: "tester.Retriever",
	Script: `package tester
	import (
		"context"

		"github.com/whitaker-io/machine"
	)

	var (
		Retriever = func(context.Context) chan []machine.Data {
			channel := make(chan []machine.Data)

			return channel
		}
		testList = []machine.Data{
			{
				"name":  "data0",
				"value": 0,
			},
		}
	)
`,
	Interval: 5 * time.Millisecond,
	Options: []*Option{
		{DeepCopy: boolP(true)},
		{FIFO: boolP(true)},
		{Injectable: boolP(true)},
		{Metrics: boolP(true)},
		{Span: boolP(false)},
		{BufferSize: intP(0)},
	},
}

var nonTerminatedStreamSerialization = &Serialization{
	ID:     "stream_id",
	Type:   "stream",
	Symbol: "tester.Retriever",
	Script: `package tester
	import (
		"context"

		"github.com/whitaker-io/machine"
	)

	var (
		Retriever = func(context.Context) chan []machine.Data {
			channel := make(chan []machine.Data)

			return channel
		}
		testList = []machine.Data{
			{
				"name":  "data0",
				"value": 0,
			},
		}
	)
`,
	Interval: 5 * time.Millisecond,
	Options: []*Option{
		{DeepCopy: boolP(true)},
		{FIFO: boolP(true)},
		{Injectable: boolP(true)},
		{Metrics: boolP(true)},
		{Span: boolP(false)},
		{BufferSize: intP(0)},
	},
}

var badSymbolSubscriptionSerialization = &Serialization{
	ID:     "sub_id",
	Type:   "subscription",
	Symbol: "tester.A",
	Script: `package tester
	import (
		"context"

		"github.com/whitaker-io/machine"
	)

	var (
		Retriever = func(context.Context) chan []machine.Data {
			channel := make(chan []machine.Data)

			return channel
		}
		testList = []machine.Data{
			{
				"name":  "data0",
				"value": 0,
			},
		}
	)
`,
	Interval: 5 * time.Millisecond,
	Options: []*Option{
		{DeepCopy: boolP(true)},
		{FIFO: boolP(true)},
		{Injectable: boolP(true)},
		{Metrics: boolP(true)},
		{Span: boolP(false)},
		{BufferSize: intP(0)},
	},
	Next: &Serialization{
		ID:     "transmit_id",
		Type:   "transmit",
		Symbol: "tester.Sender",
		Script: `package tester
		import "github.com/whitaker-io/machine"

		var Sender = func(d []machine.Data) error {
			return nil
		}
		`,
	},
}

var badSymbolStreamSerialization = &Serialization{
	ID:     "stream_id",
	Type:   "stream",
	Symbol: "tester.A",
	Script: `package tester
	import (
		"context"

		"github.com/whitaker-io/machine"
	)

	var (
		Retriever = func(context.Context) chan []machine.Data {
			channel := make(chan []machine.Data)

			return channel
		}
		testList = []machine.Data{
			{
				"name":  "data0",
				"value": 0,
			},
		}
	)
`,
	Interval: 5 * time.Millisecond,
	Options: []*Option{
		{DeepCopy: boolP(true)},
		{FIFO: boolP(true)},
		{Injectable: boolP(true)},
		{Metrics: boolP(true)},
		{Span: boolP(false)},
		{BufferSize: intP(0)},
	},
	Next: &Serialization{
		ID:     "transmit_id",
		Type:   "transmit",
		Symbol: "tester.Sender",
		Script: `package tester
		import "github.com/whitaker-io/machine"

		var Sender = func(d []machine.Data) error {
			return nil
		}
		`,
	},
}

var badScriptSubscriptionSerialization = &Serialization{
	ID:     "stream_id",
	Type:   "subscription",
	Symbol: "tester.Sub",
	Script: `package tester
	import (
		"context"

		"github.com/whitaker-io/machine"
	)

	var (
		Sub = func() int { return 0 }
		testList = []machine.Data{
			{
				"name":  "data0",
				"value": 0,
			},
		}
	)

	type t struct {
		close error
	}

	func (t *t) Read(ctx context.Context) []machine.Data {
		return testList
	}

	func (t *t) Close() error {
		return t.close
	}`,
	Interval: 5 * time.Millisecond,
	Options: []*Option{
		{DeepCopy: boolP(true)},
		{FIFO: boolP(true)},
		{Injectable: boolP(true)},
		{Metrics: boolP(true)},
		{Span: boolP(false)},
		{BufferSize: intP(0)},
	},
	Next: &Serialization{
		ID:     "transmit_id",
		Type:   "transmit",
		Symbol: "tester.Sender",
		Script: `package tester
		import "github.com/whitaker-io/machine"

		var Sender = func(d []machine.Data) error {
			return nil
		}
		`,
	},
}

var badScriptStreamSerialization = &Serialization{
	ID:     "stream_id",
	Type:   "stream",
	Symbol: "tester.Retriever",
	Script: `package tester
	import (
		"context"

		"github.com/whitaker-io/machine"
	)

	var (
		Retriever = func() int { return 0 }
		testList = []machine.Data{
			{
				"name":  "data0",
				"value": 0,
			},
		}
	)
`,
	Interval: 5 * time.Millisecond,
	Options: []*Option{
		{DeepCopy: boolP(true)},
		{FIFO: boolP(true)},
		{Injectable: boolP(true)},
		{Metrics: boolP(true)},
		{Span: boolP(false)},
		{BufferSize: intP(0)},
	},
	Next: &Serialization{
		ID:     "transmit_id",
		Type:   "transmit",
		Symbol: "tester.Sender",
		Script: `package tester
		import "github.com/whitaker-io/machine"

		var Sender = func(d []machine.Data) error {
			return nil
		}
		`,
	},
}

var badScriptNotFuncStreamSerialization = &Serialization{
	ID:     "stream_id",
	Type:   "stream",
	Symbol: "tester.Retriever",
	Script: `package tester
	import (
		"context"

		"github.com/whitaker-io/machine"
	)

	var (
		Retriever = 10
		testList = []machine.Data{
			{
				"name":  "data0",
				"value": 0,
			},
		}
	)
`,
	Interval: 5 * time.Millisecond,
	Options: []*Option{
		{DeepCopy: boolP(true)},
		{FIFO: boolP(true)},
		{Injectable: boolP(true)},
		{Metrics: boolP(true)},
		{Span: boolP(false)},
		{BufferSize: intP(0)},
	},
	Next: &Serialization{
		ID:     "transmit_id",
		Type:   "transmit",
		Symbol: "tester.Sender",
		Script: `package tester
		import "github.com/whitaker-io/machine"

		var Sender = func(d []machine.Data) error {
			return nil
		}
		`,
	},
}

var badTypeVertexSerialization = &Serialization{
	ID:     "stream_id",
	Type:   "stream",
	Symbol: "tester.Retriever",
	Script: `package tester
	import (
		"context"

		"github.com/whitaker-io/machine"
	)

	var (
		Retriever = func(context.Context) chan []machine.Data {
			channel := make(chan []machine.Data)

			return channel
		}
		testList = []machine.Data{
			{
				"name":  "data0",
				"value": 0,
			},
		}
	)
`,
	Interval: 5 * time.Millisecond,
	Options: []*Option{
		{DeepCopy: boolP(true)},
		{FIFO: boolP(true)},
		{Injectable: boolP(true)},
		{Metrics: boolP(true)},
		{Span: boolP(false)},
		{BufferSize: intP(0)},
	},
	Next: &Serialization{
		ID:     "transmit_id",
		Type:   "bad",
		Symbol: "tester.Sender",
		Script: `package tester
		import "github.com/whitaker-io/machine"

		var Sender = func(d []machine.Data) error {
			return nil
		}
		`,
	},
}

var invalidScriptVertexSerialization = &Serialization{
	ID:     "stream_id",
	Type:   "stream",
	Symbol: "tester.Retriever",
	Script: `package tester
	import (
		"context"

		"github.com/whitaker-io/machine"
	)

	var (
		Retriever = func(context.Context) chan []machine.Data {
			channel := make(chan []machine.Data)

			return channel
		}
		testList = []machine.Data{
			{
				"name":  "data0",
				"value": 0,
			},
		}
	)
`,
	Interval: 5 * time.Millisecond,
	Options: []*Option{
		{DeepCopy: boolP(true)},
		{FIFO: boolP(true)},
		{Injectable: boolP(true)},
		{Metrics: boolP(true)},
		{Span: boolP(false)},
		{BufferSize: intP(0)},
	},
	Next: &Serialization{
		ID:     "transmit_id",
		Type:   "transmit",
		Symbol: "tester.Sender",
		Script: `bad`,
	},
}

var invalidScriptFuncTransmitSerialization = &Serialization{
	ID:     "stream_id",
	Type:   "stream",
	Symbol: "tester.Retriever",
	Script: `package tester
	import (
		"context"

		"github.com/whitaker-io/machine"
	)

	var (
		Retriever = func(context.Context) chan []machine.Data {
			channel := make(chan []machine.Data)

			return channel
		}
		testList = []machine.Data{
			{
				"name":  "data0",
				"value": 0,
			},
		}
	)
`,
	Interval: 5 * time.Millisecond,
	Options: []*Option{
		{DeepCopy: boolP(true)},
		{FIFO: boolP(true)},
		{Injectable: boolP(true)},
		{Metrics: boolP(true)},
		{Span: boolP(false)},
		{BufferSize: intP(0)},
	},
	Next: &Serialization{
		ID:     "transmit_id",
		Type:   "transmit",
		Symbol: "tester.Sender",
		Script: `package tester
		import "github.com/whitaker-io/machine"

		var Sender = func(d []machine.Data) {}
		`,
	},
}

var invalidScriptFuncMapSerialization = &Serialization{
	ID:     "stream_id",
	Type:   "stream",
	Symbol: "tester.Retriever",
	Script: `package tester
	import (
		"context"

		"github.com/whitaker-io/machine"
	)

	var (
		Retriever = func(context.Context) chan []machine.Data {
			channel := make(chan []machine.Data)

			return channel
		}
		testList = []machine.Data{
			{
				"name":  "data0",
				"value": 0,
			},
		}
	)
`,
	Interval: 5 * time.Millisecond,
	Options: []*Option{
		{DeepCopy: boolP(true)},
		{FIFO: boolP(true)},
		{Injectable: boolP(true)},
		{Metrics: boolP(true)},
		{Span: boolP(false)},
		{BufferSize: intP(0)},
	},
	Next: &Serialization{
		ID:     "map_id",
		Type:   "map",
		Symbol: "tester.Map",
		Script: `package tester
		import "github.com/whitaker-io/machine"

		var Map = func(d machine.Data) {}
		`,
		Next: &Serialization{
			ID:     "transmit_id",
			Type:   "transmit",
			Symbol: "tester.Sender",
			Script: `package tester
			import "github.com/whitaker-io/machine"

			var Sender = func(d []machine.Data) error {
				return nil
			}
			`,
		},
	},
}

var invalidScriptNonTerminatedFuncMapSerialization = &Serialization{
	ID:     "stream_id",
	Type:   "stream",
	Symbol: "tester.Retriever",
	Script: `package tester
	import (
		"context"

		"github.com/whitaker-io/machine"
	)

	var (
		Retriever = func(context.Context) chan []machine.Data {
			channel := make(chan []machine.Data)

			return channel
		}
		testList = []machine.Data{
			{
				"name":  "data0",
				"value": 0,
			},
		}
	)
`,
	Interval: 5 * time.Millisecond,
	Options: []*Option{
		{DeepCopy: boolP(true)},
		{FIFO: boolP(true)},
		{Injectable: boolP(true)},
		{Metrics: boolP(true)},
		{Span: boolP(false)},
		{BufferSize: intP(0)},
	},
	Next: &Serialization{
		ID:     "map_id",
		Type:   "map",
		Symbol: "tester.Map",
		Script: `package tester
		import "github.com/whitaker-io/machine"

		var Map = func(d machine.Data) error {
			return nil
		}
		`,
	},
}

var invalidSymbolMapSerialization = &Serialization{
	ID:     "stream_id",
	Type:   "stream",
	Symbol: "tester.Retriever",
	Script: `package tester
	import (
		"context"

		"github.com/whitaker-io/machine"
	)

	var (
		Retriever = func(context.Context) chan []machine.Data {
			channel := make(chan []machine.Data)

			return channel
		}
		testList = []machine.Data{
			{
				"name":  "data0",
				"value": 0,
			},
		}
	)
`,
	Interval: 5 * time.Millisecond,
	Options: []*Option{
		{DeepCopy: boolP(true)},
		{FIFO: boolP(true)},
		{Injectable: boolP(true)},
		{Metrics: boolP(true)},
		{Span: boolP(false)},
		{BufferSize: intP(0)},
	},
	Next: &Serialization{
		ID:     "map_id",
		Type:   "map",
		Symbol: "tester.A",
		Script: `package tester
		import "github.com/whitaker-io/machine"

		var Map = func(d machine.Data) {
			return nil
		}
		`,
		Next: &Serialization{
			ID:     "transmit_id",
			Type:   "transmit",
			Symbol: "tester.Sender",
			Script: `package tester
			import "github.com/whitaker-io/machine"

			var Sender = func(d []machine.Data) error {
				return nil
			}
			`,
		},
	},
}

var invalidScriptFuncFoldSerialization = &Serialization{
	ID:     "stream_id",
	Type:   "stream",
	Symbol: "tester.Retriever",
	Script: `package tester
	import (
		"context"

		"github.com/whitaker-io/machine"
	)

	var (
		Retriever = func(context.Context) chan []machine.Data {
			channel := make(chan []machine.Data)

			return channel
		}
		testList = []machine.Data{
			{
				"name":  "data0",
				"value": 0,
			},
		}
	)
`,
	Interval: 5 * time.Millisecond,
	Options: []*Option{
		{DeepCopy: boolP(true)},
		{FIFO: boolP(true)},
		{Injectable: boolP(true)},
		{Metrics: boolP(true)},
		{Span: boolP(false)},
		{BufferSize: intP(0)},
	},
	Next: &Serialization{
		ID:     "fold_id",
		Type:   "fold_left",
		Symbol: "tester.Fold",
		Script: `package tester
			import "github.com/whitaker-io/machine"

			var Fold = func(aggragator machine.Data, next machine.Data) {}
			`,
		Next: &Serialization{
			ID:     "transmit_id",
			Type:   "transmit",
			Symbol: "tester.Sender",
			Script: `package tester
			import "github.com/whitaker-io/machine"

			var Sender = func(d []machine.Data) error {
				return nil
			}
			`,
		},
	},
}

var invalidScriptNonTerminatedFuncFoldSerialization = &Serialization{
	ID:     "stream_id",
	Type:   "stream",
	Symbol: "tester.Retriever",
	Script: `package tester
	import (
		"context"

		"github.com/whitaker-io/machine"
	)

	var (
		Retriever = func(context.Context) chan []machine.Data {
			channel := make(chan []machine.Data)

			return channel
		}
		testList = []machine.Data{
			{
				"name":  "data0",
				"value": 0,
			},
		}
	)
`,
	Interval: 5 * time.Millisecond,
	Options: []*Option{
		{DeepCopy: boolP(true)},
		{FIFO: boolP(true)},
		{Injectable: boolP(true)},
		{Metrics: boolP(true)},
		{Span: boolP(false)},
		{BufferSize: intP(0)},
	},
	Next: &Serialization{
		ID:     "fold_id",
		Type:   "fold_left",
		Symbol: "tester.Fold",
		Script: `package tester
			import "github.com/whitaker-io/machine"

			var Fold = func(aggragator machine.Data, next machine.Data) machine.Data {
				return next
			}
			`,
	},
}

var invalidSymbolFoldSerialization = &Serialization{
	ID:     "stream_id",
	Type:   "stream",
	Symbol: "tester.Retriever",
	Script: `package tester
	import (
		"context"

		"github.com/whitaker-io/machine"
	)

	var (
		Retriever = func(context.Context) chan []machine.Data {
			channel := make(chan []machine.Data)

			return channel
		}
		testList = []machine.Data{
			{
				"name":  "data0",
				"value": 0,
			},
		}
	)
`,
	Interval: 5 * time.Millisecond,
	Options: []*Option{
		{DeepCopy: boolP(true)},
		{FIFO: boolP(true)},
		{Injectable: boolP(true)},
		{Metrics: boolP(true)},
		{Span: boolP(false)},
		{BufferSize: intP(0)},
	},
	Next: &Serialization{
		ID:     "fold_id",
		Type:   "fold_left",
		Symbol: "tester.A",
		Script: `package tester
			import "github.com/whitaker-io/machine"

			var Fold = func(aggragator machine.Data, next machine.Data) machine.Data {
				return next
			}
			`,
		Next: &Serialization{
			ID:     "transmit_id",
			Type:   "transmit",
			Symbol: "tester.Sender",
			Script: `package tester
			import "github.com/whitaker-io/machine"

			var Sender = func(d []machine.Data) error {
				return nil
			}
			`,
		},
	},
}

var invalidScriptFuncForkSerialization = &Serialization{
	ID:     "stream_id",
	Type:   "stream",
	Symbol: "tester.Retriever",
	Script: `package tester
	import (
		"context"

		"github.com/whitaker-io/machine"
	)

	var (
		Retriever = func(context.Context) chan []machine.Data {
			channel := make(chan []machine.Data)

			return channel
		}
		testList = []machine.Data{
			{
				"name":  "data0",
				"value": 0,
			},
		}
	)
`,
	Interval: 5 * time.Millisecond,
	Options: []*Option{
		{DeepCopy: boolP(true)},
		{FIFO: boolP(true)},
		{Injectable: boolP(true)},
		{Metrics: boolP(true)},
		{Span: boolP(false)},
		{BufferSize: intP(0)},
	},
	Next: &Serialization{
		ID:     "fork_id",
		Type:   "fork",
		Symbol: "tester.Rule",
		Script: `package tester
		import "github.com/whitaker-io/machine"

		var Rule = func(machine.Data) {}
		`,
		Left: &Serialization{
			ID:     "transmit_id",
			Type:   "transmit",
			Symbol: "tester.A",
			Script: `package tester
			import "github.com/whitaker-io/machine"

			var Sender = func(d []machine.Data) error {
				return nil
			}
			`,
		},
		Right: &Serialization{
			ID:   "link_id",
			Type: "link",
			To:   "fork_id",
		},
	},
}

var invalidSymbolForkSerialization = &Serialization{
	ID:     "stream_id",
	Type:   "stream",
	Symbol: "tester.Retriever",
	Script: `package tester
	import (
		"context"

		"github.com/whitaker-io/machine"
	)

	var (
		Retriever = func(context.Context) chan []machine.Data {
			channel := make(chan []machine.Data)

			return channel
		}
		testList = []machine.Data{
			{
				"name":  "data0",
				"value": 0,
			},
		}
	)
`,
	Interval: 5 * time.Millisecond,
	Options: []*Option{
		{DeepCopy: boolP(true)},
		{FIFO: boolP(true)},
		{Injectable: boolP(true)},
		{Metrics: boolP(true)},
		{Span: boolP(false)},
		{BufferSize: intP(0)},
	},
	Next: &Serialization{
		ID:     "fork_id",
		Type:   "fork",
		Symbol: "tester.A",
		Script: `package tester
		import "github.com/whitaker-io/machine"

		var Rule = func(machine.Data) bool {
			return true
		}
		`,
		Left: &Serialization{
			ID:     "transmit_id",
			Type:   "transmit",
			Symbol: "tester.A",
			Script: `package tester
			import "github.com/whitaker-io/machine"

			var Sender = func(d []machine.Data) error {
				return nil
			}
			`,
		},
		Right: &Serialization{
			ID:   "link_id",
			Type: "link",
			To:   "fork_id",
		},
	},
}

var invalidForkErrorLeftSerialization = &Serialization{
	ID:     "stream_id",
	Type:   "stream",
	Symbol: "tester.Retriever",
	Script: `package tester
	import (
		"context"

		"github.com/whitaker-io/machine"
	)

	var (
		Retriever = func(context.Context) chan []machine.Data {
			channel := make(chan []machine.Data)

			return channel
		}
		testList = []machine.Data{
			{
				"name":  "data0",
				"value": 0,
			},
		}
	)
`,
	Interval: 5 * time.Millisecond,
	Options: []*Option{
		{DeepCopy: boolP(true)},
		{FIFO: boolP(true)},
		{Injectable: boolP(true)},
		{Metrics: boolP(true)},
		{Span: boolP(false)},
		{BufferSize: intP(0)},
	},
	Next: &Serialization{
		ID:     "fork_id",
		Type:   "fork",
		Symbol: "tester.Rule",
		Script: `package tester
		import "github.com/whitaker-io/machine"

		var Rule = func(machine.Data) bool {
			return true
		}
		`,
		Left: &Serialization{
			ID:     "transmit_id",
			Type:   "transmit",
			Symbol: "tester.A",
			Script: `package tester
			import "github.com/whitaker-io/machine"

			var Sender = func(d []machine.Data) error {
				return nil
			}
			`,
		},
		Right: &Serialization{
			ID:   "link_id",
			Type: "link",
			To:   "fork_id",
		},
	},
}

func Test_Pipe_Load_Bad_Stream(b *testing.T) {
	t := &tester{}

	p := NewPipe("pipe_id", t, t)

	if err := p.Load(nonTerminatedSubscriptionSerialization); err == nil {
		b.Error(fmt.Errorf("expected error"))
	}

	if err := p.Load(nonTerminatedStreamSerialization); err == nil {
		b.Error(fmt.Errorf("expected error"))
	}

	if err := p.Load(nonTerminatedHTTPSerialization); err == nil {
		b.Error(fmt.Errorf("expected error"))
	}

	if err := p.Load(badTypeSerialization); err == nil {
		b.Error(fmt.Errorf("expected error"))
	}

	if err := p.Load(badSymbolStreamSerialization); err == nil {
		b.Error(fmt.Errorf("expected error"))
	}

	if err := p.Load(badSymbolSubscriptionSerialization); err == nil {
		b.Error(fmt.Errorf("expected error"))
	}

	if err := p.Load(badScriptSubscriptionSerialization); err == nil {
		b.Error(fmt.Errorf("expected error"))
	}

	if err := p.Load(badScriptStreamSerialization); err == nil {
		b.Error(fmt.Errorf("expected error"))
	}

	if err := p.Load(badScriptNotFuncStreamSerialization); err == nil {
		b.Error(fmt.Errorf("expected error"))
	}

	if err := p.Load(badTypeVertexSerialization); err == nil {
		b.Error(fmt.Errorf("expected error"))
	}

	if err := p.Load(invalidScriptVertexSerialization); err == nil {
		b.Error(fmt.Errorf("expected error"))
	}
}

func Test_Pipe_Load_Bad_Stream2(b *testing.T) {
	t := &tester{}

	p := NewPipe("pipe_id", t, t)

	if err := p.Load(invalidScriptFuncTransmitSerialization); err == nil {
		b.Error(fmt.Errorf("expected error"))
	}

	if err := p.Load(invalidScriptFuncMapSerialization); err == nil {
		b.Error(fmt.Errorf("expected error"))
	}

	if err := p.Load(invalidSymbolMapSerialization); err == nil {
		b.Error(fmt.Errorf("expected error"))
	}

	if err := p.Load(invalidScriptNonTerminatedFuncMapSerialization); err == nil {
		b.Error(fmt.Errorf("expected error"))
	}

	if err := p.Load(invalidScriptFuncFoldSerialization); err == nil {
		b.Error(fmt.Errorf("expected error"))
	}

	if err := p.Load(invalidSymbolFoldSerialization); err == nil {
		b.Error(fmt.Errorf("expected error"))
	}

	if err := p.Load(invalidScriptNonTerminatedFuncFoldSerialization); err == nil {
		b.Error(fmt.Errorf("expected error"))
	}

	if err := p.Load(invalidForkErrorLeftSerialization); err == nil {
		b.Error(fmt.Errorf("expected error"))
	}

	if err := p.Load(invalidScriptFuncForkSerialization); err == nil {
		b.Error(fmt.Errorf("expected error"))
	}

	if err := p.Load(invalidSymbolForkSerialization); err == nil {
		b.Error(fmt.Errorf("expected error"))
	}
}
