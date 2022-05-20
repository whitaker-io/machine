package machine

import "time"

// Option type for holding machine settings.
type Option[T Identifiable] struct {
	// FIFO controls the processing order of the payloads
	// If set to true the system will wait for one payload
	// to be processed before starting the next.
	// Default: false
	FIFO bool `json:"fifo,omitempty"`
	// BufferSize sets the buffer size on the edge channels between the
	// vertices, this setting can be useful when processing large amounts
	// of data with FIFO turned on.
	// Default: 0
	BufferSize int `json:"buffer_size,omitempty"`
	// MaxParallel sets the maximum number of parallel goroutines is only applicable when FIFO is turned off.
	// Default: 0
	MaxParallel int `json:"max_parallel,omitempty"`
	// Telemetry provides the ability to enable and configure telemetry
	Telemetry Telemetry[T] `json:"telemetry,omitempty"`
	// PanicHandler is a function that is called when a panic occurs
	// Default: log the panic and no-op
	PanicHandler func(err error, payload ...T) `json:"-"`
	// DeepCopy is a function to preform a deep copy of the Payload
	DeepCopy func(T) T `json:"-"`
}

// Telemetry type for holding telemetry settings.
type Telemetry[T Identifiable] interface {
	PayloadSize(string, int64)
	IncrementPayloadCount(string)
	IncrementErrorCount(string)
	Duration(string, time.Duration)
	StartSpan(string) Span[T]
}

// Span type for holding telemetry settings.
type Span[T Identifiable] interface {
	RecordPayload(payload ...T)
	RecordError(error)
	SpanEnd()
}

func (x *Option[T]) deepCopy(payload ...T) []T {
	if x.DeepCopy == nil {
		return payload
	}

	out := make([]T, len(payload))
	for i, p := range payload {
		out[i] = x.DeepCopy(p)
	}

	return out
}
