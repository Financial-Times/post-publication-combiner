package processor

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Financial-Times/kafka-client-go/v4"

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
	log          *logger.UPPLogger
}

type MsgProcessorConfig struct {
	SupportedContentURIs []string
	SupportedHeaders     []string
}

func NewMsgProcessorConfig(supportedURIs, supportedHeaders []string) MsgProcessorConfig {
	return MsgProcessorConfig{
		SupportedContentURIs: supportedURIs,
		SupportedHeaders:     supportedHeaders,
	}
}

func NewMsgProcessor(
	log *logger.UPPLogger,
	srcCh <-chan *kafka.FTMessage,
	config MsgProcessorConfig,
	dataCombiner dataCombiner,
	producer messageProducer,
	whitelistedContentTypes []string,
) *MsgProcessor {
	return &MsgProcessor{
		src:          srcCh,
		config:       config,
		dataCombiner: dataCombiner,
		forwarder:    newForwarder(producer, whitelistedContentTypes),
		log:          log,
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

	// next-video, upp-content-validator - the system origin is not enough to help us filtering. Filter by contentUri.
	if !containsSubstringOf(p.config.SupportedContentURIs, cm.ContentURI) {
		log.WithField("contentUri", cm.ContentURI).Info("Skipped content with unsupported contentUri")
		return
	}

	uuid := cm.ContentModel.getUUID()
	if uuid == "" {
		log.WithField("contentUri", cm.ContentURI).Error("Content UUID was not found. Message will be skipped.")
		return
	}

	log = log.WithUUID(uuid)

	var combinedMSG CombinedModel

	if cm.ContentModel.isDeleted() {
		combinedMSG.UUID = uuid
		combinedMSG.ContentURI = cm.ContentURI
		combinedMSG.LastModified = cm.LastModified
		combinedMSG.Deleted = true
	} else {
		var err error
		combinedMSG, err = p.dataCombiner.GetCombinedModelForContent(cm.ContentModel)
		if err != nil {
			log.
				WithError(err).
				Error("Error obtaining the combined message. Metadata could not be read. Message will be skipped.")
			return
		}

		combinedMSG.ContentURI = cm.ContentURI
	}

	if err := p.forwarder.filterAndForwardMsg(m.Headers, &combinedMSG); err != nil {
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
		log.WithField("originSystem", h).Info("Skipped annotations with unsupported Origin-System-Id")
		return
	}

	var ann AnnotationsMessage
	if err := json.Unmarshal([]byte(m.Body), &ann); err != nil {
		log.WithError(err).Error("Could not unmarshal message")
		return
	}

	combinedMSG, err := p.dataCombiner.GetCombinedModelForAnnotations(ann)
	if err != nil {
		log.WithError(err).Error("Error obtaining the combined message. Content couldn't get read. Message will be skipped.")
		return
	}

	uuid := combinedMSG.Content.getUUID()
	if uuid == "" {
		log.Warn("Skipped. Could not find content when processing an annotations publish event.")
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
