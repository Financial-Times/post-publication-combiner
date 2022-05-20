package utils

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Client interface {
	Do(req *http.Request) (*http.Response, error)
}

func ExecuteHTTPRequest(uuid string, urlStr string, httpClient Client) (b []byte, status int, err error) {
	if uuid != "" {
		urlStr = strings.Replace(urlStr, "{uuid}", uuid, -1)
	}

	return executeHTTPRequest(urlStr, httpClient)
}

func ExecuteSimpleHTTPRequest(urlStr string, httpClient Client) (b []byte, status int, err error) {
	return executeHTTPRequest(urlStr, httpClient)
}

func executeHTTPRequest(urlStr string, httpClient Client) (b []byte, status int, err error) {
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, -1, fmt.Errorf("error creating request for url %q: %w", urlStr, err)
	}

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, -1, fmt.Errorf("error executing request for url %q: %w", urlStr, err)
	}

	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("request to %q failed with status: %d", urlStr, resp.StatusCode)
	}

	b, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, http.StatusOK, fmt.Errorf("error parsing payload from response for url %q: %w", urlStr, err)
	}

	return b, http.StatusOK, err
}
