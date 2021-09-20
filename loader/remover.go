package loader

import (
	"fmt"

	"github.com/whitaker-io/machine"
)

type removerLoader struct {
	loader
}

func (l *removerLoader) load(v *VertexSerialization, b machine.Builder) error {
	if sym, err := l.loader.symbol(); err != nil {
		return err
	} else if x, ok := sym.(machine.Remover); !ok {
		return fmt.Errorf("invalid plugin type not remover")
	} else if v.next != nil {
		return v.next.loadable.load(v.next, b.Remove(v.ID, x))
	}

	return nil
}

func (l *removerLoader) Type() string {
	return "remove"
}
