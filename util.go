package machine

import (
	"bytes"
	"encoding/gob"
	"log"
)

func defaultOptions[T Identifiable]() *Option[T] {
	return &Option[T]{
		DeepCopy:   boolP(true),
		FIFO:       boolP(true),
		BufferSize: intP(0),
		Provider:   &edgeProvider[T]{},
		PanicHandler: func(streamID, vertexID string, err error, payload ...T) {
			log.Printf("stream: %s, vertex: %s, panic: %s, data %v", streamID, vertexID, err, payload)
		},
	}
}

func deepcopy[T Identifiable](d []T) []T {
	out := []T{}
	buf := &bytes.Buffer{}
	enc, dec := gob.NewEncoder(buf), gob.NewDecoder(buf)

	_ = enc.Encode(d)
	_ = dec.Decode(&out)

	return out
}

func boolP(v bool) *bool {
	return &v
}

func stringP(v string) *string {
	return &v
}

func intP(v int) *int {
	return &v
}
