package machine

import (
	"fmt"
	"time"
)

const (
	streamConst       = "stream"
	subscriptionConst = "subscription"
	httpConst         = "http"
)

var subscriptionProviders = map[string]func(map[string]interface{}) Subscription{}
var retrieverProviders = map[string]func(map[string]interface{}) Retriever{}
var applicativeProviders = map[string]func(map[string]interface{}) Applicative{}
var foldProviders = map[string]func(map[string]interface{}) Fold{}
var forkProviders = map[string]func(map[string]interface{}) Fork{}
var transmitProviders = map[string]func(map[string]interface{}) Sender{}
var pluginProviders = map[string]PluginProvider{}

// PluginProvider interface for providing a way of loading plugins
// must return one of the following functions:
//
// func(map[string]interface{}) Subscription
// func(map[string]interface{}) Retriever
// func(map[string]interface{}) Applicative
// func(map[string]interface{}) Fold
// func(map[string]interface{}) Fork
// func(map[string]interface{}) Sender
type PluginProvider interface {
	Load(*PluginDefinition) (interface{}, error)
}

// ProviderDefinitions type used for holding provider configuration
type ProviderDefinitions struct {
	Plugins map[string]*PluginDefinition `json:"plugins,omitempty" mapstructure:"plugins,omitempty"`
}

// PluginDefinition type for declaring the path and symbol for a golang plugin containing the Provider
type PluginDefinition struct {
	Type    string `mapstructure:"type,omitempty"`
	Payload string `mapstructure:"payload,omitempty"`
	Symbol  string `mapstructure:"symbol,omitempty"`
}

// StreamSerialization config based definition for a stream
type StreamSerialization struct {
	// Type type of stream to create.
	//
	// For root serializations valid values are 'http', 'subscription', or 'stream'.
	Type string `mapstructure:"type,omitempty"`
	// Interval is the duration in nanoseconds between pulls in a 'subscription' Type. It is only read
	// if the Type is 'subscription'.
	Interval time.Duration `mapstructure:"interval,omitempty"`

	*VertexSerialization
}

// VertexSerialization config based definition for a stream vertex
type VertexSerialization struct {
	// ID unique identifier for the stream.
	ID string `mapstructure:"id,omitempty"`
	// Provider name of the registered vertex provider
	Provider string `mapstructure:"provider,omitempty"`
	// Options are a slice of machine.Option https://godoc.org/github.com/whitaker-io/machine#Option
	Options []*Option `mapstructure:"options,omitempty"`
	// Attributes are a map[string]interface{} of properties to be used with the provider to create the vertex
	Attributes map[string]interface{} `mapstructure:"attributes,omitempty"`

	next map[string]*VertexSerialization
}

// RegisterPluginProvider function for registering a PluginProvider
// to be used for loading VertexProviders
func RegisterPluginProvider(name string, p PluginProvider) {
	pluginProviders[name] = p
}

// Load method loads a stream based on github.com/traefik/yaegi
func (pipe *Pipe) Load(streams []*StreamSerialization) error {
	for _, stream := range streams {
		switch stream.Type {
		case httpConst:
			if stream.VertexSerialization == nil {
				return fmt.Errorf("http stream missing retriever config")
			}
			if err := stream.VertexSerialization.load(pipe.StreamHTTP(stream.ID, stream.Options...)); err != nil {
				return err
			}
		case subscriptionConst:
			if stream.VertexSerialization == nil {
				return fmt.Errorf("non-terminated subscription")
			}

			if _, ok := subscriptionProviders[stream.Provider]; !ok {
				return fmt.Errorf("missing subscription Provider %s", stream.Provider)
			}

			if err := stream.VertexSerialization.load(pipe.StreamSubscription(
				stream.ID,
				subscriptionProviders[stream.Provider](stream.VertexSerialization.Attributes),
				stream.Interval,
				stream.Options...)); err != nil {
				return err
			}
		case streamConst:
			if stream.VertexSerialization == nil {
				return fmt.Errorf("non-terminated subscription")
			}

			if _, ok := retrieverProviders[stream.Provider]; !ok {
				return fmt.Errorf("missing retriever Provider %s", stream.Provider)
			}

			b := pipe.Stream(
				NewStream(
					stream.ID,
					retrieverProviders[stream.Provider](stream.VertexSerialization.Attributes),
					stream.Options...,
				),
			)

			if err := stream.VertexSerialization.load(b); err != nil {
				return err
			}
		default:
			return fmt.Errorf("invalid type")
		}
	}

	return nil
}

