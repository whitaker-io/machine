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

func Test_Load(b *testing.T) {
	RegisterPluginProvider("test", &testPlugin{})

	pd := readProviderDefinitionsTestYamlFile(b)

	if len(pd.Plugins) < 1 {
		b.Error("issue loading testing/loader_test.yaml")
	}

	if err := pd.Load(); err != nil {
		b.Error(fmt.Sprintf("error loading plugins %v ", err))
	}

	count := 100
	out := make(chan []Data)

	t := &tester{}

	p := NewPipe("pipe_id", t, t)

	streams := readStreamDefinitionsTestYamlFile(b)

	streams[0].next["map"].next["fold_left"].next["fork"].next["left"].next["transmit"].Attributes["counter"] = out

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

func request(bytez []byte) *http.Request {
	req, err := http.NewRequest(http.MethodPost, "http://localhost:5000/stream/http_id", bytes.NewReader(bytez))

	if err != nil {
		panic(err)
	}

	req.Header.Set("Content-Type", "application/json")

	return req
}

func readStreamDefinitionsTestYamlFile(b *testing.T) []StreamSerialization {
	pd := []StreamSerialization{}

	err := yaml.Unmarshal([]byte(streamDefinitions), &pd)

	if err != nil {
		b.Error(fmt.Sprintf("Unmarshal: %v", err))
	}

	return pd
}

func readProviderDefinitionsTestYamlFile(b *testing.T) *ProviderDefinitions {
	pd := &ProviderDefinitions{}

	err := yaml.Unmarshal([]byte(providerDefinitions), pd)

	if err != nil {
		b.Error(fmt.Sprintf("Unmarshal: %v", err))
	}

	return pd
}

type testPlugin struct{}

func (t *testPlugin) Load(pd *PluginDefinition) (interface{}, error) {
	if strings.Contains(pd.Symbol, "Subscription") {
		return func(map[string]interface{}) Subscription {
			return t
		}, nil
	} else if strings.Contains(pd.Symbol, "Retriever") {
		return t.retriever, nil
	} else if strings.Contains(pd.Symbol, "Applicative") {
		return t.applicative, nil
	} else if strings.Contains(pd.Symbol, "Fork") {
		return t.fork, nil
	} else if strings.Contains(pd.Symbol, "Fold") {
		return t.fold, nil
	} else if strings.Contains(pd.Symbol, "Sender") {
		return t.sender, nil
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

func (t *testPlugin) sender(m map[string]interface{}) Sender {
	var counter chan []Data
	if channel, ok := m["counter"]; ok {
		counter = channel.(chan []Data)
	}
	return func(payload []Data) error {
		counter <- payload
		return nil
	}
}

var providerDefinitions = `plugins:
  testSubscription:
    type: test
    symbol: testing.Subscription
    script: |
      package testing

      import (
        "bytes"
        "context"
        "encoding/gob"

        "github.com/whitaker-io/machine"
      )

      var data = []machine.Data{
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

      type testSub struct{}

      func (t *testSub) Read(ctx context.Context) []machine.Data {
        return deepCopy(data)
      }

      func (t *testSub) Close() error {
        return nil
      }

      // Subscription is a testing artifact used for plugins
      var Subscription = func(map[string]interface{}) machine.Subscription {
        return &testSub{}
      }

      func deepCopy(data []machine.Data) []machine.Data {
        out := []machine.Data{}
        buf := &bytes.Buffer{}
        enc, dec := gob.NewEncoder(buf), gob.NewDecoder(buf)

        _ = enc.Encode(data)
        _ = dec.Decode(&out)

        return out
      }
  testRetriever:
    type: test
    symbol: testing.Retriever
    script: |
      package testing

      import (
        "bytes"
        "context"
        "encoding/gob"
        "time"

        "github.com/whitaker-io/machine"
      )

      var data = []machine.Data{
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

      // Retriever is a testing artifact used for plugins
      var Retriever = func(map[string]interface{}) machine.Retriever {
        return func(ctx context.Context) chan []machine.Data {
          channel := make(chan []machine.Data)
          go func() {
          Loop:
            for {
              select {
              case <-ctx.Done():
                break Loop
              case <-time.After(time.Second):
                channel <- deepCopy(data)
              }
            }
          }()
          return channel
        }
      }

      func deepCopy(data []machine.Data) []machine.Data {
        out := []machine.Data{}
        buf := &bytes.Buffer{}
        enc, dec := gob.NewEncoder(buf), gob.NewDecoder(buf)

        _ = enc.Encode(data)
        _ = dec.Decode(&out)

        return out
      }
  testApplicative:
    type: test
    symbol: testing.Applicative
    script: |
      package testing

      import (
        "github.com/whitaker-io/machine"
      )

      // Applicative is a testing artifact used for plugins
      var Applicative = func(map[string]interface{}) machine.Applicative {
        return func(data machine.Data) error {
          return nil
        }
      }
  testFold:
    type: test
    symbol: testing.Fold
    script: |
      package testing

      import (
        "github.com/whitaker-io/machine"
      )

      // Fold is a testing artifact used for plugins
      var Fold = func(map[string]interface{}) machine.Fold {
        return func(aggregate, next machine.Data) machine.Data {
          return next
        }
      }
  testFork:
    type: test
    symbol: testing.Fork
    script: |
      package testing

      import (
        "github.com/whitaker-io/machine"
      )

      // Fork is a testing artifact used for plugins
      var Fork = func(map[string]interface{}) machine.Fork {
        return func(list []*machine.Packet) (a []*machine.Packet, b []*machine.Packet) {
          return list, []*machine.Packet{}
        }
      }
  testSender:
    type: test
    symbol: testing.Sender
    script: |
      package testing

      import (
        "github.com/whitaker-io/machine"
      )

      // Sender is a testing artifact used for plugins
      var Sender = func(m map[string]interface{}) machine.Sender {
        counter := m["counter"].(chan []machine.Data)
        return func(payload []machine.Data) error {
          counter <- payload
          return nil
        }
      }`

var streamDefinitions = `- type: subscription
  interval: 100000
  id: subscription_test_id
  provider: testSubscription
  map:
    id: applicative_id
    provider: testApplicative
    fold_left:
      id: fold_id
      provider: testFold
      fork:
        id: fork_id
        provider: testFork
        left:
          transmit:
            id: sender_id
            provider: testSender
        right:
          loop:
            id: loop_id
            provider: testFork
            in:
              map:
                id: applicative_id
                provider: testApplicative
                fold_left:
                  id: fold_id
                  provider: testFold
                  fork:
                    id: fork_id
                    provider: testFork
                    left:
                      map:
                        id: applicative_id
                        provider: testApplicative
                    right:
                      loop:
                        id: loop_id
                        provider: testFork
                        in:
                          transmit:
                            id: sender_id
                            provider: testSender
                        out:
                          transmit:
                            id: sender_id
                            provider: testSender
            out:
              transmit:
                id: sender_id
                provider: testSender
- type: http
  id: http_test_id
  map:
    id: applicative_id
    provider: testApplicative
    fold_left:
      id: fold_id
      provider: testFold
      fork:
        id: fork_id
        provider: testFork
        left:
          transmit:
            id: sender_id
            provider: testSender
        right:
          transmit:
            id: sender_id
            provider: testSender
- type: stream
  id: stream_test_id
  provider: testRetriever
  map:
    id: applicative_id
    provider: testApplicative
    fold_left:
      id: fold_id
      provider: testFold
      fork:
        id: fork_id
        provider: testFork
        left:
          transmit:
            id: sender_id
            provider: testSender
        right:
          transmit:
            id: sender_id
            provider: testSender`
