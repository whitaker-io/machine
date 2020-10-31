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
	input  chan []map[string]interface{}
	output chan []map[string]interface{}
}

// Builder func for defining the Builder for which the Wrapper wraps
func (w *Wrapper) Builder(id string, options ...*machine.Option) *machine.Builder {
	w.input = make(chan []map[string]interface{})
	w.output = make(chan []map[string]interface{})
	initium := func(c context.Context) chan []map[string]interface{} {
		return w.input
	}

	return machine.New(id, initium, options...)
}

// Terminus func for providing the machine.Terminus that must be used to return values from the machine.Processus
func (w *Wrapper) Terminus() machine.Terminus {
	return func(m []map[string]interface{}) error {
		w.output <- m
		return nil
	}
}

// Wrap func for committing the Wrapper and providing the machine.Processus
func (w *Wrapper) Wrap() machine.Processus {
	if w.input == nil {
		log.Fatalf("cannot wrap uninitialized Wrapper")
	}

	return func(m map[string]interface{}) error {
		w.input <- []map[string]interface{}{m}
		<-w.output
		return nil
	}
}
