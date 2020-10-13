package machine

// Terminus type for ending a chain and returning an error if exists
type Terminus func([]map[string]interface{}) error

// Convert func for providing a Cap
func (t Terminus) convert(id, name string, fifo bool) vertex {
	return &cap{
		labels: labels{
			id:   id,
			name: name,
			fifo: fifo,
		},
		terminus: t,
		input:    newInChannel(),
	}
}
