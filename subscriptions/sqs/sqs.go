package sqs

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	ps "github.com/aws/aws-sdk-go/service/sqs"
	"github.com/google/uuid"

	"github.com/whitaker-io/machine"
)

type sqs struct {
	subscription *ps.SQS
	config       *ReadConfig
	logger       machine.Logger
}

// ReadConfig config used for reading messages values match sqs.ReceiveMessageInput from github.com/aws/aws-sdk-go/service/sqs
type ReadConfig struct {
	MaxNumberOfMessages   int64
	QueueURL              string
	VisibilityTimeout     int64
	WaitTimeSeconds       int64
	AttributeNames        []*string
	MessageAttributeNames []*string
}

func (k *sqs) Read(ctx context.Context) []machine.Data {
	payload := []machine.Data{}

	id := uuid.New().String()

	input := &ps.ReceiveMessageInput{
		MaxNumberOfMessages:     &k.config.MaxNumberOfMessages,
		QueueUrl:                &k.config.QueueURL,
		VisibilityTimeout:       &k.config.VisibilityTimeout,
		WaitTimeSeconds:         &k.config.WaitTimeSeconds,
		AttributeNames:          k.config.AttributeNames,
		MessageAttributeNames:   k.config.MessageAttributeNames,
		ReceiveRequestAttemptId: &id,
	}

	output, err := k.subscription.ReceiveMessage(input)

	if err != nil {
		k.logger.Error(fmt.Sprintf("error reading from sqs - %v", err))
	} else {
		for _, message := range output.Messages {
			m := map[string]interface{}{}
			err := json.Unmarshal([]byte(*message.Body), &m)
			if err != nil {
				k.logger.Error(fmt.Sprintf("error unmarshalling from sqs - %v", err))
			} else {
				m["__attributes"] = message.Attributes
				m["__messageAttributes"] = message.MessageAttributes
				m["__receiptHandle"] = message.ReceiptHandle
				payload = append(payload, m)
			}
		}
	}

	return payload
}

func (k *sqs) Close() error {
	return nil
}

// New func to provide a machine.Subscription based on Google Pub/Sub
func New(region string, config *ReadConfig, logger machine.Logger) (machine.Subscription, error) {
	s := session.Must(session.NewSession())
	svc := ps.New(s, aws.NewConfig().WithRegion(region))

	return &sqs{
		subscription: svc,
		config:       config,
		logger:       logger,
	}, nil
}
