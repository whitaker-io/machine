package machine

import (
	"fmt"
	"plugin"
	"reflect"
	"time"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
)

const streamConst = "stream"

var subscriptionProviders map[string]func(map[string]interface{}) Subscription
var retrieverProviders map[string]func(map[string]interface{}) Retriever
var applicativeProviders map[string]func(map[string]interface{}) Applicative
var foldProviders map[string]func(map[string]interface{}) Fold
var forkProviders map[string]func(map[string]interface{}) Fork
var transmitProviders map[string]func(map[string]interface{}) Sender

// ProviderDefinitions type used for holding provider configuration
type ProviderDefinitions struct {
	Plugins map[string]*PluginDefinition `json:"plugins,omitempty" mapstructure:"plugins,omitempty"`
	Scripts map[string]*YaegiDefinition  `json:"scripts,omitempty" mapstructure:"scripts,omitempty"`
}

// PluginDefinition type for declaring the path and symbol for a golang plugin containing the Provider
type PluginDefinition struct {
	Path   string `json:"path,omitempty" mapstructure:"path,omitempty"`
	Symbol string `json:"symbol,omitempty" mapstructure:"symbol,omitempty"`
}

// YaegiDefinition type for declaring the script and symbol for a Yaegi script containing the Provider
type YaegiDefinition struct {
	Script string `json:"script,omitempty" mapstructure:"script,omitempty"`
	Symbol string `json:"symbol,omitempty" mapstructure:"symbol,omitempty"`
}

// StreamSerialization config based definition for a stream
type StreamSerialization struct {
	// Type type of stream to create.
	//
	// For root serializations valid values are 'http', 'subscription', or 'stream'.
	Type string `json:"type,omitempty" mapstructure:"type,omitempty"`
	// Interval is the duration between pulls in a 'subscription' Type. It is only read
	// if the Type is 'subscription'.
	Interval time.Duration `json:"interval,omitempty" mapstructure:"interval,omitempty"`

	*VertexSerialization
}

// VertexSerialization config based definition for a stream vertex
type VertexSerialization struct {
	// ID unique identifier for the stream.
	ID string `json:"id,omitempty" mapstructure:"id,omitempty"`
	// Provider name of the registered vertex provider
	Provider string `json:"provider,omitempty" mapstructure:"provider,omitempty"`
	// Options are a slice of machine.Option https://godoc.org/github.com/whitaker-io/machine#Option
	Options []*Option `json:"options,omitempty" mapstructure:"options,omitempty"`
	// Attributes are a map[string]interface{} of properties to be used with the provider to create the vertex
	Attributes map[string]interface{} `json:"attributes,omitempty" mapstructure:"attributes,omitempty"`

	next map[string]*VertexSerialization
}

// Load method loads a stream based on github.com/traefik/yaegi
func (pipe *Pipe) Load(streams []StreamSerialization) error {
	for _, stream := range streams {
		switch stream.Type {
		case "http":
			if stream.VertexSerialization == nil {
				return fmt.Errorf("http stream missing retriever config")
			}
			return stream.VertexSerialization.load(pipe.StreamHTTP(stream.ID, stream.Options...))
		case "subscription":
			if stream.VertexSerialization == nil {
				return fmt.Errorf("non-terminated subscription")
			}

			return stream.VertexSerialization.load(pipe.StreamSubscription(
				stream.ID,
				subscriptionProviders[stream.Provider](stream.VertexSerialization.Attributes),
				stream.Interval,
				stream.Options...))
		case streamConst:
			if stream.VertexSerialization == nil {
				return fmt.Errorf("non-terminated subscription")
			}

			b := pipe.Stream(NewStream(stream.ID, retrieverProviders[stream.Provider](stream.VertexSerialization.Attributes), stream.Options...))

			return stream.VertexSerialization.load(b)
		default:
			return fmt.Errorf("invalid type")
		}
	}

	return nil
}

func (vs *VertexSerialization) load(builder Builder) error {
	if next, ok := vs.next["map"]; ok {
		return next.load(builder.Map(next.ID, applicativeProviders[next.Provider](vs.Attributes), next.Options...))
	} else if next, ok := vs.next["fold_left"]; ok {
		return next.load(builder.FoldLeft(next.ID, foldProviders[next.Provider](vs.Attributes), next.Options...))
	} else if next, ok := vs.next["fold_right"]; ok {
		return next.load(builder.FoldRight(next.ID, foldProviders[next.Provider](vs.Attributes), next.Options...))
	} else if next, ok := vs.next["fork"]; ok {
		var left, right *VertexSerialization
		if left, ok = next.next["left"]; !ok {
			return fmt.Errorf("missing left side of fork %s", next.ID)
		} else if right, ok = next.next["right"]; !ok {
			return fmt.Errorf("missing right side of fork %s", next.ID)
		}

		leftBuilder, rightBuilder := builder.Fork(next.ID, forkProviders[next.Provider](vs.Attributes), next.Options...)

		if err := left.load(leftBuilder); err != nil {
			return err
		}

		return right.load(rightBuilder)
	} else if next, ok := vs.next["loop"]; ok {
		var left, right *VertexSerialization
		if left, ok = next.next["in"]; !ok {
			return fmt.Errorf("missing inside of loop %s", next.ID)
		} else if right, ok = next.next["out"]; !ok {
			return fmt.Errorf("missing outside of loop %s", next.ID)
		}

		leftBuilder, rightBuilder := builder.Loop(next.ID, forkProviders[next.Provider](vs.Attributes), next.Options...)

		if err := left.loadLoop(leftBuilder); err != nil {
			return err
		}

		return right.load(rightBuilder)
	} else if next, ok := vs.next["transmit"]; ok {
		builder.Transmit(next.ID, transmitProviders[next.Provider](vs.Attributes), next.Options...)
		return nil
	}

	return fmt.Errorf("non-terminated vertex %s", vs.ID)
}

