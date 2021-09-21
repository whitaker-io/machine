// Package machine - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.
package machine

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/whitaker-io/data"
	"gopkg.in/yaml.v3"
)

func Test_Load(b *testing.T) {
	count := 100
	tp := &testPlugin{
		out: make(chan []data.Data),
	}

	RegisterPluginProvider("test.Subscription", tp.subscription)
	RegisterPluginProvider("test.Retriever", tp.retriever)
	RegisterPluginProvider("test.Applicative", tp.applicative)
	RegisterPluginProvider("test.Window", tp.window)
	RegisterPluginProvider("test.Sort", tp.sort)
	RegisterPluginProvider("test.Remover", tp.remove)
	RegisterPluginProvider("test.Fold", tp.fold)
	RegisterPluginProvider("test.Fork", tp.fork)
	RegisterPluginProvider("test.Publisher", tp.publisher)

	streams := readStreamDefinitionsTestYamlFile(b)

	if bytez, err := yaml.Marshal(streams); err != nil {
		b.Error(err)
		b.FailNow()
	} else {
		yaml.Unmarshal(bytez, &[]VertexSerialization{})
	}

	_, err := NewHTTPStreamPlugin(streams[1])

	if err != nil {
		b.Error(err)
		b.FailNow()
	}

	_, err = NewWebsocketStreamPlugin(streams[2])

	if err != nil {
		b.Error(err)
		b.FailNow()
	}

	_, err = NewStreamPlugin(streams[3])

	if err != nil {
		b.Error(err)
		b.FailNow()
	}

	stream, err := NewSubscriptionStreamPlugin(streams[0])

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

func Test_BadPlugins(b *testing.T) {
	tp := &testPlugin{
		out: make(chan []data.Data),
	}

	RegisterPluginProvider("test.Bad", tp.bad)

	v := &VertexSerialization{
		ID:       "test",
		Provider: "test.Bad",
		typeName: "bad",
	}

	v2 := &VertexSerialization{
		ID:       "test",
		Provider: "test.None",
		typeName: "bad",
	}

	_, err := v.subscription()

	if err == nil {
		b.Error("bad is a subscription")
		b.FailNow()
	}

	_, err = v2.subscription()

	if err == nil {
		b.Error("none is a subscription")
		b.FailNow()
	}

	_, err = v.retriever()

	if err == nil {
		b.Error("bad is a retriever")
		b.FailNow()
	}

	_, err = v2.retriever()

	if err == nil {
		b.Error("none is a retriever")
		b.FailNow()
	}

	_, err = v.applicative()

	if err == nil {
		b.Error("bad is a applicative")
		b.FailNow()
	}

	_, err = v2.applicative()

	if err == nil {
		b.Error("none is a applicative")
		b.FailNow()
	}

	_, err = v.window()

	if err == nil {
		b.Error("bad is a window")
		b.FailNow()
	}

	_, err = v2.window()

	if err == nil {
		b.Error("none is a window")
		b.FailNow()
	}

	_, err = v.comparator()

	if err == nil {
		b.Error("bad is a comparator")
		b.FailNow()
	}

	_, err = v2.comparator()

	if err == nil {
		b.Error("none is a comparator")
		b.FailNow()
	}

	_, err = v.remover()

	if err == nil {
		b.Error("bad is a remover")
		b.FailNow()
	}

	_, err = v2.remover()

	if err == nil {
		b.Error("none is a remover")
		b.FailNow()
	}

	_, err = v.fold()

	if err == nil {
		b.Error("bad is a fold")
		b.FailNow()
	}

	_, err = v2.fold()

	if err == nil {
		b.Error("none is a fold")
		b.FailNow()
	}

	_, err = v.fork()

	if err == nil {
		b.Error("bad is a fork")
		b.FailNow()
	}

	_, err = v2.fork()

	if err == nil {
		b.Error("none is a fork")
		b.FailNow()
	}

	_, err = v.publisher()

	if err == nil {
		b.Error("bad is a publisher")
		b.FailNow()
	}

	_, err = v2.publisher()

	if err == nil {
		b.Error("none is a publisher")
		b.FailNow()
	}
}

func readStreamDefinitionsTestYamlFile(b *testing.T) []*VertexSerialization {
	pd := []*VertexSerialization{}

	err := yaml.Unmarshal([]byte(streamDefinitions), &pd)

	if err != nil {
		b.Error(fmt.Sprintf("Unmarshal: %v", err))
	}

	return pd
}

type testPlugin struct {
	out chan []data.Data
}

func (t *testPlugin) Read(ctx context.Context) []data.Data {
	return deepCopy(testListBase)
}

func (t *testPlugin) Close() error {
	return nil
}

func (t *testPlugin) bad(map[string]interface{}) (interface{}, error) {
	return nil, nil
}

func (t *testPlugin) subscription(map[string]interface{}) (interface{}, error) {
	return Subscription(t), nil
}

func (t *testPlugin) retriever(map[string]interface{}) (interface{}, error) {
	return Retriever(func(ctx context.Context) chan []data.Data {
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
	}), nil
}

func (t *testPlugin) applicative(map[string]interface{}) (interface{}, error) {
	return Applicative(func(data data.Data) data.Data {
		return data
	}), nil
}

func (t *testPlugin) window(map[string]interface{}) (interface{}, error) {
	return Window(func(list ...*Packet) []*Packet {
		return list
	}), nil
}

func (t *testPlugin) fold(map[string]interface{}) (interface{}, error) {
	return Fold(func(aggregate, next data.Data) data.Data {
		return next
	}), nil
}

func (t *testPlugin) fork(map[string]interface{}) (interface{}, error) {
	return Fork(func(list []*Packet) (a []*Packet, b []*Packet) {
		return list, []*Packet{}
	}), nil
}

func (t *testPlugin) sort(map[string]interface{}) (interface{}, error) {
	return Comparator(func(a, b data.Data) int {
		return strings.Compare(a.MustString("name"), b.MustString("name"))
	}), nil
}

func (t *testPlugin) remove(map[string]interface{}) (interface{}, error) {
	return Remover(func(index int, d data.Data) bool {
		return false
	}), nil
}

func (t *testPlugin) publisher(m map[string]interface{}) (interface{}, error) {
	return publishFN(func(payload []data.Data) error {
		t.out <- payload
		return nil
	}), nil
}

var streamDefinitions = `- subscription:
    id: subscription_test_id
    provider: test.Subscription
    attributes:
      interval: 100000
    window:
      id: window_id
      provider: test.Window
      map:
        id: applicative_id
        provider: test.Applicative
        fold_left:
          id: fold_id
          provider: test.Fold
          fold_right:
            id: fold_id2
            provider: test.Fold
            fork:
              id: fork_id
              provider: test.Fork
              left:
                publish:
                  id: publisher_id
                  provider: test.Publisher
              right:
                loop:
                  id: loop_id1
                  provider: test.Fork
                  in:
                    map:
                      id: applicative_id2
                      provider: test.Applicative
                      fold_left:
                        id: fold_id3
                        provider: test.Fold
                        fold_right:
                          id: fold_id4
                          provider: test.Fold
                          fork:
                            id: fork_id2
                            provider: test.Fork
                            left:
                              map:
                                id: applicative_id3
                                provider: test.Applicative
                                sort:
                                  id: sort_id1
                                  provider: test.Sort
                                  remove:
                                    id: remove_id1
                                    provider: test.Remover

                            right:
                              loop:
                                id: loop_id2
                                provider: test.Fork
                                in:
                                  publish:
                                    id: publisher_id2
                                    provider: test.Publisher
                                out:
                                  publish:
                                    id: publisher_id3
                                    provider: test.Publisher
                  out:
                    publish:
                      id: publisher_id4
                      provider: test.Publisher
- http:
    id: http_test_id
    map:
      id: applicative_id
      provider: test.Applicative
      fold_left:
        id: fold_id
        provider: test.Fold
        fork:
          id: fork_id
          provider: test.Fork
          left:
            publish:
              id: publisher_id
              provider: test.Publisher
          right:
            publish:
              id: publisher_id
              provider: test.Publisher
- websocket:
    id: websocket_test_id
    map:
      id: applicative_id
      provider: test.Applicative
      fold_left:
        id: fold_id
        provider: test.Fold
        fork:
          id: fork_id
          provider: test.Fork
          left:
            publish:
              id: publisher_id
              provider: test.Publisher
          right:
            publish:
              id: publisher_id
              provider: test.Publisher
- stream:
    id: stream_test_id
    provider: test.Retriever
    map:
      id: applicative_id
      provider: test.Applicative
      fold_left:
        id: fold_id
        provider: test.Fold
        fork:
          id: fork_id
          provider: test.Fork
          left:
            publish:
              id: publisher_id
              provider: test.Publisher
          right:
            publish:
              id: publisher_id
              provider: test.Publisher`
