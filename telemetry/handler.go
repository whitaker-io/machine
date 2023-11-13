// Package telemetry is a package that provides a slog.Handler that supports telemetry messages.
package telemetry

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/whitaker-io/machine/common"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"
)

var providerMap = map[string]func(m metric.Meter) func(name string) (recorder, error){
	common.MetricFloat64Counter: func(m metric.Meter) func(name string) (recorder, error) {
		return func(name string) (recorder, error) {
			x, err := m.Float64Counter(name)
			return func(
				ctx context.Context,
				val attribute.KeyValue,
				option metric.MeasurementOption,
			) {
				x.Add(ctx, val.Value.AsFloat64(), option)
			}, err
		}
	},
	common.MetricInt64Counter: func(m metric.Meter) func(name string) (recorder, error) {
		return func(name string) (recorder, error) {
			x, err := m.Int64Counter(name)
			return func(
				ctx context.Context,
				val attribute.KeyValue,
				option metric.MeasurementOption,
			) {
				x.Add(ctx, val.Value.AsInt64(), option)
			}, err
		}
	},
	common.MetricFloat64Histogram: func(m metric.Meter) func(name string) (recorder, error) {
		return func(name string) (recorder, error) {
			x, err := m.Float64Histogram(name)
			return func(
				ctx context.Context,
				val attribute.KeyValue,
				option metric.MeasurementOption,
			) {
				x.Record(ctx, val.Value.AsFloat64(), option)
			}, err
		}
	},
	common.MetricInt64Histogram: func(m metric.Meter) func(name string) (recorder, error) {
		return func(name string) (recorder, error) {
			x, err := m.Int64Histogram(name)
			return func(
				ctx context.Context,
				val attribute.KeyValue,
				option metric.MeasurementOption,
			) {
				x.Record(ctx, val.Value.AsInt64(), option)
			}, err
		}
	},
}

type recorder func(ctx context.Context, val attribute.KeyValue, options metric.MeasurementOption)

type handler struct {
	passthrough slog.Handler
	meter       metric.Meter
	tracer      trace.Tracer
	teeToLog    bool
	m           sync.Mutex
	metrics     map[string]recorder
	attributes  []attribute.KeyValue
}

// Handler is a handler that supports telemetry messages.
type Handler interface {
	slog.Handler
	WithFloat64Counter(name string, x metric.Float64Counter)
	WithInt64Counter(name string, x metric.Int64Counter)
	WithFloat64Histogram(name string, x metric.Float64Histogram)
	WithInt64Histogram(name string, x metric.Int64Histogram)
}

// New returns a new handler that wraps the provided handler and handles telemetry messages.
func New(
	logHandler slog.Handler,
	meter metric.Meter,
	tracer trace.Tracer,
	teeToLog bool,
	attributes ...attribute.KeyValue,
) Handler {
	if logHandler == nil {
		logHandler = slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
			Level: common.LevelTrace,
		})
	}
	return &handler{
		passthrough: logHandler,
		meter:       meter,
		tracer:      tracer,
		teeToLog:    teeToLog,
		metrics:     make(map[string]recorder),
		attributes:  attributes,
	}
}

// SpanStart starts a new span and returns a new context with the span attached.
func SpanStart(ctx context.Context, name string, attrs ...slog.Attr) context.Context {
	spanHolder := map[string]any{}
	//nolint
	c := common.Store(ctx, &spanHolder)
	slog.LogAttrs(c, common.LevelTrace, name, append(attrs, slog.String("type", common.TraceStart))...)
	return c
}

// SpanEvent adds an event to the span in the context.
func SpanEvent(ctx context.Context, name string, attrs ...slog.Attr) {
	slog.LogAttrs(ctx, common.LevelTrace, name, append(attrs, slog.String("type", common.TraceEvent))...)
}

// SpanEnd ends the span in the context.
func SpanEnd(ctx context.Context, name string, attrs ...slog.Attr) {
	slog.LogAttrs(ctx, common.LevelTrace, name, append(attrs, slog.String("type", common.TraceEnd))...)
}

// Float64Counter logs a float64 counter metric.
func Float64Counter(ctx context.Context, name string, value float64, attrs ...slog.Attr) {
	slog.LogAttrs(
		ctx,
		common.LevelMetric,
		name,
		append(
			attrs,
			slog.String("type", common.MetricFloat64Counter),
			slog.Float64("value", value),
		)...,
	)
}

