package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Financial-Times/go-fthealth/v1_1"
	"github.com/Financial-Times/message-queue-go-producer/producer"
	"github.com/Financial-Times/message-queue-gonsumer/consumer"
	"github.com/stretchr/testify/assert"
)

const (
	DocStoreAPIPath        = "/doc-store-api"
	InternalContentAPIPath = "/internal-content-api"
)

func TestCheckIfDocumentStoreIsReachable_Errors(t *testing.T) {
	expError := errors.New("some error")
	dc := dummyClient{
		err: expError,
	}
	h := HealthcheckHandler{
		docStoreAPIBaseURL: "doc-store-base-url",
		httpClient:         &dc,
	}

	resp, err := h.checkIfDocumentStoreIsReachable()
	assert.Contains(t, err.Error(), expError.Error(), fmt.Sprintf("Expected error %v not equal with received one %v", expError, err))
	assert.Empty(t, resp)
}

func TestCheckIfDocumentStoreIsReachable_Succeeds(t *testing.T) {
	dc := dummyClient{
		statusCode: http.StatusOK,
		body:       "all good",
	}
	h := HealthcheckHandler{
		docStoreAPIBaseURL: "doc-store-base-url",
		httpClient:         &dc,
	}

	resp, err := h.checkIfDocumentStoreIsReachable()
	assert.Nil(t, err)
	assert.Equal(t, ResponseOK, resp)
}

func TestCheckIfInternalContentAPIIsReachable_Errors(t *testing.T) {
	expError := errors.New("some error")
	dc := dummyClient{
		err: expError,
	}
	h := HealthcheckHandler{
		internalContentAPIBaseURL: "internal-content-api-base-url",
		httpClient:                &dc,
	}

	resp, err := h.checkIfInternalContentAPIIsReachable()
	assert.Contains(t, err.Error(), expError.Error(), fmt.Sprintf("Expected error %v not equal with received one %v", expError, err))
	assert.Empty(t, resp)
}

func TestCheckIfInternalContentAPIIsReachable_Succeeds(t *testing.T) {
	dc := dummyClient{
		statusCode: http.StatusOK,
		body:       "all good",
	}
	h := HealthcheckHandler{
		internalContentAPIBaseURL: "internal-content-api-base-url",
		httpClient:                &dc,
	}

	resp, err := h.checkIfInternalContentAPIIsReachable()
	assert.Nil(t, err)
	assert.Equal(t, ResponseOK, resp)
}

func TestAllHealthChecks(t *testing.T) {
	dc := dummyClient{
		statusCode: http.StatusOK,
		body:       "all good",
	}
	h := HealthcheckHandler{
		internalContentAPIBaseURL: "internal-content-api-base-url",
		httpClient:                &dc,
		producer:                  &mockProducer{isConnectionHealthy: true},
		consumer:                  &mockConsumer{isConnectionHealthy: true},
	}
	testCases := []struct {
		description        string
		healthcheckHandler HealthcheckHandler
		healthcheckFunc    func(h *HealthcheckHandler) v1_1.Check
		expectedResponse   string
	}{
		{
			description:        "CombinedPostPublicationEvents messages are being forwarded to the queue",
			healthcheckHandler: h,
			healthcheckFunc:    checkKafkaProxyProducerConnectivity,
			expectedResponse:   "",
		},
		{
			description:        "PostPublicationEvents and PostMetadataPublicationEvents messages are received from the queue",
			healthcheckHandler: h,
			healthcheckFunc:    checkKafkaProxyConsumerConnectivity,
			expectedResponse:   "",
		},
		{
			description:        "Document-store-api is reachable",
			healthcheckHandler: h,
			healthcheckFunc:    checkDocumentStoreAPIHealthcheck,
			expectedResponse:   ResponseOK,
		},
		{
			description:        "Internal-content-api is reachable",
			healthcheckHandler: h,
			healthcheckFunc:    checkInternalContentAPIHealthcheck,
			expectedResponse:   ResponseOK,
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
		producer                 producer.MessageProducer
		consumer                 consumer.MessageConsumer
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

	for _, tc := range testCases {
		t.Run(tc.description, func(t *testing.T) {
			server := getMockedServer(tc.docStoreAPIStatus, tc.internalContentAPIStatus)
			defer server.Close()
			h := NewCombinerHealthcheck(tc.producer, tc.consumer, http.DefaultClient, server.URL+DocStoreAPIPath,
				server.URL+InternalContentAPIPath)

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
	body       string
	err        error
}

func (c dummyClient) Do(req *http.Request) (*http.Response, error) {
	resp := &http.Response{
		StatusCode: c.statusCode,
		Body:       ioutil.NopCloser(strings.NewReader(c.body)),
	}
	return resp, c.err
}

type mockProducer struct {
	isConnectionHealthy bool
}

func (p *mockProducer) SendMessage(string, producer.Message) error {
	return nil
}

func (p *mockProducer) ConnectivityCheck() (string, error) {
	if p.isConnectionHealthy {
		return "", nil
	}

	return "", errors.New("error connecting to the queue")
}

type mockConsumer struct {
	isConnectionHealthy bool
}

func (p *mockConsumer) Start() {
}

func (p *mockConsumer) Stop() {
}

func (p *mockConsumer) ConnectivityCheck() (string, error) {
	if p.isConnectionHealthy {
		return "", nil
	}

	return "", errors.New("error connecting to the queue")
}
