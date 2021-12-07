package processor

import (
	"encoding/json"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/kafka-client-go/v2"
)

const (
	CombinerMessageType = "cms-combined-content-published"
)

type messageProducer interface {
	SendMessage(message kafka.FTMessage) error
}

type Forwarder struct {
	producer              messageProducer
	supportedContentTypes []string
	log                   *logger.UPPLogger
}

func NewForwarder(log *logger.UPPLogger, producer messageProducer, supportedContentTypes []string) Forwarder {
	return Forwarder{
		producer:              producer,
		supportedContentTypes: supportedContentTypes,
		log:                   log,
	}
}

func (p *Forwarder) filterAndForwardMsg(headers map[string]string, combinedMSG *CombinedModel, tid string) error {
	if combinedMSG.Content != nil && !isTypeAllowed(p.supportedContentTypes, combinedMSG.Content.getType()) {
		p.log.WithTransactionID(tid).WithUUID(combinedMSG.UUID).Infof("%v - Skipped unsupported content with type: %v", tid, combinedMSG.Content.getType())
		return InvalidContentTypeError
	}

	//forward data
	err := p.forwardMsg(headers, combinedMSG)
	if err != nil {
		p.log.WithTransactionID(tid).WithError(err).Errorf("%v - Error sending transformed message to queue.", tid)
		return err
	}
	p.log.WithTransactionID(tid).Infof("%v - Mapped and sent for uuid: %v", tid, combinedMSG.UUID)
	return nil
}

func isTypeAllowed(allowedTypes []string, value string) bool {
	return contains(allowedTypes, value)
}

func (p *Forwarder) forwardMsg(headers map[string]string, model *CombinedModel) error {
	// marshall message
	b, err := json.Marshal(model)
	if err != nil {
		return err
	}
	// add special message type
	headers["Message-Type"] = CombinerMessageType
	return p.producer.SendMessage(kafka.FTMessage{Headers: headers, Body: string(b)})
}

func contains(array []string, element string) bool {
	for _, e := range array {
		if element == e {
			return true
		}
	}
	return false
}
