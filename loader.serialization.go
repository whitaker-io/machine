package machine

import (
	"fmt"
	"reflect"
	"time"

	"gopkg.in/yaml.v3"
)

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
	m["attributes"] = s.Attributes

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
			panic(fmt.Errorf("invalid interval type expecting int or int64 for %s found %v", s.ID, reflect.TypeOf(val)))
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
	m["attributes"] = vs.Attributes

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
		if x, ok := v.(map[string]interface{}); ok {
			vs2 := &VertexSerialization{
				Options:    []*Option{},
				Attributes: map[string]interface{}{},
			}

			vs2.fromMap(x)
			vs.next[k] = vs2
		}
	}
}
