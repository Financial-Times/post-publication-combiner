package policy

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/Financial-Times/go-logger/v2"
	"github.com/Financial-Times/opa-client-go"
)

const (
	testDecisionID        = "1e58b3bf-995c-473e-90e9-ab1f10af74ab"
	contentUUID           = "e566c8bb-5ba1-42b5-a4a4-c5e5888369d9"
	errMsgЕditorialDeskCB = "editorialDesk: /FT/Professional/Central Banking not allowed because it is related to Central Banking"
)

func TestAgent_EvaluateContentPolicy(t *testing.T) {
	paths := map[string]string{
		KafkaIngestContent.String():  "kafka/ingest_content",
		KafkaIngestMetadata.String(): "kafka/ingest_metadata",
	}

	tests := []struct {
		name           string
		server         *httptest.Server
		query          map[string]interface{} // query field is informative and is not being used during tests evaluation
		policy         Policy
		expectedResult *ContentPolicyResult
		expectedError  error
	}{
		{
			name: "Evaluate a Skipping Kafka Ingest Content Policy Decision",
			server: createHTTPTestServer(
				t,
				fmt.Sprintf(
					`{"decision_id": %q, "result": {"skip": true, "reasons": [%q]}}`,
					testDecisionID,
					errMsgЕditorialDeskCB,
				),
			),
			query: map[string]interface{}{
				"editorialDesk": "/FT/Professional/Central Banking", // This is informative and is not being used during test evaluation
			},
			policy: KafkaIngestContent,
			expectedResult: &ContentPolicyResult{
				Skip:    true,
				Reasons: []string{errMsgЕditorialDeskCB},
			},
			expectedError: nil,
		},
		{
			name: "Evaluate a Non-Skipping Kafka Ingest Content Policy Decision",
			server: createHTTPTestServer(
				t,
				fmt.Sprintf(`{"decision_id": %q, "result": {"skip": false}}`, testDecisionID),
			),
			query: map[string]interface{}{
				"editorialDesk": "/FT/Pink", // This is informative and is not being used during test evaluation
			},
			policy: KafkaIngestContent,
			expectedResult: &ContentPolicyResult{
				Skip: false,
			},
			expectedError: nil,
		},
		{
			name: "Evaluate a Skipping Kafka Ingest Metadata Policy Decision",
			server: createHTTPTestServer(
				t,
				fmt.Sprintf(
					`{"decision_id": %q, "result": {"skip": true, "reasons": [%q]}}`,
					testDecisionID,
					errMsgЕditorialDeskCB,
				),
			),
			query: map[string]interface{}{
				"editorialDesk": "/FT/Professional/Central Banking", // This is informative and is not being used during test evaluation
			},
			policy: KafkaIngestMetadata,
			expectedResult: &ContentPolicyResult{
				Skip:    true,
				Reasons: []string{errMsgЕditorialDeskCB},
			},
			expectedError: nil,
		},
		{
			name: "Evaluate a Non-Skipping Kafka Ingest Metadata Policy Decision",
			server: createHTTPTestServer(
				t,
				fmt.Sprintf(`{"decision_id": %q, "result": {"skip": false}}`, testDecisionID),
			),
			query: map[string]interface{}{
				"editorialDesk": "/FT/Pink", // This is informative and is not being used during test evaluation
			},
			policy: KafkaIngestMetadata,
			expectedResult: &ContentPolicyResult{
				Skip: false,
			},
			expectedError: nil,
		},
		{
			name: "Evaluate and Receive an Error.",
			server: createHTTPTestServer(
				t,
				``,
			),
			query:          make(map[string]interface{}),
			policy:         KafkaIngestContent,
			expectedResult: nil,
			expectedError:  ErrEvaluatePolicy,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(_ *testing.T) {
			defer test.server.Close()

			l := logger.NewUPPLogger("post-publication-combiner", "INFO")
			c := opa.NewOpenPolicyAgentClient(test.server.URL, paths, opa.WithLogger(l))

			o := NewOpenPolicyAgent(c, l)

			result, err := o.EvaluateKafkaIngestPolicy(test.query, test.policy)

			if err != nil {
				if !errors.Is(err, test.expectedError) {
					t.Errorf(
						"Unexpected error received from call to EvaluateContentPolicy: %v",
						err,
					)
				}
			} else {
				assert.Equal(t, test.expectedResult, result)
			}
		})
	}
}

func createHTTPTestServer(t *testing.T, response string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte(response))
		if err != nil {
			t.Fatalf("could not write response from test http server: %v", err)
		}
	}))
}
