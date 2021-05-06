package machine

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/mitchellh/mapstructure"
)

const (
	streamConst       = "stream"
	subscriptionConst = "subscription"
	httpConst         = "http"
	websocketConst    = "websocket"
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
	Type string `json:"type" mapstructure:"type"`
	// Payload is the location, script, etc provided to load the plugin.
	// Depends on the PluginProvider.
	Payload string `json:"payload" mapstructure:"payload"`
	// Symbol is the name of the symbol to be loaded from the plugin.
	Symbol string `json:"symbol" mapstructure:"symbol"`
	// Attributes are a map[string]interface{} of properties to be used with the PluginProvider.
	Attributes map[string]interface{} `json:"attributes" mapstructure:"attributes"`
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
	// Provider Plugin information to load
	Provider *PluginDefinition `json:"provider,omitempty" mapstructure:"provider,omitempty"`
	// Options are a slice of machine.Option https://godoc.org/github.com/whitaker-io/machine#Option
	Options []*Option `json:"options,omitempty" mapstructure:"options,omitempty"`

	next map[string]*VertexSerialization
}

// RegisterPluginProvider function for registering a PluginProvider
// to be used for loading VertexProviders
func RegisterPluginProvider(name string, p PluginProvider) {
	pluginProviders[name] = p
}

// Load method loads a stream based on the StreamSerialization
func Load(serialization *StreamSerialization) (Stream, error) {
	var stream Stream
	switch serialization.Type {
	case httpConst:
		if serialization.VertexSerialization == nil {
			return nil, fmt.Errorf("http stream missing config")
		}

		stream = NewHTTPStream(serialization.ID, serialization.Options...)
	case websocketConst:
		if serialization.VertexSerialization == nil {
			return nil, fmt.Errorf("websocket stream missing config")
		}

		stream = NewWebsocketStream(serialization.ID, serialization.Options...)
	case subscriptionConst:
		if serialization.VertexSerialization == nil {
			return nil, fmt.Errorf("non-terminated subscription")
		}

		stream = NewSubscriptionStream(
			serialization.ID,
			serialization.VertexSerialization.subscription(),
			serialization.Interval,
			serialization.Options...,
		)
	case streamConst:
		if serialization.VertexSerialization == nil {
			return nil, fmt.Errorf("non-terminated stream")
		}

		stream = NewStream(
			serialization.ID,
			serialization.VertexSerialization.retriever(),
			serialization.Options...,
		)
	default:
		return nil, fmt.Errorf("invalid type")
	}

	if err := serialization.VertexSerialization.load(stream.Builder()); err != nil {
		return nil, err
	}

	return stream, nil
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
		left, right := next.next["left"], next.next["right"]

		if left != nil {
			if err := left.load(leftBuilder); err != nil {
				return err
			}
		}

		if right != nil {
			return right.load(rightBuilder)
		}
	} else if next, ok := vs.next["loop"]; ok {
		leftBuilder, rightBuilder := builder.Loop(next.ID, next.fork(), next.Options...)
		left, right := next.next["in"], next.next["out"]

		if left != nil {
			if err := left.load(leftBuilder); err != nil {
				return err
			}
		}

		if right != nil {
			return right.load(rightBuilder)
		}
	} else if next, ok := vs.next["publish"]; ok {
		builder.Publish(next.ID, next.publish(), next.Options...)
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

// MarshalJSON implementation to marshal json
func (s *StreamSerialization) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{}

	s.toMap(m)

	return json.Marshal(m)
}

// UnmarshalJSON implementation to unmarshal json
func (s *StreamSerialization) UnmarshalJSON(data []byte) error {
	m := map[string]interface{}{}

	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	return s.fromMap(m)
}

// MarshalYAML implementation to marshal yaml
func (s *StreamSerialization) MarshalYAML() (interface{}, error) {
	m := map[string]interface{}{}

	s.toMap(m)

	return m, nil
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

	if s.Type == subscriptionConst {
		m["interval"] = int64(s.Interval)
	}

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

	if interval, ok := m["interval"]; ok && s.Type == subscriptionConst {
		switch val := interval.(type) {
		case int64:
			s.Interval = time.Duration(val)
		case int:
			s.Interval = time.Duration(val)
		case float64:
			s.Interval = time.Duration(val)
		case string:
		default:
			return fmt.Errorf("invalid interval type expecting int or int64 for %s found %v", s.ID, reflect.TypeOf(val))
		}
	} else if s.Type == subscriptionConst {
		return fmt.Errorf("missing interval field")
	}

	if provider, ok := m["provider"]; ok {
		s.VertexSerialization.Provider = &PluginDefinition{}
		if err := mapstructure.Decode(provider, s.VertexSerialization.Provider); err != nil {
			panic(err)
		}
	}

	delete(m, "type")
	delete(m, "interval")
	delete(m, "provider")

	s.VertexSerialization.fromMap(m)

	return nil
}

func (vs *VertexSerialization) toMap(m map[string]interface{}) {
	if vs.ID != "" {
		m["id"] = vs.ID
	}
	if vs.Provider != nil {
		m["provider"] = vs.Provider
	}
	if vs.Options != nil || len(vs.Options) < 1 {
		m["options"] = vs.Options
	}

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
		vs.Provider = &PluginDefinition{}
		if err := mapstructure.Decode(provider, vs.Provider); err != nil {
			panic(err)
		}
		delete(m, "provider")
	}

	if options, ok := m["options"]; ok {
		vs.Options = []*Option{}
		if err := mapstructure.Decode(options, &vs.Options); err != nil {
			panic(err)
		}
		delete(m, "options")
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
