package machine

import (
	"context"
	"fmt"
	"reflect"
	"time"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

const (
	stream      = "stream"
	applicative = "map"
	foldLeft    = "fold_left"
	foldRight   = "fold_right"
	fork        = "fork"
	link        = "link"
	transmit    = "transmit"
)

// Serialization type for holding information about github.com/traefik/yaegi based streams
type Serialization struct {
	// ID unique identifier for the stream.
	ID string `json:"id,omitempty" mapstructure:"id,omitempty"`
	// Type type of stream to create.
	//
	// For root serializations valid values are 'http', 'subscription', or 'stream'.
	//
	// For child serializations valid values are 'map', 'fold_left', 'fold_right', 'fork'
	// 'link', and 'transmit'
	Type string `json:"type,omitempty" mapstructure:"type,omitempty"`
	// Interval is the duration between pulls in a 'subscription' Type. It is only read
	// if the Type is 'subscription'.
	Interval time.Duration `json:"interval,omitempty" mapstructure:"interval,omitempty"`
	// Symbol is the name of the golang symbol that provides the target of the script,
	// this is typically the var/func name for the vertex in the script.
	Symbol string `json:"symbol,omitempty" mapstructure:"symbol,omitempty"`
	// Script is the yaegi script that contains the code for the vertex.
	// Symbols for the stdlib and machine are provided.
	Script string `json:"script,omitempty" mapstructure:"script,omitempty"`
	// Options are a slice of machine.Option https://godoc.org/github.com/whitaker-io/machine#Option
	Options []*Option `json:"options,omitempty" mapstructure:"options,omitempty"`
	// To is a reference ID used for the 'link' type. The value must be the ID of a predecessor
	// of this vertex
	To string `json:"to,omitempty" mapstructure:"to,omitempty"`
	// Next is the child vertex for every type other than Fork.
	Next *Serialization `json:"next,omitempty" mapstructure:"next,omitempty"`
	// Left is the left side child vertex for a Fork. It is only read for Forks.
	Left *Serialization `json:"left,omitempty" mapstructure:"left,omitempty"`
	// Right is the right side child vertex for a Fork. It is only read for Forks.
	Right *Serialization `json:"right,omitempty" mapstructure:"right,omitempty"`
}

// Load method loads a stream based on github.com/traefik/yaegi
func (pipe *Pipe) Load(lc *Serialization) error {
	switch lc.Type {
	case "http":
		if lc.Next == nil {
			return fmt.Errorf("non-terminated http stream %v", lc.ID)
		}
		return lc.Next.load(pipe.StreamHTTP(lc.ID, lc.Options...))
	case "subscription":
		if lc.Next == nil {
			return fmt.Errorf("non-terminated subscription %v", lc.ID)
		}

		i, err := lc.loadSymbol()

		if err != nil {
			return err
		}

		x, ok := i.(func() Subscription)

		if !ok {
			return fmt.Errorf("invalid symbol expected func() Subscription - %s - %s", lc.Type, lc.Symbol)
		}

		return lc.Next.load(pipe.StreamSubscription(lc.ID, x(), lc.Interval, lc.Options...))
	case stream:
		if lc.Next == nil {
			return fmt.Errorf("non-terminated stream %v", lc.ID)
		}

		i, err := lc.loadSymbol()

		if err != nil {
			return err
		}

		x, ok := i.(func(context.Context) chan []Data)

		if !ok {
			return fmt.Errorf("invalid symbol expected func(context.Context) chan []Data - %s - %s", lc.Type, lc.Symbol)
		}

		b := pipe.Stream(NewStream(lc.ID, x, lc.Options...))

		return lc.Next.load(b)
	default:
		return fmt.Errorf("invalid type")
	}
}

func (lc *Serialization) load(builder Builder) error {
	switch lc.Type {
	case applicative:
		return lc.applicative(builder)
	case foldLeft:
		return lc.fold(builder)
	case foldRight:
		return lc.fold(builder)
	case fork:
		return lc.fork(builder)
	case link:
		return lc.link(builder)
	case transmit:
		return lc.transmit(builder)
	default:
		return fmt.Errorf("invalid type")
	}
}

func (lc *Serialization) loadSymbol() (interface{}, error) {
	i := interp.New(interp.Options{})
	i.Use(stdlib.Symbols)
	i.Use(symbols)

	if _, err := i.Eval(lc.Script); err != nil {
		return nil, err
	}

	sym, err := i.Eval(lc.Symbol)

	if err != nil {
		return nil, err
	}

	if sym.Kind() != reflect.Func {
		return nil, fmt.Errorf("symbol is not of kind func")
	}

	return sym.Interface(), nil
}

func (lc *Serialization) applicative(b Builder) error {
	i, err := lc.loadSymbol()

	if err != nil {
		return err
	}

	x, ok := i.(func(Data) error)

	if !ok {
		return fmt.Errorf("invalid symbol expected func(Data) error - %s - %s", lc.Type, lc.Symbol)
	}

	b = b.Map(lc.ID, x, lc.Options...)

	if lc.Next == nil {
		return fmt.Errorf("non-terminated map %v", lc.ID)
	}

	return lc.Next.load(b)
}

func (lc *Serialization) fold(b Builder) error {
	i, err := lc.loadSymbol()

	if err != nil {
		return err
	}

	x, ok := i.(func(Data, Data) Data)

	if !ok {
		return fmt.Errorf("invalid symbol expected func(Data, Data) Data - %s - %s", lc.Type, lc.Symbol)
	}

	var folder func(string, Fold, ...*Option) Builder
	if lc.Type == foldLeft {
		folder = b.FoldLeft
	} else {
		folder = b.FoldRight
	}

	b = folder(lc.ID, x, lc.Options...)

	if lc.Next == nil {
		return fmt.Errorf("non-terminated fold %v", lc.ID)
	}

	return lc.Next.load(b)
}

func (lc *Serialization) fork(b Builder) error {
	var x Fork
	if lc.Symbol == "error" {
		x = ForkError
	} else if lc.Symbol == "duplicate" {
		x = ForkDuplicate
	} else {
		i, err := lc.loadSymbol()

		if err != nil {
			return err
		}

		rule, ok := i.(func(Data) bool)

		if !ok {
			return fmt.Errorf("invalid symbol expected func(Data) bool - %s - %s", lc.Type, lc.Symbol)
		}

		x = ForkRule(rule).Handler
	}

	b1, b2 := b.Fork(lc.ID, x, lc.Options...)

	if err := lc.Left.load(b1); err != nil {
		return err
	}
	return lc.Right.load(b2)
}

func (lc *Serialization) link(b Builder) error {
	b.Link(lc.ID, lc.To, lc.Options...)

	return nil
}

func (lc *Serialization) transmit(b Builder) error {
	i, err := lc.loadSymbol()

	if err != nil {
		return err
	}

	x, ok := i.(func([]Data) error)

	if !ok {
		return fmt.Errorf("invalid symbol expected func([]Data) error - %s - %s", lc.Type, lc.Symbol)
	}

	b.Transmit(lc.ID, x, lc.Options...)

	return nil
}
