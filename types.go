// Package machine - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.
package machine

import (
	"context"
	"log/slog"
	"os"
	"time"
)

var logger = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{}))

// Monad is a function that is applied to payload and used for transformations
type Monad[T any] func(d T) T

// Filter is a function that can be used to filter the payload.
type Filter[T any] func(d T) bool

// Edge is an interface that is used for transferring data between vertices
type Edge[T any] interface {
	Output() chan T
	Send(payload T)
}

type Option interface {
	apply(*config)
}

type option struct {
	fn func(*config)
}

func (o *option) apply(c *config) {
	o.fn(c)
}

// OptionFIF0 controls the processing order of the payloads
// If set to true the system will wait for one payload
// to be processed before starting the next.
var OptionFIF0 Option = &option{
	func(c *config) {
		c.fifo = true
	},
}

// OptionBufferSize sets the buffer size on the edge channels between the
// vertices, this setting can be useful when processing large amounts
// of data with FIFO turned on.
func OptionBufferSize(size int) Option {
	return &option{
		func(c *config) {
			c.bufferSize = size
		},
	}
}

// OptionDebug enables debug logging for the machine
var OptionDebug Option = &option{
	func(c *config) {
		c.debug = true
	},
}

type config struct {
	fifo       bool
	bufferSize int
	debug      bool
}

type vertex[T any] func(payload T)

type recursiveBaseFn[T any] func(recursiveBaseFn[T]) Monad[T]
type memoizedBaseFn[T any] func(h memoizedBaseFn[T], m map[string]T) Monad[T]

type filterList[T any] []Filter[T]
type filterComponent[T any] func(left, right chan T) vertex[T]

func (x Monad[T]) component(output chan T) vertex[T] {
	return func(payload T) {
		output <- x(payload)
	}
}

func (x filterList[T]) or() Filter[T] {
	if len(x) == 1 {
		return x[0]
	}

	return func(d T) bool {
		return x[0](d) || x[1:].or()(d)
	}
}

func (x filterList[T]) and() Filter[T] {
	if len(x) == 1 {
		return x[0]
	}

	return func(d T) bool {
		return x[0](d) && x[1:].and()(d)
	}
}

func (x Filter[T]) component(left, right chan T) vertex[T] {
	return func(payload T) {
		fr := x(payload)

		if fr {
			left <- payload
		} else {
			right <- payload
		}
	}
}

func (x vertex[T]) wrap(name string, option *config) vertex[T] {
	return func(payload T) {
		start := time.Now()

		defer recoverFn(name, start, payload, option)

		x(payload)
	}
}

func (x vertex[T]) run(ctx context.Context, name string, channel chan T, option *config) {
	h := x.wrap(name, option)

	go func() {
	Loop:
		for {
			select {
			case <-ctx.Done():
				break Loop
			case data := <-channel:
				if option.fifo {
					h(data)
				} else {
					go h(data)
				}
			}
		}
	}()
}

func recoverFn[T any](name string, start time.Time, payload T, option *config) {
	var err error

	if r := recover(); r != nil {
		err, _ = r.(error)
	}

	duration := time.Since(start)

	if option.debug {
		logger.Debug(name,
			slog.Any("payload", payload),
			slog.Any("error", err),
			slog.Duration("duration", duration),
		)
	}
}

func DebugSlogHandler(handler slog.Handler) {
	logger = slog.New(handler)
}
