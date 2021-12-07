package processor

import (
	"github.com/Financial-Times/go-logger/v2"
	"github.com/dchest/uniuri"
)

const (
	CombinerOrigin = "forced-combined-msg"
	ContentType    = "application/json"
)

type RequestProcessorI interface {
	ForceMessagePublish(uuid string, tid string) error
}

type RequestProcessor struct {
	dataCombiner DataCombinerI
	forwarder    Forwarder
	log          *logger.UPPLogger
}

func NewRequestProcessor(log *logger.UPPLogger, dataCombiner DataCombinerI, producer messageProducer, whitelistedContentTypes []string) *RequestProcessor {
	return &RequestProcessor{
		dataCombiner: dataCombiner,
		forwarder:    NewForwarder(log, producer, whitelistedContentTypes),
		log:          log,
	}
}

func (p *RequestProcessor) ForceMessagePublish(uuid string, tid string) error {
	if tid == "" {
		tid = "tid_force_publish" + uniuri.NewLen(10) + "_post_publication_combiner"
		p.log.WithTransactionID(tid).WithUUID(uuid).Infof("Generated tid: %s", tid)
	}

	h := map[string]string{
		"X-Request-Id":     tid,
		"Content-Type":     ContentType,
		"Origin-System-Id": CombinerOrigin,
	}

	//get combined message
	combinedMSG, err := p.dataCombiner.GetCombinedModel(uuid)
	if err != nil {
		p.log.WithTransactionID(tid).WithUUID(uuid).WithError(err).Errorf("%v - Error obtaining the combined message, it will be skipped.", tid)
		return err
	}

	if combinedMSG.Content.getUUID() == "" && combinedMSG.Metadata == nil {
		err := NotFoundError
		p.log.WithTransactionID(tid).WithUUID(uuid).WithError(err).Errorf("%v - Could not find content with uuid %s.", tid, uuid)
		return err
	}

	//forward data
	return p.forwarder.filterAndForwardMsg(h, &combinedMSG, tid)
}
