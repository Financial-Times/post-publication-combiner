package policy

import (
	"errors"
	"fmt"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/opa-client-go"
)

var ErrEvaluatePolicy = errors.New("error evaluating policy")

const (
	PackageName = "content_msg_evaluator"
)

type ContentPolicyResult struct {
	Skip    bool     `json:"skip"`
	Reasons []string `json:"reasons"`
}

type Agent interface {
	EvaluateContentPolicy(q map[string]interface{}) (*ContentPolicyResult, error)
}

type OpenPolicyAgent struct {
	client *opa.OpenPolicyAgentClient
	log    *logger.UPPLogger
}

func NewOpenPolicyAgent(c *opa.OpenPolicyAgentClient, l *logger.UPPLogger) *OpenPolicyAgent {
	return &OpenPolicyAgent{
		client: c,
		log:    l,
	}
}

func (o *OpenPolicyAgent) EvaluateContentPolicy(
	q map[string]interface{},
) (*ContentPolicyResult, error) {
	r := &ContentPolicyResult{}

	decisionID, err := o.client.DoQuery(q, PackageName, r)
	if err != nil {
		return nil, fmt.Errorf("%w: Content Policy: %w", ErrEvaluatePolicy, err)
	}

	o.log.Infof("Evaluated Content Policy: decisionID: %q, result: %v", decisionID, r)

	return r, nil
}
