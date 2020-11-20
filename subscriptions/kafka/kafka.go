package kafka

import (
	"context"
	"encoding/json"
	"fmt"

	kaf "github.com/segmentio/kafka-go"

	"github.com/whitaker-io/machine"
)

type kafka struct {
	client *kaf.Reader
	logger machine.Logger
}

func (k *kafka) Read(ctx context.Context) []machine.Data {
	payload := []machine.Data{}
	packet := machine.Data{}

	if message, err := k.client.ReadMessage(ctx); err != nil {
		k.logger.Error(fmt.Sprintf("error reading from kafka - %v", err))
	} else if err := json.Unmarshal(message.Value, &packet); err == nil {
		payload = []machine.Data{packet}
	} else if err := json.Unmarshal(message.Value, &payload); err != nil {
		k.logger.Error(fmt.Sprintf("error unmarshalling from kafka - %v", err))
	}

	return payload
}

func (k *kafka) Close() error {
	return k.client.Close()
}

// New func to provide a machine.Subscription based on Kafka
func New(config *kaf.ReaderConfig, logger machine.Logger) machine.Subscription {
	return &kafka{
		client: kaf.NewReader(*config),
		logger: logger,
	}
}
