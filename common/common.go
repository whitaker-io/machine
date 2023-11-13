package common

import (
	"context"
	"log/slog"
)

const (
	LevelTrace             slog.Level = -16
	LevelMetric            slog.Level = -8
	TraceStart             string     = "start"
	TraceEvent             string     = "event"
	TraceEnd               string     = "end"
	MetricFloat64Counter   string     = "float64counter"
	MetricInt64Counter     string     = "int64counter"
	MetricFloat64Histogram string     = "float64histogram"
	MetricInt64Histogram   string     = "int64histogram"
	ctxKey                 key        = iota
)

type key int

func Store(ctx context.Context, m *map[string]any) context.Context {
	return context.WithValue(ctx, ctxKey, m)
}

func Get(ctx context.Context) (*map[string]any, bool) {
	if val := ctx.Value(ctxKey); val == nil {
		return nil, false
	} else if m, ok := val.(*map[string]any); !ok {
		return nil, false
	} else {
		return m, true
	}
}
