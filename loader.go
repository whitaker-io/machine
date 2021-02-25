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
	SubscriptionPlugins map[string]*PluginDefinition `json:"subscription_plugins,omitempty" mapstructure:"subscription_plugins,omitempty"`
	SubscriptionScripts map[string]*YaegiDefinition  `json:"subscription_scripts,omitempty" mapstructure:"subscription_scripts,omitempty"`
	RetrieverPlugins    map[string]*PluginDefinition `json:"retriever_plugins,omitempty" mapstructure:"retriever_plugins,omitempty"`
	RetrieverScripts    map[string]*YaegiDefinition  `json:"retriever_scripts,omitempty" mapstructure:"retriever_scripts,omitempty"`
	ApplicativePlugins  map[string]*PluginDefinition `json:"applicative_plugins,omitempty" mapstructure:"applicative_plugins,omitempty"`
	ApplicativeScripts  map[string]*YaegiDefinition  `json:"applicative_scripts,omitempty" mapstructure:"applicative_scripts,omitempty"`
	FoldPlugins         map[string]*PluginDefinition `json:"fold_plugins,omitempty" mapstructure:"fold_plugins,omitempty"`
	FoldScripts         map[string]*YaegiDefinition  `json:"fold_scripts,omitempty" mapstructure:"fold_scripts,omitempty"`
	ForkPlugins         map[string]*PluginDefinition `json:"fork_plugins,omitempty" mapstructure:"fork_plugins,omitempty"`
	ForkScripts         map[string]*YaegiDefinition  `json:"fork_scripts,omitempty" mapstructure:"fork_scripts,omitempty"`
	TransmitPlugins     map[string]*PluginDefinition `json:"transmit_plugins,omitempty" mapstructure:"transmit_plugins,omitempty"`
	TransmitScripts     map[string]*YaegiDefinition  `json:"transmit_scripts,omitempty" mapstructure:"transmit_scripts,omitempty"`
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
	if err := pd.loadSubscriptions(); err != nil {
		return err
	}

	if err := pd.loadRetrievers(); err != nil {
		return err
	}

	if err := pd.loadApplicatives(); err != nil {
		return err
	}

	if err := pd.loadFolds(); err != nil {
		return err
	}

	if err := pd.loadForks(); err != nil {
		return err
	}

	if err := pd.loadSenders(); err != nil {
		return err
	}

	return nil
}

func (pd *ProviderDefinitions) loadSubscriptions() error {
	for name, def := range pd.SubscriptionPlugins {
		sym, err := def.load()
		if err != nil {
			return err
		}
		subscriptionProviders[name] = sym.(func(map[string]interface{}) Subscription)
	}

	for name, def := range pd.SubscriptionScripts {
		sym, err := def.load()
		if err != nil {
			return err
		}
		subscriptionProviders[name] = sym.(func(map[string]interface{}) Subscription)
	}

	return nil
}

func (pd *ProviderDefinitions) loadRetrievers() error {
	for name, def := range pd.RetrieverPlugins {
		sym, err := def.load()
		if err != nil {
			return err
		}
		retrieverProviders[name] = sym.(func(map[string]interface{}) Retriever)
	}

	for name, def := range pd.RetrieverScripts {
		sym, err := def.load()
		if err != nil {
			return err
		}
		retrieverProviders[name] = sym.(func(map[string]interface{}) Retriever)
	}

	return nil
}

func (pd *ProviderDefinitions) loadApplicatives() error {
	for name, def := range pd.ApplicativePlugins {
		sym, err := def.load()
		if err != nil {
			return err
		}
		applicativeProviders[name] = sym.(func(map[string]interface{}) Applicative)
	}

	for name, def := range pd.ApplicativeScripts {
		sym, err := def.load()
		if err != nil {
			return err
		}
		applicativeProviders[name] = sym.(func(map[string]interface{}) Applicative)
	}

	return nil
}

func (pd *ProviderDefinitions) loadFolds() error {
	for name, def := range pd.FoldPlugins {
		sym, err := def.load()
		if err != nil {
			return err
		}
		foldProviders[name] = sym.(func(map[string]interface{}) Fold)
	}

	for name, def := range pd.FoldScripts {
		sym, err := def.load()
		if err != nil {
			return err
		}
		foldProviders[name] = sym.(func(map[string]interface{}) Fold)
	}

	return nil
}

func (pd *ProviderDefinitions) loadForks() error {
	for name, def := range pd.ForkPlugins {
		sym, err := def.load()
		if err != nil {
			return err
		}
		forkProviders[name] = sym.(func(map[string]interface{}) Fork)
	}

	for name, def := range pd.ForkScripts {
		sym, err := def.load()
		if err != nil {
			return err
		}
		forkProviders[name] = sym.(func(map[string]interface{}) Fork)
	}

	return nil
}

func (pd *ProviderDefinitions) loadSenders() error {
	for name, def := range pd.TransmitPlugins {
		sym, err := def.load()
		if err != nil {
			return err
		}
		transmitProviders[name] = sym.(func(map[string]interface{}) Sender)
	}

	for name, def := range pd.TransmitScripts {
		sym, err := def.load()
		if err != nil {
			return err
		}
		transmitProviders[name] = sym.(func(map[string]interface{}) Sender)
	}

	return nil
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
