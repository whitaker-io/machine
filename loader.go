package machine

import (
	"fmt"
	"reflect"
	"time"

	"github.com/mitchellh/mapstructure"
	"gopkg.in/yaml.v3"
)

const (
	streamConst       = "stream"
	subscriptionConst = "subscription"
	httpConst         = "http"
)

var (
	pluginProviders = map[string]PluginProvider{}
)

// PluginProvider interface for providing a way of loading plugins
// must return one of the following types:
//
//  Subscription
//  Retriever
//  Applicative
//  Fold
//  Fork
//  Publisher
type PluginProvider interface {
	Load(*PluginDefinition) (interface{}, error)
}

// PluginDefinition type for declaring the path and symbol for a golang plugin containing the Provider
type PluginDefinition struct {
	// Type is the name of the PluginProvider to use.
	Type string `mapstructure:"type,omitempty"`
	// Payload is the location, script, etc provided to load the plugin.
	// Depends on the PluginProvider.
	Payload string `mapstructure:"payload,omitempty"`
	// Symbol is the name of the symbol to be loaded from the plugin.
	Symbol string `mapstructure:"symbol,omitempty"`
	// Attributes are a map[string]interface{} of properties to be used with the PluginProvider.
	Attributes map[string]interface{} `mapstructure:"attributes,omitempty"`
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
	// Provider Plugin information to load
	Provider PluginDefinition `mapstructure:"provider,omitempty"`
	// Options are a slice of machine.Option https://godoc.org/github.com/whitaker-io/machine#Option
	Options []*Option `mapstructure:"options,omitempty"`

	next map[string]*VertexSerialization
}

// RegisterPluginProvider function for registering a PluginProvider
// to be used for loading VertexProviders
func RegisterPluginProvider(name string, p PluginProvider) {
	pluginProviders[name] = p
}

// Load method loads a stream based on the StreamSerialization
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

			if err := stream.VertexSerialization.load(
				pipe.StreamSubscription(stream.ID, stream.VertexSerialization.subscription(), stream.Interval, stream.Options...),
			); err != nil {
				return err
			}
		case streamConst:
			if stream.VertexSerialization == nil {
				return fmt.Errorf("non-terminated stream")
			}

			b := pipe.Stream(NewStream(stream.ID, stream.VertexSerialization.retriever(), stream.Options...))

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
		return next.load(builder.Map(next.ID, next.applicative(), next.Options...))
	} else if next, ok := vs.next["fold_left"]; ok {
		return next.load(builder.FoldLeft(next.ID, next.fold(), next.Options...))
	} else if next, ok := vs.next["fold_right"]; ok {
		return next.load(builder.FoldRight(next.ID, next.fold(), next.Options...))
	} else if next, ok := vs.next["fork"]; ok {
		leftBuilder, rightBuilder := builder.Fork(next.ID, next.fork(), next.Options...)

		var left, right *VertexSerialization
		var ok bool

		if left, ok = next.next["left"]; !ok {
			return fmt.Errorf("missing left side of fork %s", vs.ID)
		} else if right, ok = next.next["right"]; !ok {
			return fmt.Errorf("missing right side of fork %s", vs.ID)
		} else if err := left.load(leftBuilder); err != nil {
			return err
		}

		return right.load(rightBuilder)
	} else if next, ok := vs.next["loop"]; ok {
		leftBuilder, rightBuilder := builder.Loop(next.ID, next.fork(), next.Options...)

		var left, right *VertexSerialization
		var ok bool

		if left, ok = next.next["in"]; !ok {
			return fmt.Errorf("missing inner side of loop %s", vs.ID)
		} else if right, ok = next.next["out"]; !ok {
			return fmt.Errorf("missing outer side of loop %s", vs.ID)
		} else if err := left.loadLoop(leftBuilder); err != nil {
			return err
		}

		return right.load(rightBuilder)
	} else if next, ok := vs.next["publish"]; ok {
		builder.Publish(next.ID, next.publish(), next.Options...)
		return nil
	}

	return fmt.Errorf("non-terminated vertex %s", vs.ID)
}

func (vs *VertexSerialization) loadLoop(builder LoopBuilder) error {
	if next, ok := vs.next["map"]; ok {
		return next.loadLoop(builder.Map(next.ID, next.applicative(), next.Options...))
	} else if next, ok := vs.next["fold_left"]; ok {
		return next.loadLoop(builder.FoldLeft(next.ID, next.fold(), next.Options...))
	} else if next, ok := vs.next["fold_right"]; ok {
		return next.loadLoop(builder.FoldRight(next.ID, next.fold(), next.Options...))
	} else if next, ok := vs.next["fork"]; ok {
		leftBuilder, rightBuilder := builder.Fork(next.ID, next.fork(), next.Options...)

		var left, right *VertexSerialization
		var ok bool

		if left, ok = next.next["left"]; !ok {
			return fmt.Errorf("missing left side of fork %s", vs.ID)
		} else if right, ok = next.next["right"]; !ok {
			return fmt.Errorf("missing right side of fork %s", vs.ID)
		} else if err := left.loadLoop(leftBuilder); err != nil {
			return err
		}

		return right.loadLoop(rightBuilder)
	} else if next, ok := vs.next["loop"]; ok {
		leftBuilder, rightBuilder := builder.Loop(next.ID, next.fork(), next.Options...)

		var left, right *VertexSerialization
		var ok bool

		if left, ok = next.next["in"]; !ok {
			return fmt.Errorf("missing inner side of loop %s", vs.ID)
		} else if right, ok = next.next["out"]; !ok {
			return fmt.Errorf("missing outer side of loop %s", vs.ID)
		} else if err := left.loadLoop(leftBuilder); err != nil {
			return err
		}

		return right.loadLoop(rightBuilder)
	} else if next, ok := vs.next["publish"]; ok {
		builder.Publish(next.ID, next.publish(), next.Options...)
	} else {
		builder.Done()
	}

	return nil
}

