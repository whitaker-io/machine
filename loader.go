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

var subscriptionProviders map[string]func(map[string]interface{}) Subscription = map[string]func(map[string]interface{}) Subscription{}
var retrieverProviders map[string]func(map[string]interface{}) Retriever = map[string]func(map[string]interface{}) Retriever{}
var applicativeProviders map[string]func(map[string]interface{}) Applicative = map[string]func(map[string]interface{}) Applicative{}
var foldProviders map[string]func(map[string]interface{}) Fold = map[string]func(map[string]interface{}) Fold{}
var forkProviders map[string]func(map[string]interface{}) Fork = map[string]func(map[string]interface{}) Fork{}
var transmitProviders map[string]func(map[string]interface{}) Sender = map[string]func(map[string]interface{}) Sender{}
var pluginProviders map[string]PluginProvider = map[string]PluginProvider{}

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
	Type    string `json:"type,omitempty" mapstructure:"type,omitempty"`
	Payload string `json:"payload,omitempty" mapstructure:"payload,omitempty"`
	Symbol  string `json:"symbol,omitempty" mapstructure:"symbol,omitempty"`
}

// StreamSerialization config based definition for a stream
type StreamSerialization struct {
	// Type type of stream to create.
	//
	// For root serializations valid values are 'http', 'subscription', or 'stream'.
	Type string `json:"type,omitempty" mapstructure:"type,omitempty"`
	// Interval is the duration in nanoseconds between pulls in a 'subscription' Type. It is only read
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

// RegisterPluginProvider function for registering a PluginProvider
// to be used for loading VertexProviders
func RegisterPluginProvider(name string, p PluginProvider) {
	pluginProviders[name] = p
}

// Load method loads a stream based on github.com/traefik/yaegi
func (pipe *Pipe) Load(streams []StreamSerialization) error {
	for _, stream := range streams {
		switch stream.Type {
		case httpConst:
			if stream.VertexSerialization == nil {
				return fmt.Errorf("http stream missing retriever config")
			}
			return stream.VertexSerialization.load(pipe.StreamHTTP(stream.ID, stream.Options...))
		case subscriptionConst:
			if stream.VertexSerialization == nil {
				return fmt.Errorf("non-terminated subscription")
			}

			if _, ok := subscriptionProviders[stream.Provider]; !ok {
				return fmt.Errorf("missing subscription Provider %s", stream.Provider)
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

			if _, ok := retrieverProviders[stream.Provider]; !ok {
				return fmt.Errorf("missing retriever Provider %s", stream.Provider)
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
		if _, ok := applicativeProviders[next.Provider]; !ok {
			return fmt.Errorf("missing applicative Provider %s", next.Provider)
		}

		return next.load(builder.Map(next.ID, applicativeProviders[next.Provider](next.Attributes), next.Options...))
	} else if next, ok := vs.next["fold_left"]; ok {
		if _, ok := foldProviders[next.Provider]; !ok {
			return fmt.Errorf("missing fold Provider %s", next.Provider)
		}

		return next.load(builder.FoldLeft(next.ID, foldProviders[next.Provider](next.Attributes), next.Options...))
	} else if next, ok := vs.next["fold_right"]; ok {
		if _, ok := foldProviders[next.Provider]; !ok {
			return fmt.Errorf("missing fold Provider %s", next.Provider)
		}

		return next.load(builder.FoldRight(next.ID, foldProviders[next.Provider](next.Attributes), next.Options...))
	} else if next, ok := vs.next["fork"]; ok {
		if _, xok := forkProviders[next.Provider]; !xok {
			return fmt.Errorf("missing fork Provider %s", next.Provider)
		}

		var left, right *VertexSerialization
		if left, ok = next.next["left"]; !ok {
			return fmt.Errorf("missing left side of fork %s", next.ID)
		} else if right, ok = next.next["right"]; !ok {
			return fmt.Errorf("missing right side of fork %s", next.ID)
		}

		leftBuilder, rightBuilder := builder.Fork(next.ID, forkProviders[next.Provider](next.Attributes), next.Options...)

		if err := left.load(leftBuilder); err != nil {
			return err
		}

		return right.load(rightBuilder)
	} else if next, ok := vs.next["loop"]; ok {
		if _, xok := forkProviders[next.Provider]; !xok {
			return fmt.Errorf("missing loop fork Provider %s", next.Provider)
		}

		var left, right *VertexSerialization
		if left, ok = next.next["in"]; !ok {
			return fmt.Errorf("missing inside of loop %s", next.ID)
		} else if right, ok = next.next["out"]; !ok {
			return fmt.Errorf("missing outside of loop %s", next.ID)
		}

		leftBuilder, rightBuilder := builder.Loop(next.ID, forkProviders[next.Provider](next.Attributes), next.Options...)

		if err := left.loadLoop(leftBuilder); err != nil {
			return err
		}

		return right.load(rightBuilder)
	} else if next, ok := vs.next["transmit"]; ok {
		if _, ok := transmitProviders[next.Provider]; !ok {
			return fmt.Errorf("missing sender Provider %s", next.Provider)
		}

		builder.Transmit(next.ID, transmitProviders[next.Provider](next.Attributes), next.Options...)
		return nil
	}

	return fmt.Errorf("non-terminated vertex %s", vs.ID)
}

func (vs *VertexSerialization) loadLoop(builder LoopBuilder) error {
	if next, ok := vs.next["map"]; ok {
		if _, ok := applicativeProviders[next.Provider]; !ok {
			return fmt.Errorf("missing applicative Provider %s", next.Provider)
		}

		return next.loadLoop(builder.Map(next.ID, applicativeProviders[next.Provider](next.Attributes), next.Options...))
	} else if next, ok := vs.next["fold_left"]; ok {
		if _, ok := foldProviders[next.Provider]; !ok {
			return fmt.Errorf("missing applicative Provider %s", next.Provider)
		}

		return next.loadLoop(builder.FoldLeft(next.ID, foldProviders[next.Provider](next.Attributes), next.Options...))
	} else if next, ok := vs.next["fold_right"]; ok {
		if _, ok := foldProviders[next.Provider]; !ok {
			return fmt.Errorf("missing applicative Provider %s", next.Provider)
		}

		return next.loadLoop(builder.FoldRight(next.ID, foldProviders[next.Provider](next.Attributes), next.Options...))
	} else if next, ok := vs.next["fork"]; ok {
		if _, xok := forkProviders[next.Provider]; !xok {
			return fmt.Errorf("missing applicative Provider %s", next.Provider)
		}

		var left, right *VertexSerialization
		if left, ok = next.next["left"]; !ok {
			return fmt.Errorf("missing left side of fork %s", next.ID)
		} else if right, ok = next.next["right"]; !ok {
			return fmt.Errorf("missing right side of fork %s", next.ID)
		}

		leftBuilder, rightBuilder := builder.Fork(next.ID, forkProviders[next.Provider](next.Attributes), next.Options...)

		if err := left.loadLoop(leftBuilder); err != nil {
			return err
		}

		return right.loadLoop(rightBuilder)
	} else if next, ok := vs.next["loop"]; ok {
		if _, xok := forkProviders[next.Provider]; !xok {
			return fmt.Errorf("missing loop fork Provider %s", next.Provider)
		}

		var left, right *VertexSerialization
		if left, ok = next.next["in"]; !ok {
			return fmt.Errorf("missing inside of loop %s", next.ID)
		} else if right, ok = next.next["out"]; !ok {
			return fmt.Errorf("missing outside of loop %s", next.ID)
		}

		leftBuilder, rightBuilder := builder.Loop(next.ID, forkProviders[next.Provider](next.Attributes), next.Options...)

		if err := left.loadLoop(leftBuilder); err != nil {
			return err
		}

		return right.loadLoop(rightBuilder)
	} else if next, ok := vs.next["transmit"]; ok {
		if _, ok := transmitProviders[next.Provider]; !ok {
			return fmt.Errorf("missing sender Provider %s", next.Provider)
		}

		builder.Transmit(next.ID, transmitProviders[next.Provider](next.Attributes), next.Options...)
		return nil
	} else {
		builder.Done()
	}

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
