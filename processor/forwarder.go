package processor

import (
	"encoding/json"

	"github.com/Financial-Times/go-logger"
	"github.com/Financial-Times/message-queue-go-producer/producer"
)

const (
	CombinerMessageType = "cms-combined-content-published"
)

type Forwarder struct {
	MsgProducer           producer.MessageProducer
	SupportedContentTypes []string
}

func NewForwarder(msgProducer producer.MessageProducer, supportedContentTypes []string) Forwarder {
	return Forwarder{
		MsgProducer:           msgProducer,
		SupportedContentTypes: supportedContentTypes,
	}
}

func NewProducerConfig(proxyAddress string, topic string, routingHeader string) producer.MessageProducerConfig {
	return producer.MessageProducerConfig{
		Addr:  proxyAddress,
		Topic: topic,
		Queue: routingHeader,
	}
}

func (p *Forwarder) filterAndForwardMsg(headers map[string]string, combinedMSG *CombinedModel, tid string) error {

	if combinedMSG.Content != nil && !isTypeAllowed(p.SupportedContentTypes, combinedMSG.Content.getType()) {
		logger.WithTransactionID(tid).WithUUID(combinedMSG.UUID).Infof("%v - Skipped unsupported content with type: %v", tid, combinedMSG.Content.getType())
		return InvalidContentTypeError
	}

	//forward data
	err := p.forwardMsg(headers, combinedMSG)
	if err != nil {
		logger.WithTransactionID(tid).WithError(err).Errorf("%v - Error sending transformed message to queue.", tid)
		return err
	}
	logger.WithTransactionID(tid).Infof("%v - Mapped and sent for uuid: %v", tid, combinedMSG.UUID)
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
	return p.MsgProducer.SendMessage(model.UUID, producer.Message{Headers: headers, Body: string(b)})
}

func contains(array []string, element string) bool {
	for _, e := range array {
		if element == e {
			return true
		}
	}
	return false
}
