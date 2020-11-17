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

	_ = enc.Encode(testList)
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
			b.Errorf("incorrect data have %v want %v", list, testList[0])
		}
	}

	o := []Data{}
	buf := &bytes.Buffer{}
	enc, dec := gob.NewEncoder(buf), gob.NewDecoder(buf)

	_ = enc.Encode(&testList)
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
	<-time.After(1 * time.Second)
}

func Test_Pipe_HTTP(b *testing.T) {
	out := make(chan []Data)

	t := &tester{}

	p := NewPipe("pipe_id", t, t)

	p.StreamHTTP("http_id",
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

	bytez, _ := json.Marshal(testList)
	resp, err := p.app.Test(request(bytez), -1)

	if resp.StatusCode != http.StatusAccepted || err != nil {
		b.Error(resp.StatusCode, err)
	}

	bytez, _ = json.Marshal(testList[0])
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
		b.Errorf("incorrect data have %v want %v", list, testList)
	}

	list = <-out
	if len(list) != 1 {
		b.Errorf("incorrect data have %v want %v", list, testList[0])
	}

	cancel()
	<-time.After(1 * time.Second)
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
	<-time.After(1 * time.Second)
}

func request(bytez []byte) *http.Request {
	req, err := http.NewRequest(http.MethodPost, "http://localhost:5000/stream/http_id", bytes.NewReader(bytez))

	if err != nil {
		panic(err)
	}

	req.Header.Set("Content-Type", "application/json")

	return req
}
