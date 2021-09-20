package loader

import (
	"fmt"

	"github.com/whitaker-io/machine"
)

type windowLoader struct {
	loader
}

func (l *windowLoader) load(v *VertexSerialization, b machine.Builder) error {
	if sym, err := l.loader.symbol(); err != nil {
		return err
	} else if x, ok := sym.(machine.Window); !ok {
		return fmt.Errorf("invalid plugin type not window")
	} else if v.next != nil {
		return v.next.loadable.load(v.next, b.Window(v.ID, x))
	}

	return nil
}

func (l *windowLoader) Type() string {
	return "window"
}
