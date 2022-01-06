package machine

import "context"

// EdgeProvider is an interface that is used for providing new instances
// of the Edge interface given the *Option set in the Stream
type EdgeProvider[T Identifiable] interface {
	New(ctx context.Context, id string, options *Option[T]) Edge[T]
}

// Edge is an inteface that is used for transferring data between vertices
type Edge[T Identifiable] interface {
	SetOutput(ctx context.Context, channel chan []T)
	Input(payload ...T)
}

type edgeProvider[T Identifiable] struct{}

type edge[T Identifiable] struct {
	channel chan []T
}

func (p *edgeProvider[T]) New(ctx context.Context, id string, options *Option[T]) Edge[T] {
	b := 0

	if options.BufferSize != nil {
		b = *options.BufferSize
	}

	return &edge[T]{
		channel: make(chan []T, b),
	}
}

func (out *edge[T]) SetOutput(ctx context.Context, channel chan []T) {
	go func() {
	Loop:
		for {
			select {
			case <-ctx.Done():
				break Loop
			case list := <-out.channel:
				if len(list) > 0 {
					channel <- list
				}
			}
		}
	}()
}

func (out *edge[T]) Input(payload ...T) {
	out.channel <- payload
}
