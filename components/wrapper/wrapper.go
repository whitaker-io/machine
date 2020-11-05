package wrapper

import (
	"context"
	"log"

	"github.com/whitaker-io/machine"
)

// #########################
// WIP
// #########################

// Wrapper type for wrapping a machine.Machine and turning it into a machine.Processus
type Wrapper struct {
	input  chan []machine.Data
	output chan []machine.Data
}

// Builder func for defining the Builder for which the Wrapper wraps
func (w *Wrapper) Builder(id string, options ...*machine.Option) machine.Stream {
	w.input = make(chan []machine.Data)
	w.output = make(chan []machine.Data)
	initium := func(c context.Context) chan []machine.Data {
		return w.input
	}

	return machine.NewStream(id, initium, options...)
}

// Terminus func for providing the machine.Terminus that must be used to return values from the machine.Processus
func (w *Wrapper) Terminus() machine.Sender {
	return func(m []machine.Data) error {
		w.output <- m
		return nil
	}
}

// Wrap func for committing the Wrapper and providing the machine.Processus
func (w *Wrapper) Wrap() machine.Applicative {
	if w.input == nil {
		log.Fatalf("cannot wrap uninitialized Wrapper")
	}

	return func(m machine.Data) error {
		w.input <- []machine.Data{m}
		<-w.output
		return nil
	}
}
