package pubsub

import (
	"context"
	"encoding/json"
	"log"

	"cloud.google.com/go/pubsub"
	"github.com/spf13/viper"
	"github.com/whitaker-io/machine"
)

// Initium func for providing a bigquery based Initium
func Initium(v *viper.Viper) machine.Initium {
	projectID := v.GetString("project_id")
	topic := v.GetString("topic")
	subscription := v.GetString("subscription")

	client, err := pubsub.NewClient(context.Background(), projectID)

	if err != nil {
		log.Fatalf("error connecting to pubsub %v", err)
	}

	sub, err := client.CreateSubscription(context.Background(), subscription,
		pubsub.SubscriptionConfig{Topic: client.Topic(topic)})

	channel := make(chan []map[string]interface{})
	return func(ctx context.Context) chan []map[string]interface{} {
		go func() {
		Loop:
			for {
				select {
				case <-ctx.Done():
					break Loop
				default:
					err := sub.Receive(context.Background(), func(ctx context.Context, m *pubsub.Message) {
						payload := []map[string]interface{}{}
						_ = json.Unmarshal(m.Data, &payload)
						channel <- payload
						m.Ack()
					})
					if err != nil {
						log.Printf("error receiving data %v", err)
					}
				}
			}
		}()
		return channel
	}
}

// Terminus func for providing a bigquery based Terminus
func Terminus(v *viper.Viper) machine.Terminus {
	projectID := v.GetString("project_id")
	topic := v.GetString("topic")

	client, err := pubsub.NewClient(context.Background(), projectID)

	if err != nil {
		log.Fatalf("error connecting to pubsub %v", err)
	}

	tpc := client.Topic(topic)

	return func(m []map[string]interface{}) error {
		result := tpc.Publish(context.Background(), &pubsub.Message{Data: []byte("payload")})
		<-result.Ready()
		_, err := result.Get(context.Background())
		return err
	}
}
