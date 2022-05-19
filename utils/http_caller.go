package utils

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"strings"
)

type ApiURL struct {
	BaseURL  string
	Endpoint string
}

type Client interface {
	Do(req *http.Request) (*http.Response, error)
}

func ExecuteHTTPRequest(uuid string, apiUrl ApiURL, httpClient Client) (b []byte, status int, err error) {
	urlStr := apiUrl.BaseURL + apiUrl.Endpoint

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
		_, _ = io.Copy(ioutil.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, resp.StatusCode, fmt.Errorf("request to %q failed with status: %d", urlStr, resp.StatusCode)
	}

	b, err = ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, http.StatusOK, fmt.Errorf("error parsing payload from response for url %q: %w", urlStr, err)
	}

	return b, http.StatusOK, err
}
