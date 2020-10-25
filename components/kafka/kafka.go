package kafka

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	kaf "github.com/segmentio/kafka-go"
	"github.com/spf13/viper"
	"github.com/whitaker-io/machine"
)

// Initium func for providing a kafka based Initium
func Initium(v *viper.Viper) machine.Initium {
	topic := v.GetString("topic")
	partition := v.GetInt("partition")
	brokers := v.GetStringSlice("brokers")
	deadline := viper.GetDuration("deadline")
	retries := v.GetInt("retries")
	batchInterval := v.GetDuration("batch.interval")
	batchSize := v.GetInt("batch.size")

	r := kaf.NewReader(kaf.ReaderConfig{
		Brokers:     brokers,
		Topic:       topic,
		Partition:   partition,
		MaxWait:     deadline,
		MaxAttempts: retries,
	})

	channel := make(chan []map[string]interface{})
	return func(ctx context.Context) chan []map[string]interface{} {
		go func() {
		Loop:
			for {
				select {
				case <-ctx.Done():
					break Loop
				case <-time.After(batchInterval):
					payload := []map[string]interface{}{}
					for i := 0; i < batchSize; i++ {
						packet := map[string]interface{}{}
						if message, err := r.ReadMessage(ctx); err != nil {
							log.Printf("error pulling message: %v", err)
						} else if err := json.Unmarshal(message.Value, &packet); err != nil {
							payload = append(payload, packet)
						}
					}
					channel <- payload
				}
			}
		}()

		return channel
	}
}

// Terminus func for providing a kafka based Terminus
func Terminus(v *viper.Viper) machine.Terminus {
	topic := v.GetString("topic")
	brokers := v.GetStringSlice("brokers")
	retries := v.GetInt("retries")

	w := kaf.NewWriter(kaf.WriterConfig{
		Brokers:     brokers,
		Topic:       topic,
		Balancer:    &kaf.LeastBytes{},
		MaxAttempts: retries,
	})

	return func(m []map[string]interface{}) error {
		messages := []kaf.Message{}
		var errComposite error
		for _, packet := range m {
			bytez, err := json.Marshal(packet)
			if err != nil {
				if errComposite == nil {
					errComposite = err
				} else {
					errComposite = fmt.Errorf("%v "+errComposite.Error(), err)
				}
			}
			messages = append(messages, kaf.Message{Value: bytez})
		}

		if err := w.WriteMessages(context.Background(), messages...); err != nil {
			if errComposite == nil {
				errComposite = err
			} else {
				errComposite = fmt.Errorf("%v "+errComposite.Error(), err)
			}
		}

		return errComposite
	}
}
