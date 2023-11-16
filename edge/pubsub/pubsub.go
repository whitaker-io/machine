package pubsub

import (
	"context"
	"log/slog"

	"cloud.google.com/go/pubsub"

	"github.com/whitaker-io/machine/v3"
)

type ps[T any] struct {
	subscription *pubsub.Subscription
	publisher    *pubsub.Topic
	to           func(T) *pubsub.Message
	from         func(context.Context, *pubsub.Message) T
	channel      chan T
}

func New[T any](
	ctx context.Context,
	subscription *pubsub.Subscription,
	publisher *pubsub.Topic,
	to func(T) *pubsub.Message,
	from func(context.Context, *pubsub.Message) T,
) machine.Edge[T] {
	channel := make(chan T)

	go func() {
		if err := subscription.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
			channel <- from(ctx, msg)
		}); err != nil {
			slog.Error("SubscriberClient.Receive error", slog.String("error", err.Error()))
		}
	}()

	return &ps[T]{
		subscription: subscription,
		publisher:    publisher,
		to:           to,
		from:         from,
		channel:      channel,
	}
}

func (p *ps[T]) Send(ctx context.Context, payload T) {
	res := p.publisher.Publish(ctx, p.to(payload))

	<-res.Ready()

	if _, err := res.Get(ctx); err != nil {
		panic(err)
	}
}

func (p *ps[T]) Output() chan T {
	return p.channel
}
