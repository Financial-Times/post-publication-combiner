package policy

import (
	"errors"
	"fmt"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/opa-client-go"
)

var ErrEvaluatePolicy = errors.New("error evaluating policy")

type Policy int

const (
	KafkaIngestContent Policy = iota
	KafkaIngestMetadata
)

func (p Policy) String() string {
	switch p {
	case KafkaIngestContent:
		return "kafka_ingest_content"
	case KafkaIngestMetadata:
		return "kafka_ingest_metadata"
	}

	return ""
}

type ContentPolicyResult struct {
	Skip    bool     `json:"skip"`
	Reasons []string `json:"reasons"`
}

type Agent interface {
	EvaluateKafkaIngestPolicy(q map[string]interface{}, p Policy) (*ContentPolicyResult, error)
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

func (o *OpenPolicyAgent) EvaluateKafkaIngestPolicy(
	q map[string]interface{},
	p Policy,
) (*ContentPolicyResult, error) {
	r := &ContentPolicyResult{}

	decisionID, err := o.client.DoQuery(q, p.String(), r)
	if err != nil {
		return nil, fmt.Errorf("%w: Content Policy: %w", ErrEvaluatePolicy, err)
	}

	o.log.Infof(
		"Evaluated Kafka Ingest Policy: %s: decisionID: %q, result: %v",
		p.String(),
		decisionID,
		r,
	)

	return r, nil
}
