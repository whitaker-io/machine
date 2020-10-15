package machine

import (
	"fmt"
	"reflect"

	"github.com/mitchellh/copystructure"
)

// Packet type that holds information traveling through the machine
type Packet struct {
	ID    string
	Data  map[string]interface{}
	Error error
	last  map[string]interface{}
}

func (c *Packet) apply(id string, p func(map[string]interface{}) error) {
	c.handleError(id, p(c.log(id).Data))
}

func (c *Packet) handleError(id string, err error) *Packet {
	if err != nil {
		c.Error = fmt.Errorf("%s %s %w", id, err.Error(), c.Error)
	}

	return c
}

func (c *Packet) log(id string) *Packet {
	payload, err := copystructure.Copy(c.Data)

	if err != nil {
		return c.handleError(id, err)
	}

	m := payload.(map[string]interface{})

	for k, v := range c.Data {
		if old, ok := m[k]; !ok || !reflect.DeepEqual(old, v) {
			m[k] = v
		} else {
			delete(m, k)
		}
	}

	for k := range m {
		if _, ok := c.Data[k]; !ok {
			m[k] = fmt.Sprintf("REMOVED during: %s", id)
		}
	}

	c.last = payload.(map[string]interface{})

	return c
}

func (c *Packet) error() string {
	if c.Error == nil {
		return ""
	}
	return c.Error.Error()
}
