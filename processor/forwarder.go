package processor

import (
	"encoding/json"
	"fmt"

	"github.com/Financial-Times/kafka-client-go/v3"
)

const (
	CombinerMessageType = "cms-combined-content-published"
)

type messageProducer interface {
	SendMessage(message kafka.FTMessage) error
}

type forwarder struct {
	producer              messageProducer
	supportedContentTypes []string
}

func newForwarder(producer messageProducer, supportedContentTypes []string) *forwarder {
	return &forwarder{
		producer:              producer,
		supportedContentTypes: supportedContentTypes,
	}
}

func (f *forwarder) filterAndForwardMsg(headers map[string]string, message *CombinedModel) error {
	if message.Content != nil {
		contentType := message.Content.getType()

		if !f.isTypeAllowed(contentType) {
			return fmt.Errorf("%w: %s", ErrInvalidContentType, contentType)
		}
	}

	if err := f.forwardMsg(headers, message); err != nil {
		return fmt.Errorf("error forwarding message to Kafka: %w", err)
	}

	return nil
}

func (f *forwarder) isTypeAllowed(contentType string) bool {
	for _, t := range f.supportedContentTypes {
		if contentType == t {
			return true
		}
	}
	return false
}

func (f *forwarder) forwardMsg(headers map[string]string, message *CombinedModel) error {
	b, err := json.Marshal(message)
	if err != nil {
		return err
	}

	headers["Message-Type"] = CombinerMessageType
	return f.producer.SendMessage(kafka.FTMessage{
		Headers: headers,
		Body:    string(b),
	})
}
