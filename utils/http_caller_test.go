package utils

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

type dummyClient struct {
	statusCode int
	body       string
	err        error
}

func (c *dummyClient) Do(*http.Request) (*http.Response, error) {
	resp := &http.Response{
		StatusCode: c.statusCode,
		Body:       io.NopCloser(strings.NewReader(c.body)),
	}

	return resp, c.err
}

func TestExecuteHTTPRequest(t *testing.T) {
	tests := []struct {
		dc            dummyClient
		url           string
		expRespBody   []byte
		expRespStatus int
		expErrStr     string
	}{
		{
			dc: dummyClient{
				body: "hey",
			},
			url:           "one malformed:url",
			expRespBody:   nil,
			expRespStatus: -1,
			expErrStr:     "error creating request for url \"one malformed:url\"",
		},
		{
			dc: dummyClient{
				err: fmt.Errorf("some error"),
			},
			url:           "url",
			expRespBody:   nil,
			expRespStatus: -1,
			expErrStr:     "error executing request for url \"url\": some error",
		},
		{
			dc: dummyClient{
				statusCode: http.StatusNotFound,
				body:       "simple body",
				err:        nil,
			},
			url:           "url",
			expRespBody:   nil,
			expRespStatus: http.StatusNotFound,
			expErrStr:     fmt.Sprintf("request to \"url\" failed with status: %d", http.StatusNotFound),
		},
		{
			dc: dummyClient{
				statusCode: http.StatusOK,
				body:       "simple body",
				err:        nil,
			},
			url:           "url",
			expRespBody:   []byte("simple body"),
			expRespStatus: http.StatusOK,
			expErrStr:     "",
		},
	}

	for _, testCase := range tests {
		b, s, err := executeHTTPRequest(testCase.url, &testCase.dc)

		if err != nil {
			assert.Contains(t, err.Error(), testCase.expErrStr)
		} else {
			assert.Empty(t, testCase.expErrStr)
		}

		assert.Equal(t, testCase.expRespBody, b)
		assert.Equal(t, testCase.expRespStatus, s)
	}
}
