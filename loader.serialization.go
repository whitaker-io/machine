package machine

import (
	"encoding/json"
	"fmt"
	"time"

	"gopkg.in/yaml.v3"
)

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
	} else if s.Type != httpConst {
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
