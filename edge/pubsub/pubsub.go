// Package pubsub provides a simple interface for sending and receiving messages
package pubsub

import (
	"context"
	"fmt"
	"time"

	gcp "cloud.google.com/go/pubsub"
)

// Edge is a simple interface for sending and receiving messages.
type Edge[T any] struct {
	ctx          context.Context
	topic        *gcp.Topic
	sub          *gcp.Subscription
	convertTo    func(T) (*gcp.Message, error)
	convertFrom  func(*gcp.Message) (T, error)
	errorHandler func(error)
}

// New creates a new Edge.
//nolint
func New[T any](
	ctx 					context.Context,
	topic 				*gcp.Topic,
	sub 					*gcp.Subscription,
	convertTo 		func(T) (*gcp.Message, error),
	convertFrom 	func(*gcp.Message) (T, error),
	errorHandler 	func(error),
) (*Edge[T], error) {

	return &Edge[T]{
		ctx:          ctx,
		topic:        topic,
		sub:          sub,
		convertTo:    convertTo,
		convertFrom:  convertFrom,
		errorHandler: errorHandler,
	}, nil
}

// Send will send a message to the pubsub topic.
func (e *Edge[T]) Send(payload T) {
	msg, err := e.convertTo(payload)

	if err != nil {
		e.errorHandler(fmt.Errorf("got err from e.convertTo: %v", err))
	}

	result := e.topic.Publish(e.ctx, msg)

	go func(res *gcp.PublishResult) {
		if _, err := res.Get(e.ctx); err != nil {
			e.errorHandler(fmt.Errorf("got err publishing: %v", err))
		}
	}(result)
}

// ReceiveOn will receive messages from the pubsub subscription and send them to the channel.
func (e *Edge[T]) ReceiveOn(ctx context.Context, channel chan T) {
	//nolint
	e.sub.ReceiveSettings.MinExtensionPeriod = 20 * time.Second

	if err := e.sub.Receive(ctx, func(ctx context.Context, msg *gcp.Message) {
		r := msg.AckWithResult()

		// Block until the result is returned and a pubsub.AcknowledgeStatus
		// is returned for the acked message.
		status, err := r.Get(ctx)
		if err != nil {
			e.errorHandler(fmt.Errorf("got err from r.Get: %v", err))
		}

		switch status {
		case gcp.AcknowledgeStatusSuccess:
			if payload, err := e.convertFrom(msg); err != nil {
				e.errorHandler(fmt.Errorf("got err from e.convert: %v", err))
			} else {
				channel <- payload
			}
		default:
			e.errorHandler(errorFromStatus(msg.ID, status))
		}
	}); err != nil {
		e.errorHandler(fmt.Errorf("got err from sub.Receive: %v", err))
	}
}

func errorFromStatus(id string, status gcp.AcknowledgeStatus) error {
	switch status {
	case gcp.AcknowledgeStatusInvalidAckID:
		return fmt.Errorf("message failed to ack with response of Invalid. ID: %s", id)
	case gcp.AcknowledgeStatusPermissionDenied:
		return fmt.Errorf("message failed to ack with response of Permission Denied. ID: %s", id)
	case gcp.AcknowledgeStatusFailedPrecondition:
		return fmt.Errorf("message failed to ack with response of Failed Precondition. ID: %s", id)
	case gcp.AcknowledgeStatusOther:
		return fmt.Errorf("message failed to ack with response of Other. ID: %s", id)
	default:
		return fmt.Errorf("message failed to ack with unknown status. ID: %s, status: %v", id, status)
	}
}
