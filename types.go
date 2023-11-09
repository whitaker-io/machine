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

// Monad is a function that is applied to data and used for transformations
type Monad[T any] func(d T) T

// Filter is a function that can be used to filter the data.
type Filter[T any] func(d T) bool

// Edge is an interface that is used for transferring data between vertices
type Edge[T any] interface {
	Output() chan T
	Send(data T)
}

// Option is used to configure the machine
type Option interface {
	apply(*config)
}

type option struct {
	fn func(*config)
}

func (o *option) apply(c *config) {
	o.fn(c)
}

// OptionFIF0 controls the processing order of the datas
// If set to true the system will wait for one data
// to be processed before starting the next.
var OptionFIF0 Option = &option{func(c *config) { c.fifo = true }}

// OptionBufferSize sets the buffer size on the edge channels between the
// vertices, this setting can be useful when processing large amounts
// of data with FIFO turned on.
func OptionBufferSize(size int) Option {
	return &option{func(c *config) { c.bufferSize = size }}
}

// OptionDebug enables debug logging for the machine
var OptionDebug Option = &option{func(c *config) { c.debug = true }}

type config struct {
	fifo       bool
	bufferSize int
	debug      bool
}

type vertex[T any] func(data T)

type recursiveBaseFn[T any] func(recursiveBaseFn[T]) Monad[T]
type memoizedBaseFn[T any] func(h memoizedBaseFn[T], m map[string]T) Monad[T]

type filterList[T any] []Filter[T]
type filterComponent[T any] func(left, right chan T) vertex[T]

func (x Monad[T]) component(output chan T) vertex[T] {
	return func(data T) { output <- x(data) }
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
	return func(data T) {
		if x(data) {
			left <- data
		} else {
			right <- data
		}
	}
}

func (x vertex[T]) wrap(name string, option *config) vertex[T] {
	return func(data T) {
		start := time.Now()

		defer recoverFn(name, start, data, option)

		x(data)
	}
}

func (x vertex[T]) run(ctx context.Context, name string, channel chan T, option *config) {
	h := x.wrap(name, option)

	if option.fifo {
		go transfer(ctx, channel, h)
	} else {
		go transfer(ctx, channel, func(data T) { go h(data) })
	}
}

func recoverFn[T any](name string, start time.Time, data T, option *config) {
	var err error

	if r := recover(); r != nil {
		err, _ = r.(error)
	}

	if option.debug {
		logger.Debug(name,
			slog.Any("data", data),
			slog.Any("error", err),
			slog.Duration("duration", time.Since(start)),
		)
	}
}

// DebugSlogHandler is used for setting the slog handler for the machine debug output
// the default is the stdlin JSON handler outputting to stdout
func DebugSlogHandler(handler slog.Handler) {
	logger = slog.New(handler)
}
