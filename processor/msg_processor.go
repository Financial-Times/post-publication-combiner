package processor

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/message-queue-gonsumer/consumer"
	"github.com/dchest/uniuri"
)

var (
	NotFoundError           = errors.New("content not found")
	InvalidContentTypeError = errors.New("invalid content type")
)

type MsgProcessor struct {
	src          <-chan *KafkaMessage
	config       MsgProcessorConfig
	dataCombiner DataCombinerI
	forwarder    *forwarder
	log          *logger.UPPLogger
}

type MsgProcessorConfig struct {
	SupportedContentURIs []string
	SupportedHeaders     []string
	ContentTopic         string
	MetadataTopic        string
}

func NewMsgProcessorConfig(supportedURIs []string, supportedHeaders []string, contentTopic string, metadataTopic string) MsgProcessorConfig {
	return MsgProcessorConfig{
		SupportedContentURIs: supportedURIs,
		SupportedHeaders:     supportedHeaders,
		ContentTopic:         contentTopic,
		MetadataTopic:        metadataTopic,
	}
}

func NewMsgProcessor(log *logger.UPPLogger, srcCh <-chan *KafkaMessage, config MsgProcessorConfig, dataCombiner DataCombinerI, producer messageProducer, whitelistedContentTypes []string) *MsgProcessor {
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
		m := <-p.src
		if m.topic == p.config.ContentTopic {
			p.processContentMsg(m.message)
		} else if m.topic == p.config.MetadataTopic {
			p.processMetadataMsg(m.message)
		}
	}
}

func (p *MsgProcessor) processContentMsg(m consumer.Message) {
	tid := p.extractTID(m.Headers)
	m.Headers["X-Request-Id"] = tid

	log := p.log.
		WithTransactionID(tid).
		WithField("processor", "content")

	//parse message - collect data, then forward it to the next queue
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
			log.WithError(err).Error("Error obtaining the combined message. Metadata could not be read. Message will be skipped.")
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

func (p *MsgProcessor) processMetadataMsg(m consumer.Message) {
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