func (vs *VertexSerialization) subscription() Subscription {
	if sym, err := vs.Provider.load(); err != nil {
		panic(err)
	} else if x, ok := sym.(Subscription); ok {
		return x
	}

	panic(fmt.Errorf("invalid plugin type not subscription"))
}

func (vs *VertexSerialization) retriever() Retriever {
	if sym, err := vs.Provider.load(); err != nil {
		panic(err)
	} else if x, ok := sym.(Retriever); ok {
		return x
	}

	panic(fmt.Errorf("invalid plugin type not retriever"))
}

func (vs *VertexSerialization) applicative() Applicative {
	if sym, err := vs.Provider.load(); err != nil {
		panic(err)
	} else if x, ok := sym.(Applicative); ok {
		return x
	}

	panic(fmt.Errorf("invalid plugin type not applicative"))
}

func (vs *VertexSerialization) fold() Fold {
	if sym, err := vs.Provider.load(); err != nil {
		panic(err)
	} else if x, ok := sym.(Fold); ok {
		return x
	}

	panic(fmt.Errorf("invalid plugin type not fold"))
}

func (vs *VertexSerialization) fork() Fork {
	if sym, err := vs.Provider.load(); err != nil {
		panic(err)
	} else if x, ok := sym.(Fork); ok {
		return x
	}

	panic(fmt.Errorf("invalid plugin type not fork"))
}

func (vs *VertexSerialization) publish() Publisher {
	if sym, err := vs.Provider.load(); err != nil {
		panic(err)
	} else if x, ok := sym.(Publisher); ok {
		return x
	}

	panic(fmt.Errorf("invalid plugin type not publisher"))
}

// Load is a function to load all of the Providers into memory
func (def *PluginDefinition) load() (interface{}, error) {
	if provider, ok := pluginProviders[def.Type]; ok {
		return provider.Load(def)
	}
	return nil, fmt.Errorf("missing PluginProvider %s", def.Type)
}

// MarshalYAML implementation to marshal yaml
func (s *StreamSerialization) MarshalYAML() (interface{}, error) {
	m := map[string]interface{}{}

	s.toMap(m)

	return yaml.Marshal(m)
}

// UnmarshalYAML implementation to unmarshal yaml
func (s *StreamSerialization) UnmarshalYAML(unmarshal func(interface{}) error) error {
	m := map[string]interface{}{}

	if err := unmarshal(&m); err != nil {
		return err
	}

	return s.fromMap(m)
}

func (s *StreamSerialization) toMap(m map[string]interface{}) {
	m["id"] = s.ID
	m["type"] = s.Type
	m["interval"] = int64(s.Interval)
	m["provider"] = s.Provider
	m["options"] = s.Options

	for k, v := range s.next {
		out := map[string]interface{}{}
		v.toMap(out)
		m[k] = out
	}
}

func (s *StreamSerialization) fromMap(m map[string]interface{}) error {
	s.VertexSerialization = &VertexSerialization{}

	if t, ok := m["type"]; ok {
		s.Type = t.(string)
	} else {
		return fmt.Errorf("missing type field")
	}

	if interval, ok := m["interval"]; ok {
		switch val := interval.(type) {
		case int64:
			s.Interval = time.Duration(val)
		case int:
			s.Interval = time.Duration(val)
		case string:
		default:
			return fmt.Errorf("invalid interval type expecting int or int64 for %s found %v", s.ID, reflect.TypeOf(val))
		}
	} else if s.Type == subscriptionConst {
		return fmt.Errorf("missing interval field")
	}

	delete(m, "type")
	delete(m, "interval")

	s.VertexSerialization.fromMap(m)

	return nil
}

func (vs *VertexSerialization) toMap(m map[string]interface{}) {
	m["id"] = vs.ID
	m["provider"] = vs.Provider
	m["options"] = vs.Options

	for k, v := range vs.next {
		out := map[string]interface{}{}
		v.toMap(out)
		m[k] = out
	}
}

func (vs *VertexSerialization) fromMap(m map[string]interface{}) {
	if id, ok := m["id"]; ok {
		vs.ID = id.(string)
	}

	if provider, ok := m["provider"]; ok {
		vs.Provider = PluginDefinition{}
		if err := mapstructure.Decode(provider, &vs.Provider); err != nil {
			panic(err)
		}
	}

	if options, ok := m["options"]; ok {
		vs.Options = []*Option{}
		if err := mapstructure.Decode(options, &vs.Options); err != nil {
			panic(err)
		}
	}

	vs.next = map[string]*VertexSerialization{}

	for k, v := range m {
		if x, ok := v.(map[string]interface{}); ok {
			vs2 := &VertexSerialization{
				Options: []*Option{},
			}

			vs2.fromMap(x)
			vs.next[k] = vs2
		}
	}
}
