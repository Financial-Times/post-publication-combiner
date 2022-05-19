package main

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/post-publication-combiner/v2/processor"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func Test_PublishMessage(t *testing.T) {
	tests := []struct {
		uuid   string
		tid    string
		err    error
		status int
	}{
		{
			uuid:   "a78cf3ea-b221-46f8-8cbc-a61e5e454e88",
			tid:    "tid_1",
			status: 200,
		},
		{
			uuid:   "a78cf3ea-b221-46f8-8cbc-a61e5e454e88",
			status: 200,
		},
		{
			uuid:   "invalid",
			tid:    "tid_1",
			status: 400,
		},
		{
			uuid:   "a78cf3ea-b221-46f8-8cbc-a61e5e454e88",
			tid:    "tid_1",
			err:    fmt.Errorf("test error"),
			status: 500,
		},
		{
			uuid:   "a78cf3ea-b221-46f8-8cbc-a61e5e454e88",
			tid:    "tid_1",
			err:    processor.NotFoundError,
			status: 404,
		},
		{
			uuid:   "a78cf3ea-b221-46f8-8cbc-a61e5e454e88",
			tid:    "tid_1",
			err:    processor.InvalidContentTypeError,
			status: 422,
		},
	}

	requestProcessor := &DummyRequestProcessor{t: t}

	rh := requestHandler{
		requestProcessor: requestProcessor,
		log:              logger.NewUPPLogger("TEST", "PANIC"),
	}
	servicesRouter := mux.NewRouter()
	servicesRouter.HandleFunc("/{id}", rh.publishMessage).Methods("POST")

	r := http.NewServeMux()
	r.Handle("/", servicesRouter)

	server := httptest.NewServer(r)

	defer server.Close()

	for _, testCase := range tests {
		requestProcessor.uuid = testCase.uuid
		requestProcessor.tid = testCase.tid
		requestProcessor.err = testCase.err

		req, err := http.NewRequest("POST", fmt.Sprintf("%s/%s", server.URL, testCase.uuid), nil)
		require.NoError(t, err)

		req.Header.Add("X-Request-Id", testCase.tid)

		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		assert.NoError(t, resp.Body.Close())

		assert.Equal(t, testCase.status, resp.StatusCode)
	}
}

type DummyRequestProcessor struct {
	t    *testing.T
	uuid string
	tid  string
	err  error
}

func (p *DummyRequestProcessor) ForcePublication(uuid, tid string) error {
	assert.Equal(p.t, p.uuid, uuid)
	if p.tid == "" {
		assert.NotEmpty(p.t, tid)
	} else {
		assert.Equal(p.t, p.tid, tid)
	}
	return p.err
}
