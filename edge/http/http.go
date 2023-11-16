package http

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/whitaker-io/machine/v3"
)

type edge[T any] struct {
	httpClient http.Client
	fn         func(context.Context, T) *http.Request
	channel    chan T
}

func (e *edge[T]) Output() chan T {
	return e.channel
}

func (e *edge[T]) Send(ctx context.Context, data T) {
	res, err := e.httpClient.Do(e.fn(ctx, data))
	if err != nil {
		panic(err)
	}
	defer res.Body.Close()

	bytez := make([]byte, res.ContentLength)
	var out T
	if _, err := res.Body.Read(bytez); err != nil {
		panic(err)
	} else if err := json.Unmarshal(bytez, &out); err != nil {
		panic(err)
	}

	e.channel <- out
}

// New returns a function that can be used to make http requests
func New[T any](c http.Client, fn func(context.Context, T) *http.Request) machine.Edge[T] {
	return &edge[T]{httpClient: c, fn: fn, channel: make(chan T)}
}
