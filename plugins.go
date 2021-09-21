package machine

import (
	"encoding/json"
	"fmt"

	"github.com/whitaker-io/data"
)

var (
	pluginProviders = map[string]PluginProvider{}

	nodes = map[string]bool{
		"map":        true,
		"window":     true,
		"sort":       true,
		"remove":     true,
		"fold_left":  true,
		"fold_right": true,
		"fork":       true,
		"loop":       true,
		"publish":    true,
	}

	left = map[string]bool{
		"in":   true,
		"left": true,
	}

	right = map[string]bool{
		"out":   true,
		"right": true,
	}
)

// PluginProvider interface for providing a way of loading plugins
// must return one of the following types:
//
//  Subscription
//  Retriever
//  Applicative
//  Comparator
//  Remover
//  Fold
//  Fork
//  Publisher
type PluginProvider interface {
	Load(attributes map[string]interface{}) (interface{}, error)
}

// VertexSerialization config based definition for a stream vertex
type VertexSerialization struct {
	// ID unique identifier for the stream.
	ID string `json:"id,omitempty" mapstructure:"id,omitempty"`

	// Type is the name of the PluginProvider to use.
	Provider string `json:"provider" mapstructure:"provider"`

	// Attributes are a map[string]interface{} of properties to be used with the PluginProvider.
	Attributes map[string]interface{} `json:"attributes" mapstructure:"attributes"`

	typeName string
	next     *VertexSerialization
	left     *VertexSerialization
	right    *VertexSerialization
}

// RegisterPluginProvider function for registering a PluginProvider
// to be used for loading VertexProviders
func RegisterPluginProvider(name string, p PluginProvider) {
	pluginProviders[name] = p
}

func (v *VertexSerialization) load() (interface{}, error) {
	if provider, ok := pluginProviders[v.Provider]; ok {
		return provider.Load(v.Attributes)
	}
	return nil, fmt.Errorf("missing PluginProvider %s", v.Provider)
}

func (v *VertexSerialization) subscription() (Subscription, error) {
	var x Subscription
	var ok bool
	if sym, err := v.load(); err != nil {
		return nil, err
	} else if x, ok = sym.(Subscription); !ok {
		return nil, fmt.Errorf("%s invalid plugin type not Subscription", v.ID)
	}
	return x, nil
}

func (v *VertexSerialization) retriever() (Retriever, error) {
	var x Retriever
	var ok bool
	if sym, err := v.load(); err != nil {
		return nil, err
	} else if x, ok = sym.(Retriever); !ok {
		return nil, fmt.Errorf("%s invalid plugin type not Retriever", v.ID)
	}
	return x, nil
}

func (v *VertexSerialization) applicative() (Applicative, error) {
	var x Applicative
	var ok bool
	if sym, err := v.load(); err != nil {
		return nil, err
	} else if x, ok = sym.(Applicative); !ok {
		return nil, fmt.Errorf("%s invalid plugin type not Applicative", v.ID)
	}
	return x, nil
}

func (v *VertexSerialization) window() (Window, error) {
	var x Window
	var ok bool
	if sym, err := v.load(); err != nil {
		return nil, err
	} else if x, ok = sym.(Window); !ok {
		return nil, fmt.Errorf("%s invalid plugin type not Window", v.ID)
	}
	return x, nil
}

func (v *VertexSerialization) fold() (Fold, error) {
	var x Fold
	var ok bool
	if sym, err := v.load(); err != nil {
		return nil, err
	} else if x, ok = sym.(Fold); !ok {
		return nil, fmt.Errorf("%s invalid plugin type not Fold", v.ID)
	}
	return x, nil
}

func (v *VertexSerialization) fork() (Fork, error) {
	var x Fork
	var ok bool
	if sym, err := v.load(); err != nil {
		return nil, err
	} else if x, ok = sym.(Fork); !ok {
		return nil, fmt.Errorf("%s invalid plugin type not Fork", v.ID)
	}
	return x, nil
}

func (v *VertexSerialization) comparator() (Comparator, error) {
	var x Comparator
	var ok bool
	if sym, err := v.load(); err != nil {
		return nil, err
	} else if x, ok = sym.(Comparator); !ok {
		return nil, fmt.Errorf("%s invalid plugin type not Comparator", v.ID)
	}
	return x, nil
}

func (v *VertexSerialization) remover() (Remover, error) {
	var x Remover
	var ok bool
	if sym, err := v.load(); err != nil {
		return nil, err
	} else if x, ok = sym.(Remover); !ok {
		return nil, fmt.Errorf("%s invalid plugin type not Remover", v.ID)
	}
	return x, nil
}

func (v *VertexSerialization) publisher() (Publisher, error) {
	var x Publisher
	var ok bool
	if sym, err := v.load(); err != nil {
		return nil, err
	} else if x, ok = sym.(Publisher); !ok {
		return nil, fmt.Errorf("%s invalid plugin type not Publisher", v.ID)
	}
	return x, nil
}

