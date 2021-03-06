package processor

import (
	"encoding/json"
	"errors"
	"strings"

	"github.com/Financial-Times/go-logger"
	"github.com/Financial-Times/message-queue-go-producer/producer"
	"github.com/Financial-Times/message-queue-gonsumer/consumer"
	"github.com/dchest/uniuri"
)

var (
	NotFoundError           = errors.New("content not found") // used when the content can not be found by the platform
	InvalidContentTypeError = errors.New("invalid content type")
)

type MsgProcessor struct {
	src          <-chan *KafkaQMessage
	config       MsgProcessorConfig
	DataCombiner DataCombinerI
	Forwarder    Forwarder
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

func NewMsgProcessor(srcCh <-chan *KafkaQMessage, config MsgProcessorConfig, dataCombiner DataCombinerI, producer producer.MessageProducer, whitelistedContentTypes []string) *MsgProcessor {
	return &MsgProcessor{src: srcCh, config: config, DataCombiner: dataCombiner, Forwarder: NewForwarder(producer, whitelistedContentTypes)}
}

func (p *MsgProcessor) ProcessMessages() {
	for {
		m := <-p.src
		if m.msgType == p.config.ContentTopic {
			p.processContentMsg(m.msg)
		} else if m.msgType == p.config.MetadataTopic {
			p.processMetadataMsg(m.msg)
		}
	}
}

func (p *MsgProcessor) processContentMsg(m consumer.Message) {
	tid := extractTID(m.Headers)
	m.Headers["X-Request-Id"] = tid

	//parse message - collect data, then forward it to the next queue
	var cm ContentMessage
	b := []byte(m.Body)
	if err := json.Unmarshal(b, &cm); err != nil {
		logger.WithTransactionID(tid).WithError(err).Errorf("Could not unmarshal message with TID=%v", tid)
		return
	}

	// wordpress, next-video, methode-article - the system origin is not enough to help us filtering. Filter by contentUri.
	if !containsSubstringOf(p.config.SupportedContentURIs, cm.ContentURI) {
		logger.WithTransactionID(tid).Infof("%v - Skipped unsupported content with contentUri: %v. ", tid, cm.ContentURI)
		return
	}

	uuid := cm.ContentModel.getUUID()
	if uuid == "" {
		logger.WithTransactionID(tid).Errorf("UUID not found after message marshalling, skipping message with contentUri=%v.", cm.ContentURI)
		return
	}

	var combinedMSG CombinedModel

	if cm.ContentModel.isDeleted() {
		combinedMSG.UUID = uuid
		combinedMSG.ContentURI = cm.ContentURI
		combinedMSG.LastModified = cm.LastModified
		combinedMSG.Deleted = true
	} else {
		var err error
		combinedMSG, err = p.DataCombiner.GetCombinedModelForContent(cm.ContentModel)
		if err != nil {
			logger.WithTransactionID(tid).WithUUID(cm.ContentModel.getUUID()).WithError(err).Errorf("%v - Error obtaining the combined message. Metadata could not be read. Message will be skipped.", tid)
			return
		}

		combinedMSG.ContentURI = cm.ContentURI
	}

	//forward data
	p.Forwarder.filterAndForwardMsg(m.Headers, &combinedMSG, tid)
}

func (p *MsgProcessor) processMetadataMsg(m consumer.Message) {

	tid := extractTID(m.Headers)
	m.Headers["X-Request-Id"] = tid
	h := m.Headers["Origin-System-Id"]

	//decide based on the origin system header - whether you want to process the message or not
	if !containsSubstringOf(p.config.SupportedHeaders, h) {
		logger.WithTransactionID(tid).Infof("%v - Skipped unsupported annotations with Origin-System-Id: %v. ", tid, h)
		return
	}

	//parse message - collect data, then forward it to the next queue
	var ann AnnotationsMessage
	b := []byte(m.Body)
	if err := json.Unmarshal(b, &ann); err != nil {
		logger.WithTransactionID(tid).WithError(err).Errorf("Could not unmarshal message with TID=%v", tid)
		return
	}

	//combine data
	combinedMSG, err := p.DataCombiner.GetCombinedModelForAnnotations(ann)
	if err != nil {
		logger.WithTransactionID(tid).WithError(err).Errorf("%v - Error obtaining the combined message. Content couldn't get read. Message will be skipped.", tid)
		return
	}
	if combinedMSG.Content.getUUID() == "" {
		logger.WithTransactionID(tid).Warnf("%v - Skipped. Could not find content when processing an annotations publish event.", tid)
		return
	}

	p.Forwarder.filterAndForwardMsg(m.Headers, &combinedMSG, tid)
}

func extractTID(headers map[string]string) string {
	tid := headers["X-Request-Id"]

	if tid == "" {
		logger.Infof("Couldn't extract transaction id - X-Request-Id header could not be found.")
		tid = "tid_" + uniuri.NewLen(10) + "_post_publication_combiner"
		logger.Infof("Generated tid: %s", tid)
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
