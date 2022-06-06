package httputils

import (
	"fmt"
	"io"
	"net/http"
)

type Client interface {
	Do(req *http.Request) (*http.Response, error)
}

type StatusCodeError struct {
	url        string
	StatusCode int
}

func (e *StatusCodeError) Error() string {
	return fmt.Sprintf("request to %q failed with status: %d", e.url, e.StatusCode)
}

func ExecuteRequest(url string, client Client) ([]byte, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("error creating request for url %q: %w", url, err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("error executing request for url %q: %w", url, err)
	}

	defer func() {
		_, _ = io.Copy(io.Discard, resp.Body)
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		return nil, &StatusCodeError{
			url:        url,
			StatusCode: resp.StatusCode,
		}
	}

	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error parsing payload from response for url %q: %w", url, err)
	}

	return b, nil
}
