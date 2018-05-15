package processor

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	logger "github.com/Financial-Times/go-logger"
	"github.com/Financial-Times/message-queue-go-producer/producer"
	"github.com/Financial-Times/message-queue-gonsumer/consumer"
	"github.com/Financial-Times/post-publication-combiner/utils"
	"github.com/dchest/uniuri"

	uuidlib "github.com/satori/go.uuid"
)

const (
	CombinerMessageType = "cms-combined-content-published"
	CombinerOrigin      = "forced-combined-msg"
	ContentType         = "application/json"
)

var (
	NotFoundError           = errors.New("Content not found") // used when the content can not be found by the platform
	InvalidContentTypeError = errors.New("Invalid content type")
)

type Processor interface {
	ProcessMessages()
	ForceMessagePublish(uuid, tid string) error
}

type MsgProcessor struct {
	src          <-chan *KafkaQMessage
	client       *http.Client
	config       MsgProcessorConfig
	DataCombiner DataCombinerI
	MsgProducer  producer.MessageProducer
}

type MsgProcessorConfig struct {
	SupportedContentTypes []string
	SupportedContentURIs  []string
	SupportedHeaders      []string
	ContentTopic          string
	MetadataTopic         string
}

func NewMsgProcessorConfig(supportedContentTypes []string, supportedURIs []string, supportedHeaders []string, contentTopic string, metadataTopic string) MsgProcessorConfig {
	return MsgProcessorConfig{
		SupportedContentTypes: supportedContentTypes,
		SupportedContentURIs:  supportedURIs,
		SupportedHeaders:      supportedHeaders,
		ContentTopic:          contentTopic,
		MetadataTopic:         metadataTopic,
	}
}

func NewMsgProcessor(prodConf producer.MessageProducerConfig, srcCh <-chan *KafkaQMessage, docStoreApiUrl utils.ApiURL, annApiUrl utils.ApiURL, c *http.Client, config MsgProcessorConfig) *MsgProcessor {
	p := producer.NewMessageProducerWithHTTPClient(prodConf, c)

	var cRetriever contentRetrieverI = dataRetriever{docStoreApiUrl, c}
	var mRetriever metadataRetrieverI = dataRetriever{annApiUrl, c}

	var combiner DataCombinerI = DataCombiner{
		ContentRetriever:  cRetriever,
		MetadataRetriever: mRetriever,
	}
	return &MsgProcessor{src: srcCh, MsgProducer: p, config: config, client: c, DataCombiner: combiner}
}

