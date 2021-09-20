package loader

import "github.com/whitaker-io/machine"

type retrieverLoader struct {
	loader
}

func (l *retrieverLoader) load(*VertexSerialization, machine.Builder) error {
	return nil
}

func (l *retrieverLoader) Type() string {
	return "stream"
}