// Int64Counter logs an int64 counter metric.
func Int64Counter(ctx context.Context, name string, value int64, attrs ...slog.Attr) {
	slog.LogAttrs(
		ctx,
		common.LevelMetric,
		name,
		append(
			attrs,
			slog.String("type", common.MetricInt64Counter),
			slog.Int64("value", value),
		)...,
	)
}

// Float64Histogram logs a float64 histogram metric.
func Float64Histogram(ctx context.Context, name string, value float64, attrs ...slog.Attr) {
	slog.LogAttrs(
		ctx,
		common.LevelMetric,
		name,
		append(
			attrs,
			slog.String("type", common.MetricFloat64Histogram),
			slog.Float64("value", value),
		)...,
	)
}

// Int64Histogram logs an int64 histogram metric.
func Int64Histogram(ctx context.Context, name string, value int64, attrs ...slog.Attr) {
	slog.LogAttrs(
		ctx,
		common.LevelMetric,
		name,
		append(
			attrs,
			slog.String("type", common.MetricInt64Histogram),
			slog.Int64("value", value),
		)...,
	)
}

// WithFloat64Counter adds a float64 counter metric to the handler.
func (h *handler) WithFloat64Counter(name string, x metric.Float64Counter) {
	h.addMetric(
		name,
		func(
			ctx context.Context,
			val attribute.KeyValue,
			option metric.MeasurementOption,
		) {
			x.Add(ctx, val.Value.AsFloat64(), option)
		},
	)
}

// WithInt64Counter adds an int64 counter metric to the handler.
func (h *handler) WithInt64Counter(name string, x metric.Int64Counter) {
	h.addMetric(
		name,
		func(
			ctx context.Context,
			val attribute.KeyValue,
			option metric.MeasurementOption,
		) {
			x.Add(ctx, val.Value.AsInt64(), option)
		},
	)
}

// WithFloat64Histogram adds a float64 histogram metric to the handler.
func (h *handler) WithFloat64Histogram(name string, x metric.Float64Histogram) {
	h.addMetric(
		name,
		func(
			ctx context.Context,
			val attribute.KeyValue,
			option metric.MeasurementOption,
		) {
			x.Record(ctx, val.Value.AsFloat64(), option)
		},
	)
}

// WithInt64Histogram adds an int64 histogram metric to the handler.
func (h *handler) WithInt64Histogram(name string, x metric.Int64Histogram) {
	h.addMetric(
		name,
		func(
			ctx context.Context,
			val attribute.KeyValue,
			option metric.MeasurementOption,
		) {
			x.Record(ctx, val.Value.AsInt64(), option)
		},
	)
}

func (h *handler) addMetric(name string, x recorder) {
	h.m.Lock()
	defer h.m.Unlock()
	h.metrics[name] = x
}

// Enabled returns true if the provided level is enabled.
func (h *handler) Enabled(ctx context.Context, level slog.Level) bool {
	return level == common.LevelTrace || level == common.LevelMetric || h.passthrough.Enabled(ctx, level)
}

// Handle handles the provided record.
func (h *handler) Handle(ctx context.Context, r slog.Record) error {
	defer recov()
	var err error

	switch r.Level {
	case common.LevelTrace:
		err = h.handleTrace(ctx, r)
	case common.LevelMetric:
		err = h.handleMetric(ctx, r)
	default:
		err = h.passthrough.Handle(ctx, r)
	}

	if err != nil {
		fmt.Println("telemetry/handler.go: 284", err, r)
	}

	return err
}

func recov() {
	if r := recover(); r != nil {
		fmt.Println("telemetry/handler.go: 292", r)
	}
}

// WithAttrs returns a new handler with the provided attributes.
func (h *handler) WithAttrs(attrs []slog.Attr) slog.Handler {
	for _, a := range attrs {
		h.attributes = append(h.attributes, convertAttr(a))
	}
	h.passthrough.WithAttrs(attrs)
	return h
}

// WithGroup returns a new handler with the provided group.
func (h *handler) WithGroup(name string) slog.Handler {
	h.passthrough.WithGroup(name)
	return h
}

