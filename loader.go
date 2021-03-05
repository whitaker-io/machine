package machine

import (
	"encoding/json"
	"fmt"
	"plugin"
	"reflect"
	"time"

	"github.com/traefik/yaegi/interp"
	"github.com/traefik/yaegi/stdlib"
	"gopkg.in/yaml.v3"
)

const (
	streamConst       = "stream"
	subscriptionConst = "subscription"
)

var subscriptionProviders map[string]func(map[string]interface{}) Subscription = map[string]func(map[string]interface{}) Subscription{}
var retrieverProviders map[string]func(map[string]interface{}) Retriever = map[string]func(map[string]interface{}) Retriever{}
var applicativeProviders map[string]func(map[string]interface{}) Applicative = map[string]func(map[string]interface{}) Applicative{}
var foldProviders map[string]func(map[string]interface{}) Fold = map[string]func(map[string]interface{}) Fold{}
var forkProviders map[string]func(map[string]interface{}) Fork = map[string]func(map[string]interface{}) Fork{}
var transmitProviders map[string]func(map[string]interface{}) Sender = map[string]func(map[string]interface{}) Sender{}

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

// MarshalJSON implementation to marshal json
func (s *StreamSerialization) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{}

	s.toMap(m)

	return json.Marshal(m)
}

// UnmarshalJSON implementation to unmarshal json
func (s *StreamSerialization) UnmarshalJSON(b []byte) error {
	m := map[string]interface{}{}

	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	return s.fromMap(m)
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
	m["interval"] = s.Interval
	m["provider"] = s.Provider
	m["options"] = s.Options
	m["attributes"] = s.Attributes

	for k, v := range s.next {
		m[k] = v
	}
}

func (s *StreamSerialization) fromMap(m map[string]interface{}) error {
	s.VertexSerialization = &VertexSerialization{}

	if id, ok := m["id"]; ok {
		s.ID = id.(string)
	} else {
		return fmt.Errorf("missing id field")
	}

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
		default:
			panic(fmt.Errorf("invalid interval type expecting int or int64 for %s", s.ID))
		}
	} else if s.Type == subscriptionConst {
		return fmt.Errorf("missing interval field")
	}

	if provider, ok := m["provider"]; ok {
		s.Provider = provider.(string)
	} else {
		return fmt.Errorf("missing provider field")
	}

	if options, ok := m["options"]; ok {
		s.Options = options.([]*Option)
	} else {
		s.Options = []*Option{}
	}

	if attributes, ok := m["attributes"]; ok {
		s.Attributes = attributes.(map[string]interface{})
		delete(m, "attributes")
	} else {
		s.Attributes = map[string]interface{}{}
	}

	s.next = map[string]*VertexSerialization{}

	for k, v := range m {
		switch x := v.(type) {
		case map[string]interface{}:
			vs := &VertexSerialization{
				Options:    []*Option{},
				Attributes: map[string]interface{}{},
			}

			vs.fromMap(x)
			s.next[k] = vs
		case map[interface{}]interface{}:
			m2 := map[string]interface{}{}

			for k2, v2 := range x {
				if str, ok := k2.(string); ok {
					m2[str] = v2
				}
			}

			vs := &VertexSerialization{
				Options:    []*Option{},
				Attributes: map[string]interface{}{},
			}

			vs.fromMap(m2)
			s.next[k] = vs
		}
	}

	return nil
}

// MarshalJSON implementation to marshal json
func (vs *VertexSerialization) MarshalJSON() ([]byte, error) {
	m := map[string]interface{}{}

	vs.toMap(m)

	return json.Marshal(m)
}

// UnmarshalJSON implementation to unmarshal json
func (vs *VertexSerialization) UnmarshalJSON(b []byte) error {
	m := map[string]interface{}{}

	if err := json.Unmarshal(b, &m); err != nil {
		return err
	}

	vs.fromMap(m)

	return nil
}

// MarshalYAML implementation to marshal yaml
func (vs *VertexSerialization) MarshalYAML() (interface{}, error) {
	m := map[string]interface{}{}

	vs.toMap(m)

	return yaml.Marshal(m)
}

// UnmarshalYAML implementation to unmarshal yaml
func (vs *VertexSerialization) UnmarshalYAML(unmarshal func(interface{}) error) error {
	m := map[string]interface{}{}

	if err := unmarshal(&m); err != nil {
		return err
	}

	vs.fromMap(m)

	return nil
}

func (vs *VertexSerialization) toMap(m map[string]interface{}) {
	m["id"] = vs.ID
	m["provider"] = vs.Provider
	m["options"] = vs.Options
	m["attributes"] = vs.Attributes

	for k, v := range vs.next {
		m[k] = v
	}
}

func (vs *VertexSerialization) fromMap(m map[string]interface{}) {
	if id, ok := m["id"]; ok {
		vs.ID = id.(string)
	}

	if provider, ok := m["provider"]; ok {
		vs.Provider = provider.(string)
	}

	if options, ok := m["options"]; ok {
		vs.Options = options.([]*Option)
	}

	if attributes, ok := m["attributes"]; ok {
		vs.Attributes = attributes.(map[string]interface{})
		delete(m, "attributes")
	}

	vs.next = map[string]*VertexSerialization{}

	for k, v := range m {
		switch x := v.(type) {
		case map[string]interface{}:
			vs2 := &VertexSerialization{
				Options:    []*Option{},
				Attributes: map[string]interface{}{},
			}

			vs2.fromMap(x)
			vs.next[k] = vs2
		case map[interface{}]interface{}:
			m2 := map[string]interface{}{}

			for k2, v2 := range x {
				if str, ok := k2.(string); ok {
					m2[str] = v2
				}
			}

			vs2 := &VertexSerialization{
				Options:    []*Option{},
				Attributes: map[string]interface{}{},
			}

			vs2.fromMap(m2)
			vs.next[k] = vs2
		}
	}
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
		case subscriptionConst:
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
	symbols := map[string]interface{}{}

	if pd.Plugins != nil {
		for name, def := range pd.Plugins {
			sym, err := def.load()
			if err != nil {
				return err
			}
			symbols[name] = sym
		}
	}

	if pd.Scripts != nil {
		for name, def := range pd.Scripts {
			sym, err := def.load()
			if err != nil {
				return err
			}
			symbols[name] = sym
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

func (yd *YaegiDefinition) load() (interface{}, error) {
	i := interp.New(interp.Options{})
	i.Use(stdlib.Symbols)
	i.Use(symbols)

	if _, err := i.Eval(yd.Script); err != nil {
		return nil, fmt.Errorf("error evaluating script %w", err)
	}

	sym, err := i.Eval(yd.Symbol)

	if err != nil {
		return nil, fmt.Errorf("error evaluating symbol %w", err)
	}

	if sym.Kind() != reflect.Func {
		return nil, fmt.Errorf("symbol is not of kind func")
	}

	return sym.Interface(), nil
}

func (pd *PluginDefinition) load() (interface{}, error) {
	p, err := plugin.Open(pd.Path)

	if err != nil {
		return nil, fmt.Errorf("error opening plugin %w", err)
	}

	sym, err := p.Lookup(pd.Symbol)

	if err != nil {
		return nil, fmt.Errorf("error looking up symbol %w", err)
	}

	return sym, nil
}
