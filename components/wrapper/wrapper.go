package wrapper

import (
	"context"
	"log"

	"github.com/karlseguin/typed"
	"github.com/whitaker-io/machine"
)

// #########################
// WIP
// #########################

// Wrapper type for wrapping a machine.Machine and turning it into a machine.Processus
type Wrapper struct {
	input  chan []typed.Typed
	output chan []typed.Typed
}

// Builder func for defining the Builder for which the Wrapper wraps
func (w *Wrapper) Builder(id string, options ...*machine.Option) *machine.Builder {
	w.input = make(chan []typed.Typed)
	w.output = make(chan []typed.Typed)
	initium := func(c context.Context) chan []typed.Typed {
		return w.input
	}

	return machine.New(id, initium, options...)
}

// Terminus func for providing the machine.Terminus that must be used to return values from the machine.Processus
func (w *Wrapper) Terminus() machine.Sender {
	return func(m []typed.Typed) error {
		w.output <- m
		return nil
	}
}

// Wrap func for committing the Wrapper and providing the machine.Processus
func (w *Wrapper) Wrap() machine.Applicative {
	if w.input == nil {
		log.Fatalf("cannot wrap uninitialized Wrapper")
	}

	return func(m typed.Typed) error {
		w.input <- []typed.Typed{m}
		<-w.output
		return nil
	}
}
