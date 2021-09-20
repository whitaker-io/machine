package loader

import (
	"fmt"

	"github.com/whitaker-io/machine"
)

type mapLoader struct {
	loader
}

func (l *mapLoader) load(v *VertexSerialization, b machine.Builder) error {
	if sym, err := l.loader.symbol(); err != nil {
		return err
	} else if x, ok := sym.(machine.Applicative); !ok {
		return fmt.Errorf("invalid plugin type not applicative")
	} else if v.next != nil {
		return v.next.loadable.load(v.next, b.Map(v.ID, x))
	}

	return nil
}

func (l *mapLoader) Type() string {
	return "map"
}
