package processor

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

const (
	OpaContentMsgEvaluatorPackageName = "CONTENT_MSG_EVALUATOR"
	OpaVariableName                   = "failed_check_messages"
	pathPrefix                        = "v1/data"
)

type Evaluator struct {
	url        string
	paths      map[string]string
	httpClient *http.Client
}

type query struct {
	Input map[string]interface{} `json:"input"`
}

type decision struct {
	DecisionID string                 `json:"decision_id,omitempty"`
	Result     map[string]interface{} `json:"result"`
}

func CreateEvaluator(u string, p map[string]string, c *http.Client) *Evaluator {
	return &Evaluator{
		url:        u,
		paths:      p,
		httpClient: c,
	}
}

func (e *Evaluator) EvaluateMsgAccessLevel(query map[string]interface{}, pathKey, varName string) (string, error) {
	specialContentPath, ok := e.paths[pathKey]
	if !ok {
		return "", fmt.Errorf("key %s missing from path config supplied to the policy agent client", pathKey)
	}

	res, err := e.queryPolicyAgent(query, specialContentPath)
	if err != nil {
		return "", err
	}

	d := decision{}
	err = json.Unmarshal(res, &d)
	if err != nil {
		return "", err
	}

	decisionForKey, ok := d.Result[varName]
	if !ok {
		return "", fmt.Errorf("decision for key: %s is missing", varName)
	}

	decisions, ok := decisionForKey.([]interface{})
	if !ok {
		return "", fmt.Errorf("decision for key: %s can't be converted to array", varName)
	}
	if len(decisions) > 0 {
		return fmt.Sprintf("content will be skipped due to: %v", decisions), nil
	}

	return "", nil
}

func (e *Evaluator) queryPolicyAgent(input map[string]interface{}, path string) ([]byte, error) {
	q := query{
		Input: input,
	}

	m, err := json.Marshal(q)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("%s/%s/%s", e.url, pathPrefix, path),
		bytes.NewReader(m),
	)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	res, err := e.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("request returned non-200 response")
	}

	b, err := io.ReadAll(res.Body)
	if err != nil {
		return nil, err
	}

	return b, nil
}
