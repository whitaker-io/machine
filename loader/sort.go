package loader

import (
	"fmt"

	"github.com/whitaker-io/machine"
)

type sortLoader struct {
	loader
}

func (l *sortLoader) load(v *VertexSerialization, b machine.Builder) error {
	if sym, err := l.loader.symbol(); err != nil {
		return err
	} else if x, ok := sym.(machine.Comparator); !ok {
		return fmt.Errorf("invalid plugin type not comparator")
	} else if v.next != nil {
		return v.next.loadable.load(v.next, b.Sort(v.ID, x))
	}

	return nil
}

func (l *sortLoader) Type() string {
	return "sort"
}
