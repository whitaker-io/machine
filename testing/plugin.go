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

// Applicative is a testing artifact used for plugins
var Applicative = func(map[string]interface{}) machine.Applicative {
	return func(data machine.Data) error {
		return nil
	}
}

// Fold is a testing artifact used for plugins
var Fold = func(map[string]interface{}) machine.Fold {
	return func(aggregate, next machine.Data) machine.Data {
		return next
	}
}

// Fork is a testing artifact used for plugins
var Fork = func(map[string]interface{}) machine.Fork {
	return func(list []*machine.Packet) (a []*machine.Packet, b []*machine.Packet) {
		return list, []*machine.Packet{}
	}
}

// Sender is a testing artifact used for plugins
var Sender = func(m map[string]interface{}) machine.Sender {
	counter := m["counter"].(chan []machine.Data)
	return func(payload []machine.Data) error {
		counter <- payload
		return nil
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
