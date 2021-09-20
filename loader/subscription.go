package loader

import "github.com/whitaker-io/machine"

type subscriptionLoader struct {
	loader
}

func (l *subscriptionLoader) load(*VertexSerialization, machine.Builder) error {
	return nil
}

func (l *subscriptionLoader) Type() string {
	return "subscription"
}
