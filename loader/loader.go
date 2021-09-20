// Package loader - Copyright © 2020 Jonathan Whitaker <github@whitaker.io>.
//
// Use of this source code is governed by an MIT-style
// license that can be found in the LICENSE file.
package loader

import (
	"encoding/json"
	"fmt"
	"reflect"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/whitaker-io/data"
	"github.com/whitaker-io/machine"
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

type loadable interface {
	symbol() (interface{}, error)
	load(*VertexSerialization, machine.Builder) error
	Type() string
	toMap() map[string]interface{}
	setAttribute(string, interface{})
}

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
	Load(xType, payload, symbol string, attributes map[string]interface{}) (interface{}, error)
}

type loader struct {
	typeName   string
	payload    string
	reference  string
	attributes map[string]interface{}
}

// StreamSerialization config based definition for a stream
type StreamSerialization struct {
	// Interval is the duration in nanoseconds between pulls in a 'subscription' Type. It is only read
	// if the Type is 'subscription'.
	Interval time.Duration `json:"interval,omitempty" mapstructure:"interval,omitempty"`
	// Options are a slice of machine.Option https://godoc.org/github.com/whitaker-io/machine#Option
	Options []*machine.Option `json:"options,omitempty" mapstructure:"options,omitempty"`

	*VertexSerialization
}

// VertexSerialization config based definition for a stream vertex
type VertexSerialization struct {
	// ID unique identifier for the stream.
	ID string `json:"id,omitempty" mapstructure:"id,omitempty"`

	loadable

	next  *VertexSerialization
	left  *VertexSerialization
	right *VertexSerialization
}

// RegisterPluginProvider function for registering a PluginProvider
// to be used for loading VertexProviders
func RegisterPluginProvider(name string, p PluginProvider) {
	pluginProviders[name] = p
}

// Load method loads a stream based on the StreamSerialization
func Load(serialization *StreamSerialization) (machine.Stream, error) {
	if serialization.next == nil {
		return nil, fmt.Errorf("%s non-terminated", serialization.ID)
	}

	var stream machine.Stream
	switch serialization.Type() {
	case subscriptionConst:
		if sym, err := serialization.symbol(); err != nil {
			return nil, err
		} else if x, ok := sym.(machine.Subscription); !ok {
			return nil, fmt.Errorf("invalid plugin type not subscription")
		} else {
			stream = machine.NewSubscriptionStream(serialization.ID, x, serialization.Interval, serialization.Options...)
		}
	case streamConst:
		if sym, err := serialization.symbol(); err != nil {
			return nil, err
		} else if x, ok := sym.(machine.Retriever); !ok {
			return nil, fmt.Errorf("invalid plugin type not retriever")
		} else {
			stream = machine.NewStream(serialization.ID, x, serialization.Options...)
		}
	default:
		return nil, fmt.Errorf("invalid type")
	}

	if err := serialization.next.load(serialization.next, stream.Builder()); err != nil {
		return nil, err
	}

	return stream, nil
}

// LoadHTTP method loads an HTTPStream based on the StreamSerialization
func LoadHTTP(serialization *StreamSerialization) (machine.HTTPStream, error) {
	var stream machine.HTTPStream
	switch serialization.Type() {
	case httpConst:
		stream = machine.NewHTTPStream(serialization.ID, serialization.Options...)
	case websocketConst:
		stream = machine.NewWebsocketStream(serialization.ID, serialization.Options...)
	default:
		return nil, fmt.Errorf("invalid type")
	}

	if err := serialization.next.load(serialization.next, stream.Builder()); err != nil {
		return nil, err
	}

	return stream, nil
}

func (l *loader) symbol() (interface{}, error) {
	if provider, ok := pluginProviders[l.typeName]; ok {
		return provider.Load(l.typeName, l.payload, l.reference, l.attributes)
	}
	return nil, fmt.Errorf("missing PluginProvider %s", l.typeName)
}

func (l *loader) setAttribute(key string, val interface{}) {
	l.attributes[key] = val
}

func (l *loader) toMap() map[string]interface{} {
	m := map[string]interface{}{}

	m["type"] = l.typeName
	m["symbol"] = l.reference
	m["payload"] = l.payload
	m["attributes"] = l.attributes

	return m
}

// MarshalJSON implementation to marshal json
func (s *StreamSerialization) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.toMap())
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
	return s.toMap(), nil
}

// UnmarshalYAML implementation to unmarshal yaml
func (s *StreamSerialization) UnmarshalYAML(unmarshal func(interface{}) error) error {
	m := map[string]interface{}{}

	if err := unmarshal(&m); err != nil {
		return err
	}

	return s.fromMap(m)
}

func (s *StreamSerialization) toMap() map[string]interface{} {
	typeName, m := s.VertexSerialization.toMap()

	m["type"] = typeName

	if s.Type() == subscriptionConst {
		m["interval"] = int64(s.Interval)
	}

	m["options"] = s.Options

	return m
}

