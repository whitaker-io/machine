package loader

import (
	"fmt"

	"github.com/whitaker-io/machine"
)

type foldLeftLoader struct {
	loader
}

type foldRightLoader struct {
	loader
}

func (l *foldLeftLoader) load(v *VertexSerialization, b machine.Builder) error {
	if sym, err := l.loader.symbol(); err != nil {
		return err
	} else if x, ok := sym.(machine.Fold); !ok {
		return fmt.Errorf("invalid plugin type not fold")
	} else if v.next != nil {
		return v.next.loadable.load(v.next, b.FoldLeft(v.ID, x))
	}

	return nil
}

func (l *foldLeftLoader) Type() string {
	return "fold_left"
}

func (l *foldRightLoader) load(v *VertexSerialization, b machine.Builder) error {
	if sym, err := l.loader.symbol(); err != nil {
		return err
	} else if x, ok := sym.(machine.Fold); !ok {
		return fmt.Errorf("invalid plugin type not fold")
	} else if v.next != nil {
		return v.next.loadable.load(v.next, b.FoldRight(v.ID, x))
	}

	return nil
}

func (l *foldRightLoader) Type() string {
	return "fold_right"
}