func (vs *VertexSerialization) load(builder Builder) error {
	if next, ok := vs.next["map"]; ok {
		return next.mapper(builder)
	} else if next, ok := vs.next["fold_left"]; ok {
		return next.fold(true, builder)
	} else if next, ok := vs.next["fold_right"]; ok {
		return next.fold(false, builder)
	} else if next, ok := vs.next["fork"]; ok {
		return next.fork(builder)
	} else if next, ok := vs.next["loop"]; ok {
		return next.loop(builder)
	} else if next, ok := vs.next["transmit"]; ok {
		return next.transmit(builder)
	}

	return fmt.Errorf("non-terminated vertex %s", vs.ID)
}

func (vs *VertexSerialization) loadLoop(builder LoopBuilder) error {
	if next, ok := vs.next["map"]; ok {
		return next.mapLoop(builder)
	} else if next, ok := vs.next["fold_left"]; ok {
		return next.foldLoop(true, builder)
	} else if next, ok := vs.next["fold_right"]; ok {
		return next.foldLoop(false, builder)
	} else if next, ok := vs.next["fork"]; ok {
		return next.forkLoop(builder)
	} else if next, ok := vs.next["loop"]; ok {
		return next.nestLoop(builder)
	} else if next, ok := vs.next["transmit"]; ok {
		return next.transmitLoop(builder)
	} else {
		builder.Done()
	}

	return nil
}

func (vs *VertexSerialization) mapper(builder Builder) error {
	if _, ok := applicativeProviders[vs.Provider]; !ok {
		return fmt.Errorf("missing applicative Provider %s", vs.Provider)
	}

	return vs.load(builder.Map(vs.ID, applicativeProviders[vs.Provider](vs.Attributes), vs.Options...))
}

func (vs *VertexSerialization) mapLoop(builder LoopBuilder) error {
	if _, ok := applicativeProviders[vs.Provider]; !ok {
		return fmt.Errorf("missing applicative Provider %s", vs.Provider)
	}

	return vs.loadLoop(builder.Map(vs.ID, applicativeProviders[vs.Provider](vs.Attributes), vs.Options...))
}

func (vs *VertexSerialization) fold(left bool, builder Builder) error {
	if _, ok := foldProviders[vs.Provider]; !ok {
		return fmt.Errorf("missing applicative Provider %s", vs.Provider)
	}

	if !left {
		return vs.load(builder.FoldRight(vs.ID, foldProviders[vs.Provider](vs.Attributes), vs.Options...))
	}

	return vs.load(builder.FoldLeft(vs.ID, foldProviders[vs.Provider](vs.Attributes), vs.Options...))
}

func (vs *VertexSerialization) foldLoop(left bool, builder LoopBuilder) error {
	if _, ok := foldProviders[vs.Provider]; !ok {
		return fmt.Errorf("missing applicative Provider %s", vs.Provider)
	}

	if !left {
		return vs.loadLoop(builder.FoldRight(vs.ID, foldProviders[vs.Provider](vs.Attributes), vs.Options...))
	}

	return vs.loadLoop(builder.FoldLeft(vs.ID, foldProviders[vs.Provider](vs.Attributes), vs.Options...))
}