func NewProducerConfig(proxyAddress string, topic string, routingHeader string) producer.MessageProducerConfig {
	return producer.MessageProducerConfig{
		Addr:  proxyAddress,
		Topic: topic,
		Queue: routingHeader,
	}
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

func (p *MsgProcessor) ForceMessagePublish(uuid string, tid string) error {

	if tid == "" {
		tid = "tid_force_publish" + uniuri.NewLen(10) + "_post_publication_combiner"
		logger.WithTransactionID(tid).WithUUID(uuid).Infof("Generated tid: %s", tid)
	}

	h := map[string]string{
		"X-Request-Id":     tid,
		"Content-Type":     ContentType,
		"Origin-System-Id": CombinerOrigin,
	}

	//get combined message
	combinedMSG, err := p.DataCombiner.GetCombinedModel(uuid)
	if err != nil {
		logger.WithTransactionID(tid).WithUUID(uuid).WithError(err).Errorf("%v - Error obtaining the combined message, it will be skipped.", tid)
		return err
	}

	if combinedMSG.Content.getUUID() == "" && combinedMSG.Metadata == nil {
		err := NotFoundError
		logger.WithTransactionID(tid).WithUUID(uuid).WithError(err).Errorf("%v - Could not find content with uuid %s.", tid, uuid)
		return err
	}

	//forward data
	return p.filterAndForwardMsg(h, &combinedMSG, tid)
}

func (p *MsgProcessor) processContentMsg(m consumer.Message) {

	tid := extractTID(m.Headers)
	m.Headers["X-Request-Id"] = tid

	//parse message - collect data, then forward it to the next queue
	var cm MessageContent
	b := []byte(m.Body)
	if err := json.Unmarshal(b, &cm); err != nil {
		logger.WithTransactionID(tid).WithError(err).Errorf("Could not unmarshall message with TID=%v", tid)
		return
	}

	// wordpress, next-video, methode-article - the system origin is not enough to help us filtering. Filter by contentUri.
	if !containsSubstringOf(p.config.SupportedContentURIs, cm.ContentURI) {
		logger.WithTransactionID(tid).Infof("%v - Skipped unsupported content with contentUri: %v. ", tid, cm.ContentURI)
		return
	}

	var combinedMSG CombinedModel
	// delete messages have empty payload
	if cm.ContentModel == nil || len(cm.ContentModel) == 0 {

		//handle delete events
		sl := strings.Split(cm.ContentURI, "/")
		uuid := sl[len(sl)-1]
		if _, err := uuidlib.FromString(uuid); err != nil || uuid == "" {
			logger.WithTransactionID(tid).WithError(err).Errorf("UUID couldn't be determined, skipping message with TID=%v.", tid)
			return
		}
		combinedMSG.UUID = uuid
		combinedMSG.ContentURI = cm.ContentURI
		combinedMSG.LastModified = cm.LastModified
		combinedMSG.MarkedDeleted = "true"
	} else {

		//combine data
		if cm.ContentModel.getUUID() == "" {
			logger.WithTransactionID(tid).Errorf("UUID not found after message marshalling, skipping message with contentUri=%v.", cm.ContentURI)
			return
		}

		var err error
		combinedMSG, err = p.DataCombiner.GetCombinedModelForContent(cm.ContentModel)
		if err != nil {
			logger.WithTransactionID(tid).WithUUID(cm.ContentModel.getUUID()).WithError(err).Errorf("%v - Error obtaining the combined message. Metadata could not be read. Message will be skipped.", tid)
			return
		}

		combinedMSG.ContentURI = cm.ContentURI
		combinedMSG.MarkedDeleted = "false"
	}

	//forward data
	p.filterAndForwardMsg(m.Headers, &combinedMSG, tid)
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
	var ann Annotations
	b := []byte(m.Body)
	if err := json.Unmarshal(b, &ann); err != nil {
		logger.WithTransactionID(tid).WithError(err).Errorf("Could not unmarshall message with TID=%v", tid)
		return
	}

	//combine data
	combinedMSG, err := p.DataCombiner.GetCombinedModelForAnnotations(ann)
	if err != nil {
		logger.WithTransactionID(tid).WithError(err).Errorf("%v - Error obtaining the combined message. Content couldn't get read. Message will be skipped.", tid)
		return
	}
	p.filterAndForwardMsg(m.Headers, &combinedMSG, tid)
}

func (p *MsgProcessor) filterAndForwardMsg(headers map[string]string, combinedMSG *CombinedModel, tid string) error {

	if combinedMSG.Content != nil && !isTypeAllowed(p.config.SupportedContentTypes, combinedMSG.Content.getType()) {
		logger.WithTransactionID(tid).Infof("%v - Skipped unsupported content with type: %v", tid, combinedMSG.Content.getType())
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

func (p *MsgProcessor) forwardMsg(headers map[string]string, model *CombinedModel) error {
	// marshall message
	b, err := json.Marshal(model)
	if err != nil {
		return err
	}
	// add special message type
	headers["Message-Type"] = CombinerMessageType
	return p.MsgProducer.SendMessage(model.UUID, producer.Message{Headers: headers, Body: string(b)})
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

func contains(array []string, element string) bool {
	for _, e := range array {
		if element == e {
			return true
		}
	}
	return false
}
