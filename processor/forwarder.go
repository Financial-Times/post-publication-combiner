package processor

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/kafka-client-go/v4"
	"github.com/Financial-Times/post-publication-combiner/v2/policy"
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
	log                   *logger.UPPLogger
	opaAgent              policy.Agent
}

func newForwarder(producer messageProducer, supportedContentTypes []string, log *logger.UPPLogger, opaAgent policy.Agent) *forwarder {
	return &forwarder{
		producer:              producer,
		supportedContentTypes: supportedContentTypes,
	}
}

func (f *forwarder) filterAndForwardMsg(headers map[string]string, message *CombinedModel) error {

	tid := message.UUID
	log := f.log.
		WithTransactionID(tid).
		WithField("processor", "forwarder")

	if message.Content != nil {
		contentType := message.Content.getType()

		if !f.isTypeAllowed(contentType) {
			return fmt.Errorf("%w: %s", ErrInvalidContentType, contentType)
		}
	}

	result, err := f.opaAgent.EvaluateKafkaIngestPolicy(
		message.Content,
		policy.KafkaIngestMetadata,
	)
	if err != nil {
		log.WithError(err).
			Error("Could not evaluate the OPA Kafka Ingest policy while processing a /content/ metadata message.")
		return err
	}
	if result.Skip {
		log.Error(formatOPASkipReasons(result.Reasons))
		return err
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

func formatOPASkipReasons(r []string) string {
	return strings.Join(r[:], ", ")
}
