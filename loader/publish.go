package loader

import (
	"fmt"

	"github.com/whitaker-io/machine"
)

type publishLoader struct {
	loader
}

func (l *publishLoader) load(v *VertexSerialization, b machine.Builder) error {
	if sym, err := l.loader.symbol(); err != nil {
		return err
	} else if x, ok := sym.(machine.Publisher); ok {
		b.Publish(v.ID, x)
		return nil
	}

	return fmt.Errorf("invalid plugin type not publisher")
}

func (l *publishLoader) Type() string {
	return "publish"
}
