// Package machine - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package machine

import (
  "bytes"
  "context"
  "encoding/gob"
  "encoding/json"
  "fmt"
  "strings"
  "testing"
  "time"

  "gopkg.in/yaml.v3"

  "github.com/whitaker-io/data"
)

type publishFN func([]data.Data) error

func (p publishFN) Send(payload []data.Data) error {
  return p(payload)
}

type tester struct {
  close error
}

func (t *tester) Read(ctx context.Context) []data.Data {
  out := []data.Data{}
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

func Test_Load(b *testing.T) {
  count := 100
  out := make(chan []data.Data)

  RegisterPluginProvider("test", &testPlugin{})

  streams := readStreamDefinitionsTestYamlFile(b)

  if bytez, err := yaml.Marshal(streams); err != nil {
    b.Error(err)
    b.FailNow()
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

  for _, s := range streams {
    if _, err := Load(s); err != nil && (s.Type == "subscription" || s.Type == "stream") {
      b.Error(err)
      b.FailNow()
    } else if _, err := LoadHTTP(s); err != nil && (s.Type == "http" || s.Type == "websocket") {
      b.Error(err)
      b.FailNow()
    }
  }

  stream, err := Load(streams[0])

  if err != nil {
    b.Error(err)
    b.FailNow()
  }

  go func() {
    if err := stream.Run(context.Background()); err != nil {
      b.Error(err)
    }
  }()

  for n := 0; n < count; n++ {
    list := <-out

    if len(list) != 1 {
      b.Errorf("incorrect data have %v want %v", list, testListBase[0])
      b.FailNow()
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
  } else if strings.Contains(pd.Symbol, "Sort") {
    return t.sort(pd.Attributes), nil
  } else if strings.Contains(pd.Symbol, "Remove") {
    return t.remove(pd.Attributes), nil
  } else if strings.Contains(pd.Symbol, "Publisher") {
    return t.publisher(pd.Attributes), nil
  }

  return nil, fmt.Errorf("not found")
}

func (t *testPlugin) Read(ctx context.Context) []data.Data {
  return deepCopy(testListBase)
}

func (t *testPlugin) Close() error {
  return nil
}

func (t *testPlugin) retriever(map[string]interface{}) Retriever {
  return func(ctx context.Context) chan []data.Data {
    channel := make(chan []data.Data)
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
  return func(data data.Data) data.Data {
    return data
  }
}

func (t *testPlugin) fold(map[string]interface{}) Fold {
  return func(aggregate, next data.Data) data.Data {
    return next
  }
}

func (t *testPlugin) fork(map[string]interface{}) Fork {
  return func(list []*Packet) (a []*Packet, b []*Packet) {
    return list, []*Packet{}
  }
}

func (t *testPlugin) sort(map[string]interface{}) Comparator {
  return func(a, b data.Data) int {
    return strings.Compare(a.MustString("name"), b.MustString("name"))
  }
}

func (t *testPlugin) remove(map[string]interface{}) Remover {
  return func(index int, d data.Data) bool {
    return false
  }
}

func (t *testPlugin) publisher(m map[string]interface{}) Publisher {
  var counter chan []data.Data
  if channel, ok := m["counter"]; ok {
    counter = channel.(chan []data.Data)
  }

  return publishFN(func(payload []data.Data) error {
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
        id: fold_id2
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
              id: loop_id1
              provider: 
                type: test
                symbol: Fork
                payload: ""
              in:
                map:
                  id: applicative_id2
                  provider: 
                    type: test
                    symbol: Applicative
                    payload: ""
                  fold_left:
                    id: fold_id3
                    provider: 
                      type: test
                      symbol: Fold
                      payload: ""
                    fold_right:
                      id: fold_id4
                      provider: 
                        type: test
                        symbol: Fold
                        payload: ""
                      fork:
                        id: fork_id2
                        provider: 
                          type: test
                          symbol: Fork
                          payload: ""
                        left:
                          map:
                            id: applicative_id3
                            provider: 
                              type: test
                              symbol: Applicative
                              payload: ""
                            sort:
                              id: sort_id1
                              provider: 
                                type: test
                                symbol: Sort
                                payload: ""
                              remove:
                                id: remove_id1
                                provider: 
                                  type: test
                                  symbol: Remover
                                  payload: ""

                        right:
                          loop2:
                            id: loop_id2
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
                                id: publisher_id2
                                provider: 
                                  type: test
                                  symbol: Publisher
                                  payload: ""
              out:
                publish:
                  id: publisher_id3
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
- type: websocket
  id: websocket_test_id
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
