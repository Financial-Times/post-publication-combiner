package httputils

import (
	"fmt"
	"io"
	"net/http"
	"strings"
)

type Client interface {
	Do(req *http.Request) (*http.Response, error)
}

func ExecuteRequestForUUID(url string, uuid string, client Client) (b []byte, status int, err error) {
	if uuid != "" {
		url = strings.Replace(url, "{uuid}", uuid, -1)
	}

	return executeRequest(url, client)
}

func ExecuteRequest(url string, client Client) (b []byte, status int, err error) {
	return executeRequest(url, client)
}

func executeRequest(url string, client Client) (b []byte, status int, err error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, -1, fmt.Errorf("error creating request for url %q: %w", url, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, -1, fmt.Errorf("error executing request for url %q: %w", url, err)
	}

	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("request to %q failed with status: %d", url, resp.StatusCode)
	}

	b, err = io.ReadAll(resp.Body)
	if err != nil {
		return nil, http.StatusOK, fmt.Errorf("error parsing payload from response for url %q: %w", url, err)
	}

	return b, http.StatusOK, err
}
