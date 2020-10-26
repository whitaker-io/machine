package sqs

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/sqs"
	"github.com/google/uuid"
	"github.com/spf13/viper"
	"github.com/whitaker-io/machine"
)

// Initium func for providing a sqs based Initium
func Initium(v *viper.Viper) machine.Initium {
	s := session.Must(session.NewSession())

	region := v.GetString("region")
	url := v.GetString("queue_url")
	attributeNames := v.GetStringSlice("attribute_names")
	messageAttributeNames := v.GetStringSlice("message_attribute_names")
	visibilityTimeout := v.GetInt64("visibility_timeout")
	batchSize := v.GetInt64("batch_size")
	waitTimeSeconds := v.GetInt64("wait_time_seconds")
	interval := v.GetDuration("interval")

	attributePNames := []*string{}

	for _, v := range attributeNames {
		value := v
		attributePNames = append(attributePNames, &value)
	}

	messageAttributePNames := []*string{}

	for _, v := range messageAttributeNames {
		value := v
		messageAttributePNames = append(messageAttributePNames, &value)
	}

	svc := sqs.New(s, aws.NewConfig().WithRegion(region))

	channel := make(chan []map[string]interface{})
	return func(ctx context.Context) chan []map[string]interface{} {
		go func() {
		Loop:
			for {
				select {
				case <-ctx.Done():
					break Loop
				case <-time.After(interval):
					id := uuid.New().String()

					input := &sqs.ReceiveMessageInput{
						MaxNumberOfMessages:     &batchSize,
						QueueUrl:                &url,
						VisibilityTimeout:       &visibilityTimeout,
						WaitTimeSeconds:         &waitTimeSeconds,
						AttributeNames:          attributePNames,
						MessageAttributeNames:   messageAttributePNames,
						ReceiveRequestAttemptId: &id,
					}

					output, err := svc.ReceiveMessage(input)

					if err != nil {
						log.Printf("error pulling from sqs %v", err)
					} else {
						payload := []map[string]interface{}{}

						for _, message := range output.Messages {
							m := map[string]interface{}{}
							err := json.Unmarshal([]byte(*message.Body), &m)
							if err != nil {
								log.Printf("error pulling from sqs %v", err)
							} else {
								m["__attributes"] = message.Attributes
								m["__messageAttributes"] = message.MessageAttributes
								m["__receiptHandle"] = message.ReceiptHandle
								payload = append(payload, m)
							}
						}

						channel <- payload
					}
				}
			}
		}()

		return channel
	}
}

// Terminus func for providing a sqs based Terminus
func Terminus(v *viper.Viper) machine.Terminus {
	s := session.Must(session.NewSession())

	region := v.GetString("region")
	url := v.GetString("queue_url")
	attributes := v.GetStringMapString("attributes")
	delay := v.GetInt64("delay")

	attributeMap := map[string]*sqs.MessageAttributeValue{}

	stringType := "String"
	for k, v := range attributes {
		value := v
		attributeMap[k] = &sqs.MessageAttributeValue{
			DataType:    &stringType,
			StringValue: &value,
		}
	}

	svc := sqs.New(s, aws.NewConfig().WithRegion(region))

	return func(m []map[string]interface{}) error {
		groupID := uuid.New().String()
		entries := []*sqs.SendMessageBatchRequestEntry{}
		var errComposite error

		for _, value := range m {
			id := uuid.New().String()

			payload, err := json.Marshal(value)

			if err != nil {
				if errComposite == nil {
					errComposite = err
				} else {
					errComposite = fmt.Errorf("%v "+errComposite.Error(), err)
				}
			} else {
				payloadString := string(payload)
				entries = append(entries, &sqs.SendMessageBatchRequestEntry{
					MessageGroupId:         &groupID,
					DelaySeconds:           &delay,
					Id:                     &id,
					MessageDeduplicationId: &id,
					MessageAttributes:      attributeMap,
					MessageBody:            &payloadString,
				})
			}
		}

		input := &sqs.SendMessageBatchInput{
			QueueUrl: &url,
			Entries:  entries,
		}
		_, err := svc.SendMessageBatch(input)

		if err != nil {
			if errComposite == nil {
				errComposite = err
			} else {
				errComposite = fmt.Errorf("%v "+errComposite.Error(), err)
			}
		}

		return errComposite
	}
}
