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

	"github.com/google/uuid"
	"gopkg.in/yaml.v3"
)

type publishFN func([]Data) error

func (p publishFN) Send(payload []Data) error {
	return p(payload)
}

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

	p := NewPipe("pipe_id", nil, t)

	p.StreamSubscription("stream_id", t, 5*time.Millisecond,
		&Option{DeepCopy: boolP(true)},
		&Option{FIFO: boolP(false)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(false)},
		&Option{BufferSize: intP(0)},
	).Publish("publish_id",
		publishFN(func(d []Data) error {
			out <- d
			return nil
		}),
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
			VertexID:   "publish_id",
			VertexType: "publish",
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
			VertexID:   "publish_id",
			VertexType: "publish",
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

	p := NewPipe("pipe_id", nil, t)

	p.StreamHTTP("http_id",
		&Option{DeepCopy: boolP(true)},
		&Option{FIFO: boolP(true)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(false)},
		&Option{BufferSize: intP(0)},
	).Publish("publish_id",
		publishFN(func(d []Data) error {
			out <- d
			return nil
		}),
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

	p := NewPipe("pipe_id", nil, t)

	if err := p.Run(context.Background(), ":5000", time.Second); err == nil {
		b.Error("expected error")
	}
}

func Test_Pipe_Bad_Stream(b *testing.T) {
	t := &tester{}

	p := NewPipe("pipe_id", nil, t)

	p.StreamSubscription("stream_id", t, 5*time.Millisecond,
		&Option{DeepCopy: boolP(true)},
		&Option{FIFO: boolP(false)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(false)},
		&Option{BufferSize: intP(0)},
	).Map("publish_id",
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

	p := NewPipe("pipe_id", nil, t)

	p.StreamHTTP("http_id",
		&Option{DeepCopy: boolP(true)},
		&Option{FIFO: boolP(true)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(false)},
		&Option{BufferSize: intP(0)},
	).Publish("publish_id",
		publishFN(func(d []Data) error {
			out <- d
			return nil
		}),
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

	p := NewPipe("pipe_id", nil, t)

	p.StreamSubscription("stream_id", t, 5*time.Millisecond,
		&Option{DeepCopy: boolP(true)},
		&Option{FIFO: boolP(false)},
		&Option{Injectable: boolP(true)},
		&Option{Metrics: boolP(true)},
		&Option{Span: boolP(false)},
		&Option{BufferSize: intP(0)},
	).Publish("publish_id",
		publishFN(func(d []Data) error {
			out <- d
			return nil
		}),
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

func Test_Load(b *testing.T) {
	count := 100
	out := make(chan []Data)

	RegisterPluginProvider("test", &testPlugin{})

	t := &tester{}

	p := NewPipe("pipe_id", nil, t)

	streams := readStreamDefinitionsTestYamlFile(b)

	if bytez, err := yaml.Marshal(streams); err != nil {
		b.Error(err)
	} else {
		yaml.Unmarshal(bytez, &[]StreamSerialization{})
	}

	streams[0].
		next["map"].
		next["fold_left"].
		next["fold_right"].
		next["fork"].
		next["left"].
		next["publish"].
		Provider.Attributes = map[string]interface{}{
		"counter": out,
	}

	if err := p.Load(streams); err != nil {
		b.Error(err)
	}

	go func() {
		if err := p.Run(context.Background(), ":5000", time.Second); err != nil {
			b.Error(err)
		}
	}()

	for n := 0; n < count; n++ {
		list := <-out

		if len(list) != 1 {
			b.Errorf("incorrect data have %v want %v", list, testListBase[0])
		}
	}
}

func Test_Serialization(b *testing.T) {
	streams := readStreamDefinitionsTestYamlFile(b)

	var bytez []byte
	var err error
	if bytez, err = json.Marshal(streams); err != nil {
		b.Error(err)
	} else if err := json.Unmarshal(bytez, &streams); err != nil {
		b.Error(err)
	}

	if bytez, err = yaml.Marshal(streams); err != nil {
		b.Error(err)
	} else if err := yaml.Unmarshal(bytez, &streams); err != nil {
		b.Error(err)
	}
}

func request(bytez []byte) *http.Request {
	req, err := http.NewRequest(http.MethodPost, "http://localhost:5000/stream/http_id", bytes.NewReader(bytez))

	if err != nil {
		panic(err)
	}

	req.Header.Set("Content-Type", "application/json")

	return req
}

func readStreamDefinitionsTestYamlFile(b *testing.T) []*StreamSerialization {
	pd := []*StreamSerialization{}

	err := yaml.Unmarshal([]byte(streamDefinitions), &pd)

	if err != nil {
		b.Error(fmt.Sprintf("Unmarshal: %v", err))
	}

	return pd
}

type testPlugin struct{}

func (t *testPlugin) Load(pd *PluginDefinition) (interface{}, error) {
	if strings.Contains(pd.Symbol, "Subscription") {
		return t, nil
	} else if strings.Contains(pd.Symbol, "Retriever") {
		return t.retriever(pd.Attributes), nil
	} else if strings.Contains(pd.Symbol, "Applicative") {
		return t.applicative(pd.Attributes), nil
	} else if strings.Contains(pd.Symbol, "Fork") {
		return t.fork(pd.Attributes), nil
	} else if strings.Contains(pd.Symbol, "Fold") {
		return t.fold(pd.Attributes), nil
	} else if strings.Contains(pd.Symbol, "Publisher") {
		return t.publisher(pd.Attributes), nil
	}

	return nil, fmt.Errorf("not found")
}

func (t *testPlugin) Read(ctx context.Context) []Data {
	return deepCopy(testListBase)
}

func (t *testPlugin) Close() error {
	return nil
}

func (t *testPlugin) retriever(map[string]interface{}) Retriever {
	return func(ctx context.Context) chan []Data {
		channel := make(chan []Data)
		go func() {
		Loop:
			for {
				select {
				case <-ctx.Done():
					break Loop
				case <-time.After(time.Second):
					channel <- deepCopy(testListBase)
				}
			}
		}()
		return channel
	}
}

func (t *testPlugin) applicative(map[string]interface{}) Applicative {
	return func(data Data) error {
		return nil
	}
}

func (t *testPlugin) fold(map[string]interface{}) Fold {
	return func(aggregate, next Data) Data {
		return next
	}
}

func (t *testPlugin) fork(map[string]interface{}) Fork {
	return func(list []*Packet) (a []*Packet, b []*Packet) {
		return list, []*Packet{}
	}
}

func (t *testPlugin) publisher(m map[string]interface{}) Publisher {
	var counter chan []Data
	if channel, ok := m["counter"]; ok {
		counter = channel.(chan []Data)
	}

	return publishFN(func(payload []Data) error {
		counter <- payload
		return nil
	})
}

var streamDefinitions = `- type: subscription
  interval: 100000
  id: subscription_test_id
  provider: 
    type: test
    symbol: Subscription
    payload: ""
  map:
    id: applicative_id
    provider: 
      type: test
      symbol: Applicative
      payload: ""
    fold_left:
      id: fold_id
      provider: 
        type: test
        symbol: Fold
        payload: ""
      fold_right:
        id: fold_id
        provider: 
          type: test
          symbol: Fold
          payload: ""
        fork:
          id: fork_id
          provider: 
            type: test
            symbol: Fork
            payload: ""
          left:
            publish:
              id: publisher_id
              provider: 
                type: test
                symbol: Publisher
                payload: ""
          right:
            loop:
              id: loop_id
              provider: 
                type: test
                symbol: Fork
                payload: ""
              in:
                map:
                  id: applicative_id
                  provider: 
                    type: test
                    symbol: Applicative
                    payload: ""
                  fold_left:
                    id: fold_id
                    provider: 
                      type: test
                      symbol: Fold
                      payload: ""
                    fold_right:
                      id: fold_id
                      provider: 
                        type: test
                        symbol: Fold
                        payload: ""
                      fork:
                        id: fork_id
                        provider: 
                          type: test
                          symbol: Fork
                          payload: ""
                        left:
                          map:
                            id: applicative_id
                            provider: 
                              type: test
                              symbol: Applicative
                              payload: ""
                        right:
                          loop:
                            id: loop_id
                            provider: 
                              type: test
                              symbol: Fork
                              payload: ""
                            in:
                              publish:
                                id: publisher_id
                                provider: 
                                  type: test
                                  symbol: Publisher
                                  payload: ""
                            out:
                              publish:
                                id: publisher_id
                                provider: 
                                  type: test
                                  symbol: Publisher
                                  payload: ""
              out:
                publish:
                  id: publisher_id
                  provider: 
                    type: test
                    symbol: Publisher
                    payload: ""
- type: http
  id: http_test_id
  map:
    id: applicative_id
    provider:
      type: test
      symbol: Applicative
      payload: ""
    fold_left:
      id: fold_id
      provider: 
        type: test
        symbol: Fold
        payload: ""
      fork:
        id: fork_id
        provider: 
          type: test
          symbol: Fork
          payload: ""
        left:
          publish:
            id: publisher_id
            provider: 
              type: test
              symbol: Publisher
              payload: ""
        right:
          publish:
            id: publisher_id
            provider: 
              type: test
              symbol: Publisher
              payload: ""
- type: stream
  id: stream_test_id
  provider: 
    type: test
    symbol: Retriever
    payload: ""
  map:
    id: applicative_id
    provider: 
      type: test
      symbol: Applicative
      payload: ""
    fold_left:
      id: fold_id
      provider: 
        type: test
        symbol: Fold
        payload: ""
      fork:
        id: fork_id
        provider: 
          type: test
          symbol: Fork
          payload: ""
        left:
          publish:
            id: publisher_id
            provider: 
              type: test
              symbol: Publisher
              payload: ""
        right:
          publish:
            id: publisher_id
            provider: 
              type: test
              symbol: Publisher
              payload: ""`
