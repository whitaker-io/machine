package loader

import (
	"fmt"

	"github.com/whitaker-io/machine"
)

type loopLoader struct {
	loader
}

func (l *loopLoader) load(v *VertexSerialization, b machine.Builder) error {
	sym, err := l.loader.symbol()

	if err != nil {
		return err
	}

	x, ok := sym.(machine.Fork)

	if !ok {
		return fmt.Errorf("invalid plugin type not loop")
	}

	left, right := b.Loop(v.ID, x)

	if v.left != nil {
		if err := v.left.loadable.load(v.left, left); err != nil {
			return err
		}
	}

	if v.right != nil {
		return v.right.loadable.load(v.right, right)
	}

	return nil
}

func (l *loopLoader) Type() string {
	return "loop"
}
