package machine

// TypeProcessus - processus
const TypeProcessus = "processus"

// Processus type for applying a change to a context
type Processus func(map[string]interface{}) error

// Convert func for providing a Cap
func (p Processus) convert(id, name string, fifo bool) *node {
	return &node{
		labels: labels{
			id:   id,
			name: name,
			fifo: fifo,
		},
		processus: p,
	}
}
