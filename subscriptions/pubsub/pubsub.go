package pubsub

import (
	"context"
	"encoding/json"
	"fmt"

	ps "cloud.google.com/go/pubsub"

	"github.com/whitaker-io/machine"
)

type pubsub struct {
	subscription *ps.Subscription
	logger       machine.Logger
}

func (k *pubsub) Read(ctx context.Context) []machine.Data {
	payload := []machine.Data{}
	packet := machine.Data{}

	err := k.subscription.Receive(context.Background(), func(ctx context.Context, message *ps.Message) {
		if err := json.Unmarshal(message.Data, &packet); err == nil {
			payload = []machine.Data{packet}
		} else if err := json.Unmarshal(message.Data, &payload); err != nil {
			k.logger.Error(fmt.Sprintf("error unmarshalling from pubsub - %v", err))
		}
		message.Ack()
	})

	if err != nil {
		k.logger.Error(fmt.Sprintf("error reading from pubsub - %v", err))
	}

	return payload
}

func (k *pubsub) Close() error {
	return nil
}

// New func to provide a machine.Subscription based on Google Pub/Sub
func New(projectID, subscription, topic string, config *ps.SubscriptionConfig, logger machine.Logger) (machine.Subscription, error) {
	client, err := ps.NewClient(context.Background(), projectID)

	if err != nil {
		return nil, err
	}

	config.Topic = client.Topic(topic)

	sub, err := client.CreateSubscription(context.Background(), subscription, *config)

	if err != nil {
		return nil, err
	}

	return &pubsub{
		subscription: sub,
		logger:       logger,
	}, nil
}