func (s *StreamSerialization) fromMap(m map[string]interface{}) error {
	s.VertexSerialization = &VertexSerialization{}

	if typeName, err := data.Data(m).String("type"); err != nil {
		return err
	} else if err := s.VertexSerialization.fromMap(typeName, m); err != nil {
		return err
	}

	if interval, ok := m["interval"]; ok && s.Type() == subscriptionConst {
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
	} else if s.Type() == subscriptionConst {
		return fmt.Errorf("%s missing interval field", s.ID)
	}

	if options, ok := m["options"]; ok {
		s.Options = []*machine.Option{}
		if err := mapstructure.Decode(options, &s.Options); err != nil {
			return err
		}
	}

	return nil
}

func (vs *VertexSerialization) toMap() (string, map[string]interface{}) {
	m := map[string]interface{}{}
	m["id"] = vs.ID
	m["provider"] = vs.loadable.toMap()

	if vs.Type() == "loop" {
		if vs.left != nil {
			leftType, left := vs.left.toMap()
			m["in"] = map[string]interface{}{
				leftType: left,
			}
		}

		if vs.right != nil {
			rightType, right := vs.right.toMap()
			m["out"] = map[string]interface{}{
				rightType: right,
			}
		}
	} else if vs.Type() == "fork" {
		if vs.left != nil {
			leftType, left := vs.left.toMap()
			m["left"] = map[string]interface{}{
				leftType: left,
			}
		}

		if vs.right != nil {
			rightType, right := vs.right.toMap()
			m["right"] = map[string]interface{}{
				rightType: right,
			}
		}
	} else {
		if vs.next != nil {
			nextType, next := vs.next.toMap()
			m[nextType] = next
		}
	}

	return vs.Type(), m
}

func (vs *VertexSerialization) fromMap(typeName string, m map[string]interface{}) error {
	id, exists := m["id"]

	if x, isString := id.(string); exists && isString {
		vs.ID = x
	} else {
		return fmt.Errorf("%v missing id", vs)
	}

	provider, exists := m["provider"]

	if x, isMap := provider.(map[string]interface{}); exists && isMap {
		l := &loader{}

		var err error

		if l.typeName, err = data.Data(x).String("type"); err != nil {
			return err
		} else if l.payload, err = data.Data(x).String("payload"); err != nil {
			return err
		} else if l.reference, err = data.Data(x).String("symbol"); err != nil {
			return err
		}
		l.attributes = data.Data(x).MapStringInterfaceOr("attributes", map[string]interface{}{})

		vs.loadable = toLoadable(typeName, l)
	} else if typeName == "http" {
		vs.loadable = &httpLoader{}
	} else if typeName == "websocket" {
		vs.loadable = &websocketLoader{}
	} else {
		return fmt.Errorf("%s missing provider", vs.ID)
	}

	delete(m, "provider")

	var err error

	for k, v := range m {
		if x, ok := v.(map[string]interface{}); ok {
			switch k {
			case "map":
				fallthrough
			case "window":
				fallthrough
			case "fold_left":
				fallthrough
			case "fold_right":
				fallthrough
			case "sort":
				fallthrough
			case "remove":
				fallthrough
			case "publish":
				fallthrough
			case "fork":
				fallthrough
			case "loop":
				vs.next, err = fromMap(k, x)
				return err
			case "left":
				fallthrough
			case "in":
				key, val, hErr := handleSplit(x)

				if hErr != nil {
					return err
				}

				vs.left, err = fromMap(key, val)

				if err != nil {
					return err
				}
			case "right":
				fallthrough
			case "out":
				key, val, hErr := handleSplit(x)

				if hErr != nil {
					return err
				}

				vs.right, err = fromMap(key, val)

				if err != nil {
					return err
				}
			}
		}
	}

	return err
}

func fromMap(typeName string, m map[string]interface{}) (*VertexSerialization, error) {
	if len(m) < 1 {
		return nil, fmt.Errorf("empty serialization")
	}

	vs := &VertexSerialization{}

	if err := vs.fromMap(typeName, m); err != nil {
		return nil, err
	}

	return vs, nil
}

func toLoadable(typeName string, l *loader) loadable {
	switch typeName {
	case "stream":
		return &retrieverLoader{*l}
	case "subscription":
		return &subscriptionLoader{*l}
	case "http":
		return &httpLoader{*l}
	case "websocket":
		return &websocketLoader{*l}
	case "map":
		return &mapLoader{*l}
	case "window":
		return &windowLoader{*l}
	case "fold_left":
		return &foldLeftLoader{*l}
	case "fold_right":
		return &foldRightLoader{*l}
	case "sort":
		return &sortLoader{*l}
	case "remove":
		return &removerLoader{*l}
	case "loop":
		return &loopLoader{*l}
	case "publish":
		return &publishLoader{*l}
	case "fork":
		return &forkLoader{*l}
	}

	return nil
}

func handleSplit(x map[string]interface{}) (string, map[string]interface{}, error) {
	for k, v := range x {
		if val, ok := v.(map[string]interface{}); ok {
			return k, val, nil
		}
		return "", nil, fmt.Errorf("invalid type")
	}
	return "", nil, fmt.Errorf("missing type")
}
