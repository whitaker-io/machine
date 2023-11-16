// Package machine - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.
package machine

import (
	"context"
	"log/slog"
	"time"

	"github.com/whitaker-io/machine/common"
)

// Monad is a function that is applied to data and used for transformations
type Monad[T any] func(d T) T

// Filter is a function that can be used to filter the data.
type Filter[T any] func(d T) bool

// Edge is an interface that is used for transferring data between vertices
type Edge[T any] interface {
	Output() chan T
	Send(ctx context.Context, data T)
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

// OptionAttributes apply the slog.Attr's to the machine metrics and spans
// Do not override the "name", "type", "duration", "error", or "value" attributes
func OptionAttributes(attributes ...slog.Attr) Option {
	return &option{func(c *config) { c.attributes = attributes }}
}

// OptionFlush attempts to send all data to the flushFN before exiting after the gracePeriod has expired
// Im looking for a good way to make this type specific, but want to avoid having to add separate option
// settings for the Transform function.
func OptionFlush(gracePeriod time.Duration, flushFN func(vertexName string, payload any)) Option {
	return &option{func(c *config) { c.flushFN = flushFN; c.gracePeriod = gracePeriod }}
}

type config struct {
	fifo        bool
	bufferSize  int
	attributes  []slog.Attr
	gracePeriod time.Duration
	flushFN     func(vertexName string, payload any)
}

type vertex[T any] func(ctx context.Context, data T)

type recursiveBaseFn[T any] func(recursiveBaseFn[T]) Monad[T]
type memoizedBaseFn[T any] func(h memoizedBaseFn[T], m map[string]T) Monad[T]

type monadList[T any] []Monad[T]
type filterList[T any] []Filter[T]
type filterComponent[T any] func(left, right chan T) vertex[T]

func (x Monad[T]) component(output chan T) vertex[T] {
	return func(ctx context.Context, data T) { output <- x(data) }
}
func (x monadList[T]) combine() Monad[T] {
	if len(x) == 1 {
		return x[0]
	}

	return func(data T) T {
		return x[1:].combine()(x[0](data))
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
	return func(ctx context.Context, data T) {
		if x(data) {
			left <- data
		} else {
			right <- data
		}
	}
}

func (x vertex[T]) wrap(name string) vertex[T] {
	return func(ctx context.Context, data T) {
		start := time.Now()

		spanHolder := map[string]any{}
		c := common.Store(ctx, &spanHolder)
		slog.LogAttrs(
			c,
			common.LevelTrace,
			name,
			slog.String("type", common.TraceStart),
		)

		slog.LogAttrs(
			c,
			common.LevelMetric,
			"machine.runs",
			slog.String("name", name),
			slog.String("type", common.MetricInt64Counter),
			slog.Int64("value", 1),
		)

		defer recoverFn(c, name, start)

		x(c, data)
	}
}

func (x vertex[T]) run(ctx context.Context, name string, channel chan T, option *config) {
	h := x.wrap(name)

	if option.fifo {
		go transfer(ctx, channel, h, name, option)
	} else {
		go transfer(ctx, channel, func(ctx context.Context, data T) { go h(ctx, data) }, name, option)
	}
}

func recoverFn(ctx context.Context, name string, start time.Time) {
	var err error

	duration := time.Since(start)
	if r := recover(); r != nil {
		err, _ = r.(error)
		slog.LogAttrs(
			ctx,
			common.LevelTrace,
			name,
			slog.String("type", common.TraceEvent),
			slog.Any("error", err),
		)
		slog.LogAttrs(
			ctx,
			common.LevelMetric,
			"machine.errors",
			slog.String("name", name),
			slog.String("type", common.MetricInt64Counter),
			slog.Int64("value", 1),
		)
	}

	slog.LogAttrs(
		ctx,
		common.LevelMetric,
		"machine.duration",
		slog.String("name", name),
		slog.String("type", common.MetricInt64Histogram),
		slog.Int64("value", duration.Milliseconds()),
	)
	slog.LogAttrs(
		ctx,
		common.LevelTrace,
		name,
		slog.String("type", common.TraceEnd),
		slog.Int64("duration", duration.Milliseconds()),
	)
}
