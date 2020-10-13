package machine

import (
	"context"
)

// Initium type for providing the data to flow into the system
type Initium func(context.Context) chan []map[string]interface{}

// Machine func for providing a Machine
func (i Initium) machine(id, name string, fifo bool, recorder Recorder) *Machine {
	return &Machine{
		labels: labels{
			id:       id,
			name:     name,
			fifo:     fifo,
			recorder: recorder,
		},
		initium: i,
	}
}
