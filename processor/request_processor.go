package processor

import (
	"fmt"
)

const (
	CombinerOrigin = "forced-combined-msg"
	ContentType    = "application/json"
)

type RequestProcessor struct {
	dataCombiner DataCombinerI
	forwarder    *forwarder
}

func NewRequestProcessor(dataCombiner DataCombinerI, producer messageProducer, allowedContentTypes []string) *RequestProcessor {
	return &RequestProcessor{
		dataCombiner: dataCombiner,
		forwarder:    newForwarder(producer, allowedContentTypes),
	}
}

func (p *RequestProcessor) ForcePublication(uuid string, tid string) error {
	h := map[string]string{
		"X-Request-Id":     tid,
		"Content-Type":     ContentType,
		"Origin-System-Id": CombinerOrigin,
	}

	message, err := p.dataCombiner.GetCombinedModel(uuid)
	if err != nil {
		return fmt.Errorf("error obtaining combined message: %w", err)
	}

	if message.Content.getUUID() == "" && message.Metadata == nil {
		return ErrNotFound
	}

	return p.forwarder.filterAndForwardMsg(h, &message)
}
