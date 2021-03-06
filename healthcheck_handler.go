package main

import (
	health "github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/go-logger"
	"github.com/Financial-Times/message-queue-go-producer/producer"
	"github.com/Financial-Times/message-queue-gonsumer/consumer"
	"github.com/Financial-Times/post-publication-combiner/v2/utils"
	"github.com/Financial-Times/service-status-go/gtg"
)

const (
	GTGEndpoint = "/__gtg"
	ResponseOK  = "OK"
)

type HealthcheckHandler struct {
	httpClient                utils.Client
	producer                  producer.MessageProducer
	consumer                  consumer.MessageConsumer
	docStoreAPIBaseURL        string
	internalContentAPIBaseURL string
}

func NewCombinerHealthcheck(p producer.MessageProducer, c consumer.MessageConsumer, client utils.Client, docStoreAPIURL string, internalContentAPIURL string) *HealthcheckHandler {
	return &HealthcheckHandler{
		httpClient:                client,
		producer:                  p,
		consumer:                  c,
		docStoreAPIBaseURL:        docStoreAPIURL,
		internalContentAPIBaseURL: internalContentAPIURL,
	}
}

func checkKafkaProxyProducerConnectivity(h *HealthcheckHandler) health.Check {
	return health.Check{
		BusinessImpact:   "Can't write CombinedPostPublicationEvents and ForcedCombinedPostPublicationEvents messages to queue. Indexing for search won't work.",
		Name:             "Check connectivity to the kafka-proxy",
		PanicGuide:       "https://runbooks.in.ft.com/post-publication-combiner",
		Severity:         2,
		TechnicalSummary: "CombinedPostPublicationEvents and ForcedCombinedPostPublicationEvents messages can't be forwarded to the queue. Check if kafka-proxy is reachable.",
		Checker:          h.producer.ConnectivityCheck,
	}
}

func checkKafkaProxyConsumerConnectivity(h *HealthcheckHandler) health.Check {
	return health.Check{
		BusinessImpact:   "Can't process PostPublicationEvents and PostMetadataPublicationEvents messages. Indexing for search won't work.",
		Name:             "Check connectivity to the kafka-proxy",
		PanicGuide:       "https://runbooks.in.ft.com/post-publication-combiner",
		Severity:         2,
		TechnicalSummary: "PostPublicationEvents and PostMetadataPublicationEvents messages are not received from the queue. Check if kafka-proxy is reachable.",
		Checker:          h.consumer.ConnectivityCheck,
	}
}

func checkDocumentStoreAPIHealthcheck(h *HealthcheckHandler) health.Check {
	return health.Check{
		BusinessImpact:   "CombinedPostPublication messages can't be constructed. Indexing for content search won't work.",
		Name:             "Check connectivity to document-store-api",
		PanicGuide:       "https://runbooks.in.ft.com/document-store-api",
		Severity:         2,
		TechnicalSummary: "Document-store-api is not reachable. Messages can't be successfully constructed, neither forwarded.",
		Checker:          h.checkIfDocumentStoreIsReachable,
	}
}

func checkInternalContentAPIHealthcheck(h *HealthcheckHandler) health.Check {
	return health.Check{
		BusinessImpact:   "CombinedPostPublication messages can't be constructed. Indexing for content search won't work.",
		Name:             "Check connectivity to internal-content-api",
		PanicGuide:       "https://runbooks.in.ft.com/up-ica",
		Severity:         2,
		TechnicalSummary: "Internal-content-api is not reachable. Messages can't be successfully constructed, neither forwarded.",
		Checker:          h.checkIfInternalContentAPIIsReachable,
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
	internalContentAPICheck := func() gtg.Status {
		return gtgCheck(h.checkIfInternalContentAPIIsReachable)
	}

	return gtg.FailFastParallelCheck([]gtg.StatusChecker{
		consumerCheck,
		producerCheck,
		docStoreCheck,
		internalContentAPICheck,
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
		logger.WithError(err).Errorf("Healthcheck error: %v", err.Error())
		return "", err
	}
	return ResponseOK, nil
}

func (h *HealthcheckHandler) checkIfInternalContentAPIIsReachable() (string, error) {
	_, _, err := utils.ExecuteSimpleHTTPRequest(h.internalContentAPIBaseURL+GTGEndpoint, h.httpClient)
	if err != nil {
		logger.WithError(err).Errorf("Healthcheck error: %v", err.Error())
		return "", err
	}
	return ResponseOK, nil
}