func (vs *VertexSerialization) fork(builder Builder) error {
	if _, ok := forkProviders[vs.Provider]; !ok {
		return fmt.Errorf("missing applicative Provider %s", vs.Provider)
	}

	var left, right *VertexSerialization
	var ok bool
	if left, ok = vs.next["left"]; !ok {
		return fmt.Errorf("missing left side of fork %s", vs.ID)
	} else if right, ok = vs.next["right"]; !ok {
		return fmt.Errorf("missing right side of fork %s", vs.ID)
	}

	leftBuilder, rightBuilder := builder.Fork(vs.ID, forkProviders[vs.Provider](vs.Attributes), vs.Options...)

	if err := left.load(leftBuilder); err != nil {
		return err
	}

	return right.load(rightBuilder)
}

func (vs *VertexSerialization) forkLoop(builder LoopBuilder) error {
	if _, ok := forkProviders[vs.Provider]; !ok {
		return fmt.Errorf("missing applicative Provider %s", vs.Provider)
	}

	var left, right *VertexSerialization
	var ok bool
	if left, ok = vs.next["left"]; !ok {
		return fmt.Errorf("missing left side of fork %s", vs.ID)
	} else if right, ok = vs.next["right"]; !ok {
		return fmt.Errorf("missing right side of fork %s", vs.ID)
	}

	leftBuilder, rightBuilder := builder.Fork(vs.ID, forkProviders[vs.Provider](vs.Attributes), vs.Options...)

	if err := left.loadLoop(leftBuilder); err != nil {
		return err
	}

	return right.loadLoop(rightBuilder)
}

func (vs *VertexSerialization) loop(builder Builder) error {
	if _, ok := forkProviders[vs.Provider]; !ok {
		return fmt.Errorf("missing loop fork Provider %s", vs.Provider)
	}

	var left, right *VertexSerialization
	var ok bool
	if left, ok = vs.next["in"]; !ok {
		return fmt.Errorf("missing inside of loop %s", vs.ID)
	} else if right, ok = vs.next["out"]; !ok {
		return fmt.Errorf("missing outside of loop %s", vs.ID)
	}

	leftBuilder, rightBuilder := builder.Loop(vs.ID, forkProviders[vs.Provider](vs.Attributes), vs.Options...)

	if err := left.loadLoop(leftBuilder); err != nil {
		return err
	}

	return right.load(rightBuilder)
}

func (vs *VertexSerialization) nestLoop(builder LoopBuilder) error {
	if _, ok := forkProviders[vs.Provider]; !ok {
		return fmt.Errorf("missing loop fork Provider %s", vs.Provider)
	}

	var left, right *VertexSerialization
	var ok bool
	if left, ok = vs.next["in"]; !ok {
		return fmt.Errorf("missing inside of loop %s", vs.ID)
	} else if right, ok = vs.next["out"]; !ok {
		return fmt.Errorf("missing outside of loop %s", vs.ID)
	}

	leftBuilder, rightBuilder := builder.Loop(vs.ID, forkProviders[vs.Provider](vs.Attributes), vs.Options...)

	if err := left.loadLoop(leftBuilder); err != nil {
		return err
	}

	return right.loadLoop(rightBuilder)
}

func (vs *VertexSerialization) transmit(builder Builder) error {
	if _, ok := transmitProviders[vs.Provider]; !ok {
		return fmt.Errorf("missing sender Provider %s", vs.Provider)
	}

	builder.Transmit(vs.ID, transmitProviders[vs.Provider](vs.Attributes), vs.Options...)
	return nil
}

func (vs *VertexSerialization) transmitLoop(builder LoopBuilder) error {
	if _, ok := transmitProviders[vs.Provider]; !ok {
		return fmt.Errorf("missing sender Provider %s", vs.Provider)
	}

	builder.Transmit(vs.ID, transmitProviders[vs.Provider](vs.Attributes), vs.Options...)
	return nil
}

// Load is a function to load all of the Providers into memory
func (pd *ProviderDefinitions) Load() error {
	symbols := map[string]interface{}{}

	if pd.Plugins != nil {
		for name, def := range pd.Plugins {
			if provider, ok := pluginProviders[def.Type]; ok {
				sym, err := provider.Load(def)
				if err != nil {
					return err
				}
				symbols[name] = sym
			} else {
				return fmt.Errorf("missing PluginProvider %s", def.Type)
			}
		}
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