func (vs *VertexSerialization) loadLoop(builder LoopBuilder) error {
	if next, ok := vs.next["map"]; ok {
		return next.loadLoop(builder.Map(next.ID, applicativeProviders[next.Provider](vs.Attributes), next.Options...))
	} else if next, ok := vs.next["fold_left"]; ok {
		return next.loadLoop(builder.FoldLeft(next.ID, foldProviders[next.Provider](vs.Attributes), next.Options...))
	} else if next, ok := vs.next["fold_right"]; ok {
		return next.loadLoop(builder.FoldRight(next.ID, foldProviders[next.Provider](vs.Attributes), next.Options...))
	} else if next, ok := vs.next["fork"]; ok {
		var left, right *VertexSerialization
		if left, ok = next.next["left"]; !ok {
			return fmt.Errorf("missing left side of fork %s", next.ID)
		} else if right, ok = next.next["right"]; !ok {
			return fmt.Errorf("missing right side of fork %s", next.ID)
		}

		leftBuilder, rightBuilder := builder.Fork(next.ID, forkProviders[next.Provider](vs.Attributes), next.Options...)

		if err := left.loadLoop(leftBuilder); err != nil {
			return err
		}

		return right.loadLoop(rightBuilder)
	} else if next, ok := vs.next["loop"]; ok {
		var left, right *VertexSerialization
		if left, ok = next.next["in"]; !ok {
			return fmt.Errorf("missing inside of loop %s", next.ID)
		} else if right, ok = next.next["out"]; !ok {
			return fmt.Errorf("missing outside of loop %s", next.ID)
		}

		leftBuilder, rightBuilder := builder.Loop(next.ID, forkProviders[next.Provider](vs.Attributes), next.Options...)

		if err := left.loadLoop(leftBuilder); err != nil {
			return err
		}

		return right.loadLoop(rightBuilder)
	} else if next, ok := vs.next["transmit"]; ok {
		builder.Transmit(next.ID, transmitProviders[next.Provider](vs.Attributes), next.Options...)
		return nil
	} else {
		builder.Done()
	}

	return nil
}

// Load is a function to load all of the Providers into memory
func (pd *ProviderDefinitions) Load() error {
	var symbols map[string]interface{}
	var err error

	if symbols, err = pd.load(); err != nil {
		return err
	}

	for k, v := range symbols {
		switch x := v.(type) {
		case func(map[string]interface{}) Subscription:
			subscriptionProviders[k] = x
		case func(map[string]interface{}) Retriever:
			retrieverProviders[k] = x
		case func(map[string]interface{}) Applicative:
			applicativeProviders[k] = x
		case func(map[string]interface{}) Fold:
			foldProviders[k] = x
		case func(map[string]interface{}) Fork:
			forkProviders[k] = x
		case func(map[string]interface{}) Sender:
			transmitProviders[k] = x
		default:
			return fmt.Errorf("unknown provider type for key %s", k)
		}
	}

	return nil
}

func (pd *ProviderDefinitions) load() (map[string]interface{}, error) {
	symbols := map[string]interface{}{}
	for name, def := range pd.Plugins {
		sym, err := def.load()
		if err != nil {
			return nil, err
		}
		symbols[name] = sym
	}

	for name, def := range pd.Scripts {
		sym, err := def.load()
		if err != nil {
			return nil, err
		}
		symbols[name] = sym
	}

	return symbols, nil
}

func (yd *YaegiDefinition) load() (interface{}, error) {
	i := interp.New(interp.Options{})
	i.Use(stdlib.Symbols)
	i.Use(symbols)

	if _, err := i.Eval(yd.Script); err != nil {
		return nil, err
	}

	sym, err := i.Eval(yd.Symbol)

	if err != nil {
		return nil, err
	}

	if sym.Kind() != reflect.Func {
		return nil, fmt.Errorf("symbol is not of kind func")
	}

	return sym.Interface(), nil
}

func (pd *PluginDefinition) load() (interface{}, error) {
	p, err := plugin.Open(pd.Path)

	if err != nil {
		return nil, err
	}

	sym, err := p.Lookup(pd.Symbol)

	if err != nil {
		return nil, err
	}

	return sym, nil
}
