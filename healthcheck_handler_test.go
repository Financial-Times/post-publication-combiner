package main

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/go-logger/v2"
	"github.com/stretchr/testify/assert"
)

const (
	DocStoreAPIPath        = "/doc-store-api"
	InternalContentAPIPath = "/internal-content-api"
)

func TestCheckIfDocumentStoreIsReachable_Errors(t *testing.T) {
	expError := fmt.Errorf("some error")
	dc := dummyClient{
		err: expError,
	}
	h := HealthcheckHandler{
		docStoreAPIBaseURL: "doc-store-base-url",
		httpClient:         &dc,
		log:                logger.NewUPPLogger("TEST", "PANIC"),
	}

	resp, err := h.checkIfDocumentStoreIsReachable()
	assert.ErrorIs(t, err, expError)
	assert.Empty(t, resp)
}

func TestCheckIfDocumentStoreIsReachable_Succeeds(t *testing.T) {
	dc := dummyClient{
		statusCode: http.StatusOK,
	}
	h := HealthcheckHandler{
		docStoreAPIBaseURL: "doc-store-base-url",
		httpClient:         &dc,
		log:                logger.NewUPPLogger("TEST", "PANIC"),
	}

	resp, err := h.checkIfDocumentStoreIsReachable()
	assert.Nil(t, err)
	assert.Equal(t, ResponseOK, resp)
}

func TestCheckIfInternalContentAPIIsReachable_Errors(t *testing.T) {
	expError := fmt.Errorf("some error")
	dc := dummyClient{
		err: expError,
	}
	h := HealthcheckHandler{
		internalContentAPIBaseURL: "internal-content-api-base-url",
		httpClient:                &dc,
		log:                       logger.NewUPPLogger("TEST", "PANIC"),
	}

	resp, err := h.checkIfInternalContentAPIIsReachable()
	assert.ErrorIs(t, err, expError)
	assert.Empty(t, resp)
}

func TestCheckIfInternalContentAPIIsReachable_Succeeds(t *testing.T) {
	dc := dummyClient{
		statusCode: http.StatusOK,
	}
	h := HealthcheckHandler{
		internalContentAPIBaseURL: "internal-content-api-base-url",
		httpClient:                &dc,
		log:                       logger.NewUPPLogger("TEST", "PANIC"),
	}

	resp, err := h.checkIfInternalContentAPIIsReachable()
	assert.Nil(t, err)
	assert.Equal(t, ResponseOK, resp)
}

func TestAllHealthChecks(t *testing.T) {
	dc := dummyClient{
		statusCode: http.StatusOK,
	}
	h := HealthcheckHandler{
		internalContentAPIBaseURL: "internal-content-api-base-url",
		httpClient:                &dc,
		producer:                  &mockProducer{isConnectionHealthy: true},
		consumer:                  &mockConsumer{isConnectionHealthy: true},
	}
	testCases := []struct {
		description      string
		healthcheckFunc  func(h *HealthcheckHandler) v1_1.Check
		expectedResponse string
	}{
		{
			description:      "CombinedPostPublicationEvents messages are being forwarded to the queue",
			healthcheckFunc:  checkKafkaProducerConnectivity,
			expectedResponse: "Successfully connected to Kafka",
		},
		{
			description:      "PostPublicationEvents and PostMetadataPublicationEvents messages are received from the queue",
			healthcheckFunc:  checkKafkaProxyConsumerConnectivity,
			expectedResponse: "",
		},
		{
			description:      "Document-store-api is reachable",
			healthcheckFunc:  checkDocumentStoreAPIHealthcheck,
			expectedResponse: ResponseOK,
		},
		{
			description:      "Internal-content-api is reachable",
			healthcheckFunc:  checkInternalContentAPIHealthcheck,
			expectedResponse: ResponseOK,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			resp, err := tc.healthcheckFunc(&h).Checker()
			assert.Nil(t, err)
			assert.Equal(t, tc.expectedResponse, resp)
		})
	}
}

func TestGTG_Good(t *testing.T) {
	dc := dummyClient{
		statusCode: http.StatusOK,
	}
	h := HealthcheckHandler{
		httpClient:                &dc,
		producer:                  &mockProducer{isConnectionHealthy: true},
		consumer:                  &mockConsumer{isConnectionHealthy: true},
		docStoreAPIBaseURL:        "doc-store-base-url",
		internalContentAPIBaseURL: "internal-content-api-base-url",
	}

	status := h.GTG()
	assert.True(t, status.GoodToGo)
	assert.Empty(t, status.Message)
}

func TestGTG_Bad(t *testing.T) {
	testCases := []struct {
		description              string
		producer                 messageProducer
		consumer                 messageConsumer
		docStoreAPIStatus        int
		internalContentAPIStatus int
	}{
		{
			description:              "Producer KafkaProxy GTG endpoint returns 503",
			producer:                 &mockProducer{isConnectionHealthy: false},
			consumer:                 &mockConsumer{isConnectionHealthy: true},
			docStoreAPIStatus:        200,
			internalContentAPIStatus: 200,
		},
		{
			description:              "Consumer KafkaProxy GTG endpoint returns 503",
			producer:                 &mockProducer{isConnectionHealthy: true},
			consumer:                 &mockConsumer{isConnectionHealthy: false},
			docStoreAPIStatus:        200,
			internalContentAPIStatus: 200,
		},
		{
			description:              "DocumentStoreApi GTG endpoint returns 503",
			producer:                 &mockProducer{isConnectionHealthy: true},
			consumer:                 &mockConsumer{isConnectionHealthy: true},
			docStoreAPIStatus:        503,
			internalContentAPIStatus: 200,
		},
		{
			description:              "InternalContentAPI GTG endpoint returns 503",
			producer:                 &mockProducer{isConnectionHealthy: true},
			consumer:                 &mockConsumer{isConnectionHealthy: true},
			docStoreAPIStatus:        200,
			internalContentAPIStatus: 503,
		},
	}

	log := logger.NewUPPLogger("TEST", "PANIC")

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			server := getMockedServer(tc.docStoreAPIStatus, tc.internalContentAPIStatus)
			defer server.Close()
			h := NewCombinerHealthcheck(log, tc.producer, tc.consumer, http.DefaultClient, server.URL+DocStoreAPIPath, server.URL+InternalContentAPIPath)

			status := h.GTG()
			assert.False(t, status.GoodToGo)
		})
	}
}

func getMockedServer(docStoreAPIStatus, internalContentAPIStatus int) *httptest.Server {
	mux := http.NewServeMux()
	server := httptest.NewServer(mux)
	mux.HandleFunc(DocStoreAPIPath+GTGEndpoint, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(docStoreAPIStatus)
	})
	mux.HandleFunc(InternalContentAPIPath+GTGEndpoint, func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(internalContentAPIStatus)
	})
	return server
}

type dummyClient struct {
	statusCode int
	err        error
}

func (c dummyClient) Do(*http.Request) (*http.Response, error) {
	resp := &http.Response{
		StatusCode: c.statusCode,
		Body:       io.NopCloser(strings.NewReader("")),
	}
	return resp, c.err
}

type mockProducer struct {
	isConnectionHealthy bool
}

func (p *mockProducer) ConnectivityCheck() error {
	if p.isConnectionHealthy {
		return nil
	}

	return fmt.Errorf("error connecting to the queue")
}

type mockConsumer struct {
	isConnectionHealthy bool
}

func (p *mockConsumer) ConnectivityCheck() (string, error) {
	if p.isConnectionHealthy {
		return "", nil
	}

	return "", fmt.Errorf("error connecting to the queue")
}
