package processor

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Financial-Times/kafka-client-go/v4"
	"github.com/Financial-Times/post-publication-combiner/v2/policy"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/dchest/uniuri"
)

var (
	ErrNotFound           = fmt.Errorf("content not found")
	ErrInvalidContentType = fmt.Errorf("invalid content type")
)

type MsgProcessor struct {
	src          <-chan *kafka.FTMessage
	config       MsgProcessorConfig
	dataCombiner dataCombiner
	forwarder    *forwarder
	opaAgent     policy.Agent
	log          *logger.UPPLogger
}

type MsgProcessorConfig struct {
	SupportedHeaders []string
}

func NewMsgProcessorConfig(supportedHeaders []string) MsgProcessorConfig {
	return MsgProcessorConfig{
		SupportedHeaders: supportedHeaders,
	}
}

func NewMsgProcessor(
	log *logger.UPPLogger,
	srcCh <-chan *kafka.FTMessage,
	config MsgProcessorConfig,
	dataCombiner dataCombiner,
	producer messageProducer,
	opaAgent policy.Agent,
	whitelistedContentTypes []string,
) MsgProcessor {
	return MsgProcessor{
		src:          srcCh,
		config:       config,
		dataCombiner: dataCombiner,
		forwarder:    newForwarder(producer, whitelistedContentTypes),
		log:          log,
		opaAgent:     opaAgent,
	}
}

func (p *MsgProcessor) ProcessMessages() {
	for {
		m, more := <-p.src
		if !more {
			break
		}

		if isAnnotationMessage(m.Headers) {
			p.processMetadataMsg(*m)
		} else {
			p.processContentMsg(*m)
		}
	}
}

func isAnnotationMessage(msgHeaders map[string]string) bool {
	msgType, ok := msgHeaders["Message-Type"]
	if !ok {
		return false
	}
	return msgType == "concept-annotation"
}

func (p *MsgProcessor) processContentMsg(m kafka.FTMessage) {
	tid := p.extractTID(m.Headers)
	m.Headers["X-Request-Id"] = tid

	log := p.log.
		WithTransactionID(tid).
		WithField("processor", "content")

	// parse message - collect data, then forward it to the next queue
	var cm ContentMessage
	if err := json.Unmarshal([]byte(m.Body), &cm); err != nil {
		log.WithError(err).Error("Could not unmarshal message")
		return
	}

	var q map[string]interface{}
	if err := json.Unmarshal([]byte(m.Body), &q); err != nil {
		log.WithError(err).Error("Could not unmarshal the OPA Kafka Ingest query.")
		return
	}

	result, err := p.opaAgent.EvaluateKafkaIngestPolicy(q)
	if err != nil {
		log.WithError(err).
			Error("Could not evaluate the OPA Kafka Ingest policy while processing a content message.")
		return
	}
	if result.Skip {
		log.Error(formatOPASkipReasons(result.Reasons))
		return
	}

	uuid := cm.ContentModel.getUUID()
	log = log.WithUUID(uuid)

	var combinedMSG CombinedModel
	if cm.ContentModel.isDeleted() {
		combinedMSG.UUID = uuid
		combinedMSG.ContentURI = cm.ContentURI
		combinedMSG.LastModified = cm.LastModified
		combinedMSG.Deleted = true
	} else {
		combinedMSG, err = p.dataCombiner.GetCombinedModelForContent(cm.ContentModel)
		if err != nil {
			log.
				WithError(err).
				Error("Error obtaining the combined message. Metadata could not be read. Message will be skipped.")
			return
		}

		combinedMSG.ContentURI = cm.ContentURI
	}

	if combinedMSG.InternalContent == nil {
		log.Warn("Could not find internal content when processing a content publish event.")
	}

	if err = p.forwarder.filterAndForwardMsg(m.Headers, &combinedMSG); err != nil {
		log.WithError(err).Error("Failed to forward message to Kafka")
		return
	}

	log.Info("Message successfully forwarded")
}

func (p *MsgProcessor) processMetadataMsg(m kafka.FTMessage) {
	tid := p.extractTID(m.Headers)
	m.Headers["X-Request-Id"] = tid
	h := m.Headers["Origin-System-Id"]

	log := p.log.
		WithTransactionID(tid).
		WithField("processor", "metadata")

	if !containsSubstringOf(p.config.SupportedHeaders, h) {
		log.WithField("originSystem", h).
			Info("Skipped annotations with unsupported Origin-System-Id")
		return
	}

	var ann AnnotationsMessage
	if err := json.Unmarshal([]byte(m.Body), &ann); err != nil {
		log.WithError(err).Error("Could not unmarshal message")
		return
	}

	log.Info(fmt.Sprintf("Content URI: %s", ann.ContentURI))

	combinedMSG, err := p.dataCombiner.GetCombinedModelForAnnotations(ann)
	if err != nil {
		log.WithError(err).
			Error("Error obtaining the combined message. Content couldn't get read. Message will be skipped.")
		return
	}

	if combinedMSG.InternalContent == nil {
		log.Warn("Could not find internal content when processing an annotations publish event.")
	}

	uuid := combinedMSG.Content.getUUID()
	if uuid == "" {
		log.Warn("Skipped. Could not find content when processing an annotations publish event.")
		return
	}

	result, err := p.opaAgent.EvaluateKafkaIngestPolicy(combinedMSG.Content)
	if err != nil {
		log.WithError(err).
			Error("Could not evaluate the OPA Kafka Ingest policy while processing a metadata message.")
		return
	}
	if result.Skip {
		log.Error(formatOPASkipReasons(result.Reasons))
		return
	}

	log = log.WithUUID(uuid)

	if err = p.forwarder.filterAndForwardMsg(m.Headers, &combinedMSG); err != nil {
		log.WithError(err).Error("Failed to forward message to Kafka")
		return
	}

	log.Info("Message successfully forwarded")
}

func (p *MsgProcessor) extractTID(headers map[string]string) string {
	tid := headers["X-Request-Id"]

	if tid == "" {
		tid = "tid_" + uniuri.NewLen(10) + "_post_publication_combiner"
		p.log.Infof("X-Request-Id header was not be found. Generated tid: %s", tid)
	}

	return tid
}

func containsSubstringOf(array []string, element string) bool {
	for _, e := range array {
		if strings.Contains(element, e) {
			return true
		}
	}
	return false
}

func formatOPASkipReasons(r []string) string {
	return strings.Join(r[:], ", ")
}
