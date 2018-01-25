package main

import (
	"errors"
	"fmt"
	"github.com/Financial-Times/post-publication-combiner/processor"
	"github.com/gorilla/mux"
	"github.com/stretchr/testify/assert"
	"net/http"
	"net/http/httptest"
	"testing"
	"github.com/Financial-Times/go-logger"
)

func TestPostMessage(t *testing.T) {

	tests := []struct {
		uuid   string
		tid    string
		err    error
		status int
	}{
		{"a78cf3ea-b221-46f8-8cbc-a61e5e454e88", "tid_1", nil, 200},
		{"a78cf3ea-b221-46f8-8cbc-a61e5e454e88", "", nil, 200},
		{"invalid", "tid_1", nil, 400},
		{"a78cf3ea-b221-46f8-8cbc-a61e5e454e88", "tid_1", errors.New("test error"), 500},
		{"a78cf3ea-b221-46f8-8cbc-a61e5e454e88", "tid_1", processor.NotFoundError, 404},
		{"a78cf3ea-b221-46f8-8cbc-a61e5e454e88", "tid_1", processor.InvalidContentTypeError, 422},
	}

	p := &DummyProcessor{t: t}
	logger.InitDefaultLogger("")
	rh := requestHandler{processor: p}
	servicesRouter := mux.NewRouter()
	servicesRouter.HandleFunc("/{id}", rh.postMessage).Methods("POST")

	r := http.NewServeMux()
	r.Handle("/", servicesRouter)

	server := httptest.NewServer(r)

	defer server.Close()

	for _, testCase := range tests {
		p.uuid = testCase.uuid
		p.tid = testCase.tid
		p.err = testCase.err

		req, err := http.NewRequest("POST", fmt.Sprintf("%s/%s", server.URL, testCase.uuid), nil)
		assert.NoError(t, err)
		req.Header.Add("X-Request-Id", testCase.tid)

		resp, err := http.DefaultClient.Do(req)
		assert.NoError(t, err)
		resp.Body.Close()

		assert.Equal(t, testCase.status, resp.StatusCode)
	}
}

type DummyProcessor struct {
	t    *testing.T
	uuid string
	tid  string
	err  error
}

func (p *DummyProcessor) ProcessMessages() {
	panic("implement me")
}

func (p *DummyProcessor) ForceMessagePublish(uuid, tid string) error {
	assert.Equal(p.t, p.uuid, uuid)
	assert.Equal(p.t, p.tid, tid)
	return p.err
}