func (v *VertexSerialization) apply(b Builder) error {
	if f, ok := b.singles()[v.typeName]; ok {
		if next, err := f(v); err != nil {
			return err
		} else if v.next != nil {
			return v.next.apply(next)
		}
	} else if f, ok := b.splits()[v.typeName]; ok {
		left, right, err := f(v)
		if err != nil {
			return err
		}

		if v.left != nil {
			if err := v.left.apply(left); err != nil {
				return err
			}
		}

		if v.right != nil {
			return v.right.apply(right)
		}
	} else if v.typeName == "publish" {
		return b.PublishPlugin(v)
	}

	return fmt.Errorf("%s type %s not found", v.ID, v.typeName)
}

// MarshalJSON implementation to marshal json
func (v *VertexSerialization) MarshalJSON() ([]byte, error) {
	return json.Marshal(convertToMap(v))
}

// UnmarshalJSON implementation to unmarshal json
func (v *VertexSerialization) UnmarshalJSON(bytez []byte) error {
	m := map[string]interface{}{}

	if err := json.Unmarshal(bytez, &m); err != nil {
		return err
	} else if x, err := convert(m); err != nil {
		return err
	} else {
		v.from(x)
	}

	return nil
}

// MarshalYAML implementation to marshal yaml
func (v *VertexSerialization) MarshalYAML() (interface{}, error) {
	return convertToMap(v), nil
}

// UnmarshalYAML implementation to unmarshal yaml
func (v *VertexSerialization) UnmarshalYAML(unmarshal func(interface{}) error) error {
	m := map[string]interface{}{}

	if err := unmarshal(&m); err != nil {
		return err
	} else if x, err := convert(m); err != nil {
		return err
	} else {
		v.from(x)
	}

	return nil
}

func (v *VertexSerialization) from(x *VertexSerialization) {
	v.ID = x.ID
	v.Provider = x.Provider
	v.Attributes = x.Attributes
	v.typeName = x.typeName
	v.next = x.next
	v.left = x.left
	v.right = x.right
}

func (v *VertexSerialization) convertChildren(m map[string]interface{}) error {
	var err error

	for key, val := range m {
		_, isNode := nodes[key]
		_, isLeft := left[key]
		_, isRight := right[key]

		if x, ok := val.(map[string]interface{}); ok {
			if isNode {
				v.next, err = convert(
					map[string]interface{}{
						key: x,
					},
				)
				return err
			} else if isLeft {
				v.left, err = convertSplit(
					map[string]interface{}{
						key: x,
					},
				)
				return err
			} else if isRight {
				v.right, err = convertSplit(
					map[string]interface{}{
						key: x,
					},
				)
				return err
			}
		}
	}

	return nil
}

func convert(m map[string]interface{}) (*VertexSerialization, error) {
	v := &VertexSerialization{}

	for key, val := range m {
		x, ok := val.(map[string]interface{})

		if !ok {
			return nil, fmt.Errorf("invalid data")
		}

		v.typeName = key

		var err error

		if v.ID, err = data.Data(x).String("id"); err != nil {
			return nil, err
		}

		if v.Provider, err = data.Data(x).String("provider"); err != nil {
			return nil, err
		}

		if v.Attributes, err = data.Data(x).MapStringInterface("attributes"); err != nil {
			return nil, err
		}

		if err := v.convertChildren(m); err != nil {
			return nil, err
		}
	}

	return v, nil
}

func convertSplit(m map[string]interface{}) (*VertexSerialization, error) {
	for key, val := range m {
		if x, ok := val.(map[string]interface{}); ok {
			v, err := convert(x)

			if err != nil {
				return nil, err
			}

			v.typeName = key

			return v, nil
		}
		return nil, fmt.Errorf("invalid type")
	}
	return nil, fmt.Errorf("missing type")
}

func convertToMap(v *VertexSerialization) map[string]interface{} {
	m := map[string]interface{}{}

	x := map[string]interface{}{
		"id":         v.ID,
		"provider":   v.Provider,
		"attributes": v.Attributes,
	}

	if v.next != nil {
		x[v.next.typeName] = convertToMap(v.next)
	}

	if v.left != nil {
		if _, ok := x[v.left.typeName]; !ok {
			x[v.left.typeName] = map[string]map[string]interface{}{}
		}

		x[v.left.typeName] = map[string]interface{}{
			"left": convertToMap(v.left),
		}
	}

	if v.right != nil {
		if _, ok := x[v.right.typeName]; !ok {
			x[v.right.typeName] = map[string]map[string]interface{}{}
		}

		x[v.right.typeName] = map[string]interface{}{
			"right": convertToMap(v.right),
		}
	}

	m[v.typeName] = x

	return m
}
