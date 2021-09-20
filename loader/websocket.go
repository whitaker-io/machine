package loader

import "github.com/whitaker-io/machine"

type websocketLoader struct {
	loader
}

func (l *websocketLoader) load(v *VertexSerialization, b machine.Builder) error {
	return nil
}

func (l *websocketLoader) Type() string {
	return "websocket"
}
