package main

import (
	"net/http"

	health "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/message-queue-go-producer/producer"
	"github.com/Financial-Times/message-queue-gonsumer/consumer"
	"github.com/Financial-Times/service-status-go/gtg"

	"github.com/Financial-Times/go-logger"
	"github.com/Financial-Times/post-publication-combiner/utils"
)

const (
	GTGEndpoint = "/__gtg"
	ResponseOK  = "OK"
)

type HealthcheckHandler struct {
	httpClient                  utils.Client
	producer                    producer.MessageProducer
	consumer                    consumer.MessageConsumer
	docStoreAPIBaseURL          string
	publicAnnotationsAPIBaseURL string
}

func NewCombinerHealthcheck(p producer.MessageProducer, c consumer.MessageConsumer, client *http.Client, docStoreAPIURL string, publicAnnotationsAPIURL string) *HealthcheckHandler {
	return &HealthcheckHandler{
		httpClient:                  client,
		producer:                    p,
		consumer:                    c,
		docStoreAPIBaseURL:          docStoreAPIURL,
		publicAnnotationsAPIBaseURL: publicAnnotationsAPIURL,
	}
}

func checkKafkaProxyProducerConnectivity(h *HealthcheckHandler) health.Check {
	return health.Check{
		BusinessImpact:   "Can't write CombinedPostPublicationEvents messages to queue. Indexing for search won't work.",
		Name:             "Check connectivity to the kafka-proxy",
		PanicGuide:       "https://dewey.ft.com/post-publication-combiner.html",
		Severity:         1,
		TechnicalSummary: "CombinedPostPublicationEvents messages can't be forwarded to the queue. Check if kafka-proxy is reachable.",
		Checker:          h.producer.ConnectivityCheck,
	}
}

func checkKafkaProxyConsumerConnectivity(h *HealthcheckHandler) health.Check {
	return health.Check{
		BusinessImpact:   "Can't process PostPublicationEvents and PostMetadataPublicationEvents messages. Indexing for search won't work.",
		Name:             "Check connectivity to the kafka-proxy",
		PanicGuide:       "https://dewey.ft.com/post-publication-combiner.html",
		Severity:         1,
		TechnicalSummary: "PostPublicationEvents and PostMetadataPublicationEvents messages are not received from the queue. Check if kafka-proxy is reachable.",
		Checker:          h.consumer.ConnectivityCheck,
	}
}

func checkDocumentStoreAPIHealthcheck(h *HealthcheckHandler) health.Check {
	return health.Check{
		BusinessImpact:   "CombinedPostPublication messages can't be constructed. Indexing for content search won't work.",
		Name:             "Check connectivity to document-store-api",
		PanicGuide:       "https://dewey.ft.com/post-publication-combiner.html",
		Severity:         1,
		TechnicalSummary: "Document-store-api is not reachable. Messages can't be successfully constructed, neither forwarded.",
		Checker:          h.checkIfDocumentStoreIsReachable,
	}
}

func checkPublicAnnotationsAPIHealthcheck(h *HealthcheckHandler) health.Check {
	return health.Check{
		BusinessImpact:   "CombinedPostPublication messages can't be constructed. Indexing for content search won't work.",
		Name:             "Check connectivity to public-annotations-api",
		PanicGuide:       "https://dewey.ft.com/post-publication-combiner.html",
		Severity:         1,
		TechnicalSummary: "Public-annotations-api is not reachable. Messages can't be successfully constructed, neither forwarded.",
		Checker:          h.checkIfPublicAnnotationsAPIIsReachable,
	}
}

func (h *HealthcheckHandler) GTG() gtg.Status {
	consumerCheck := func() gtg.Status {
		return gtgCheck(h.consumer.ConnectivityCheck)
	}
	producerCheck := func() gtg.Status {
		return gtgCheck(h.producer.ConnectivityCheck)
	}
	docStoreCheck := func() gtg.Status {
		return gtgCheck(h.checkIfDocumentStoreIsReachable)
	}
	pubAnnApiCheck := func() gtg.Status {
		return gtgCheck(h.checkIfPublicAnnotationsAPIIsReachable)
	}

	return gtg.FailFastParallelCheck([]gtg.StatusChecker{
		consumerCheck,
		producerCheck,
		docStoreCheck,
		pubAnnApiCheck,
	})()
}

func gtgCheck(handler func() (string, error)) gtg.Status {
	if _, err := handler(); err != nil {
		return gtg.Status{GoodToGo: false, Message: err.Error()}
	}
	return gtg.Status{GoodToGo: true}
}

func (h *HealthcheckHandler) checkIfDocumentStoreIsReachable() (string, error) {
	_, _, err := utils.ExecuteSimpleHTTPRequest(h.docStoreAPIBaseURL+GTGEndpoint, h.httpClient)
	if err != nil {
		logger.Errorf(nil, err,"Healthcheck: %v", err.Error())
		return "", err
	}
	return ResponseOK, nil
}

func (h *HealthcheckHandler) checkIfPublicAnnotationsAPIIsReachable() (string, error) {
	_, _, err := utils.ExecuteSimpleHTTPRequest(h.publicAnnotationsAPIBaseURL+GTGEndpoint, h.httpClient)
	if err != nil {
		logger.Errorf(nil, err,"Healthcheck: %v", err.Error())
		return "", err
	}
	return ResponseOK, nil
}
