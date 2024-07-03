package processor

import (
	"fmt"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/post-publication-combiner/v2/policy"
)

const (
	CombinerOrigin = "forced-combined-msg"
	ContentType    = "application/json"
)

type RequestProcessor struct {
	dataCombiner dataCombiner
	forwarder    *forwarder
	log          *logger.UPPLogger
	opaAgent     policy.Agent
}

func NewRequestProcessor(dataCombiner dataCombiner, producer messageProducer, allowedContentTypes []string, log *logger.UPPLogger, opaAgent policy.Agent) *RequestProcessor {
	return &RequestProcessor{
		dataCombiner: dataCombiner,
		forwarder:    newForwarder(producer, allowedContentTypes),
		log:          log,
		opaAgent:     opaAgent,
	}
}

func (p *RequestProcessor) ForcePublication(uuid string, tid string) error {
	h := map[string]string{
		"X-Request-Id":     tid,
		"Content-Type":     ContentType,
		"Origin-System-Id": CombinerOrigin,
	}

	log := p.log.
		WithTransactionID(tid).
		WithField("processor", "RequestProcessor")

	message, err := p.dataCombiner.GetCombinedModel(uuid)
	if err != nil {
		return fmt.Errorf("error obtaining combined message: %w", err)
	}

	if message.Content.getUUID() == "" && message.Metadata == nil {
		return ErrNotFound
	}

	result, err := p.opaAgent.EvaluateKafkaIngestPolicy(
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

	return p.forwarder.filterAndForwardMsg(h, &message)
}
