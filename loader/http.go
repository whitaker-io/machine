package loader

import "github.com/whitaker-io/machine"

type httpLoader struct {
	loader
}

func (l *httpLoader) load(*VertexSerialization, machine.Builder) error {
	return nil
}

func (l *httpLoader) Type() string {
	return "http"
}