func (h *handler) handleTrace(ctx context.Context, r slog.Record) error {
	attrs, flags := attrsFromRecord(r)
	if _, ok := flags["type"]; !ok {
		return fmt.Errorf("telemetry: invalid trace message format - missing operation")
	}

	operation := flags["type"].Value.AsString()
	message := r.Message
	attributes := append(h.attributes, attrs...)

	c, span, sphldr := getCtxAndSpan(ctx)
	if sphldr == nil {
		return fmt.Errorf("telemetry: sphldr not found in context %s", operation)
	} else if span == nil && operation != common.TraceStart {
		return fmt.Errorf("telemetry: span not found in context %s", operation)
	}
	switch operation {
	case common.TraceStart:
		(*sphldr)["ctx"], (*sphldr)["span"] = h.tracer.Start(
			c,
			message,
			trace.WithTimestamp(r.Time),
			trace.WithAttributes(attributes...),
		)
	case common.TraceEvent:
		span.AddEvent(message, trace.WithTimestamp(r.Time), trace.WithAttributes(attributes...))
	case common.TraceEnd:
		span.End(trace.WithTimestamp(r.Time))
		delete(*sphldr, "ctx")
		delete(*sphldr, "span")
	default:
		return fmt.Errorf("telemetry: invalid trace operation")
	}

	if h.teeToLog {
		return h.passthrough.Handle(ctx, r)
	}

	return nil
}

func (h *handler) handleMetric(ctx context.Context, r slog.Record) error {
	attrs, flags := attrsFromRecord(r)
	if _, ok := flags["type"]; !ok {
		return fmt.Errorf("telemetry: invalid metric message format - missing type")
	} else if _, ok := flags["value"]; !ok {
		return fmt.Errorf("telemetry: invalid metric message format - missing value")
	}
	metricType := flags["type"].Value.AsString()
	metricName := r.Message
	metricValue := flags["value"]
	attributes := metric.WithAttributes(append(h.attributes, attrs...)...)

	var rr recorder
	var err error
	if provider, ok := providerMap[metricType]; !ok {
		return fmt.Errorf("telemetry: invalid metric type")
	} else if rr, err = h.getRecorder(metricName, provider); err != nil {
		return err
	}

	rr(ctx, metricValue, attributes)

	if h.teeToLog {
		return h.passthrough.Handle(ctx, r)
	}

	return nil
}

func getCtxAndSpan(ctx context.Context) (context.Context, trace.Span, *map[string]any) {
	if sphldr, ok := common.Get(ctx); !ok {
		return ctx, nil, nil
	} else if cVal, ok := (*sphldr)["ctx"]; !ok {
		return ctx, nil, sphldr
	} else if c, ok := cVal.(context.Context); !ok {
		return ctx, nil, sphldr
	} else if spanVal, ok := (*sphldr)["span"]; !ok {
		return c, nil, sphldr
	} else if span, ok := spanVal.(trace.Span); !ok {
		return c, nil, sphldr
	} else {
		return c, span, sphldr
	}
}

func (h *handler) getRecorder(
	metricName string,
	provider func(metric.Meter) func(name string) (recorder, error),
) (rr recorder, err error) {
	h.m.Lock()
	defer h.m.Unlock()
	if _, ok := h.metrics[metricName]; !ok {
		h.metrics[metricName], err = provider(h.meter)(metricName)
	}
	return h.metrics[metricName], err
}

func attrsFromRecord(r slog.Record) ([]attribute.KeyValue, map[string]attribute.KeyValue) {
	attrs := make([]attribute.KeyValue, r.NumAttrs())
	flags := make(map[string]attribute.KeyValue)
	r.Attrs(func(a slog.Attr) bool {
		attr := convertAttr(a)
		attrs = append(attrs, attr)
		if a.Key == "type" {
			flags["type"] = attr
		} else if a.Key == "value" {
			flags["value"] = attr
		}
		return true
	})

	return attrs, flags
}

func convertAttr(a slog.Attr) attribute.KeyValue {
	switch a.Value.Kind() {
	case slog.KindString:
		return attribute.String(a.Key, a.Value.String())
	case slog.KindTime:
		return attribute.String(a.Key, a.Value.Time().Format(time.RFC3339Nano))
	case slog.KindBool:
		return attribute.Bool(a.Key, a.Value.Bool())
	case slog.KindInt64:
		return attribute.Int64(a.Key, a.Value.Int64())
	case slog.KindFloat64:
		return attribute.Float64(a.Key, a.Value.Float64())
	default:
		return attribute.String(a.Key, a.Value.String())
	}
}
