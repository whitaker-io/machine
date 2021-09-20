// Package loader - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.

package loader

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
	"github.com/whitaker-io/machine"
)

var testListBase = []data.Data{
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

type publishFN func([]data.Data) error

func (p publishFN) Send(payload []data.Data) error {
	return p(payload)
}

func deepCopy(d []data.Data) []data.Data {
	out := []data.Data{}
	buf := &bytes.Buffer{}
	enc, dec := gob.NewEncoder(buf), gob.NewDecoder(buf)

	_ = enc.Encode(d)
	_ = dec.Decode(&out)

	return out
}

func Test_Load(b *testing.T) {
	count := 100
	tp := &testPlugin{
		out: make(chan []data.Data),
	}

	RegisterPluginProvider("test", tp)

	streams := readStreamDefinitionsTestYamlFile(b)

	if bytez, err := yaml.Marshal(streams); err != nil {
		b.Error(err)
		b.FailNow()
	} else {
		yaml.Unmarshal(bytez, &[]StreamSerialization{})
	}

	for _, s := range streams {
		if _, err := Load(s); err != nil && (s.Type() == "subscription" || s.Type() == "stream") {
			b.Error(err)
			b.FailNow()
		} else if _, err := LoadHTTP(s); err != nil && (s.Type() == "http" || s.Type() == "websocket") {
			b.Error(err)
			b.FailNow()
		}
	}

	stream, err := Load(streams[0])

	if err != nil {
		b.Error(err)
		b.FailNow()
	}

	if err := stream.Run(context.Background()); err != nil {
		b.Error(err)
		b.FailNow()
	}

	for n := 0; n < count; n++ {
		list := <-tp.out

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

type testPlugin struct {
	out chan []data.Data
}

func (t *testPlugin) Load(xType, payload, symbol string, attributes map[string]interface{}) (interface{}, error) {
	if strings.Contains(symbol, "Subscription") {
		return t, nil
	} else if strings.Contains(symbol, "Retriever") {
		return t.retriever(attributes), nil
	} else if strings.Contains(symbol, "Applicative") {
		return t.applicative(attributes), nil
	} else if strings.Contains(symbol, "Window") {
		return t.window(attributes), nil
	} else if strings.Contains(symbol, "Fork") {
		return t.fork(attributes), nil
	} else if strings.Contains(symbol, "Fold") {
		return t.fold(attributes), nil
	} else if strings.Contains(symbol, "Sort") {
		return t.sort(attributes), nil
	} else if strings.Contains(symbol, "Remove") {
		return t.remove(attributes), nil
	} else if strings.Contains(symbol, "Publisher") {
		return t.publisher(attributes), nil
	}

	return nil, fmt.Errorf("not found")
}

func (t *testPlugin) Read(ctx context.Context) []data.Data {
	return deepCopy(testListBase)
}

func (t *testPlugin) Close() error {
	return nil
}

func (t *testPlugin) retriever(map[string]interface{}) machine.Retriever {
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

func (t *testPlugin) applicative(map[string]interface{}) machine.Applicative {
	return func(data data.Data) data.Data {
		return data
	}
}

func (t *testPlugin) window(map[string]interface{}) machine.Window {
	return func(list ...*machine.Packet) []*machine.Packet {
		return list
	}
}

func (t *testPlugin) fold(map[string]interface{}) machine.Fold {
	return func(aggregate, next data.Data) data.Data {
		return next
	}
}

func (t *testPlugin) fork(map[string]interface{}) machine.Fork {
	return func(list []*machine.Packet) (a []*machine.Packet, b []*machine.Packet) {
		return list, []*machine.Packet{}
	}
}

func (t *testPlugin) sort(map[string]interface{}) machine.Comparator {
	return func(a, b data.Data) int {
		return strings.Compare(a.MustString("name"), b.MustString("name"))
	}
}

func (t *testPlugin) remove(map[string]interface{}) machine.Remover {
	return func(index int, d data.Data) bool {
		return false
	}
}

func (t *testPlugin) publisher(m map[string]interface{}) machine.Publisher {
	return publishFN(func(payload []data.Data) error {
		t.out <- payload
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
  window:
    id: window_id
    provider: 
      type: test
      symbol: Window
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
                            loop:
                              id: loop_id2
                              provider: 
                                type: test
                                symbol: Fork
                                payload: ""
                              in:
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
                out:
                  publish:
                    id: publisher_id4
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
