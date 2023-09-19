package processor

import (
	"context"
	"fmt"
	"testing"

	"github.com/open-policy-agent/opa/rego"
	"github.com/stretchr/testify/assert"
)

func TestEvaluator_EvaluateMsgAccessLevel(t *testing.T) {
	defaultEvalQuery, err := rego.New(
		rego.Query("data.specialContent.msg"),
		rego.Load([]string{"../opa_modules/special_content.rego"}, nil),
	).PrepareForEval(context.TODO())
	assert.NoError(t, err)

	tests := []struct {
		name      string
		evalQuery rego.PreparedEvalQuery
		input     map[string]interface{}
		wantErr   assert.ErrorAssertionFunc
	}{
		{
			name:      "Basic special content message",
			evalQuery: defaultEvalQuery,
			input: map[string]interface{}{
				"EditorialDesk": "/FT/Professional/Central Banking",
			},
			wantErr: assert.Error,
		},
		{
			name:      "Basic non-special banking message",
			evalQuery: defaultEvalQuery,
			input: map[string]interface{}{
				"EditorialDesk": "/FT/Newsletters",
			},
			wantErr: assert.NoError,
		},
		{
			name:      "Missing editorial desk field",
			evalQuery: defaultEvalQuery,
			input:     map[string]interface{}{},
			wantErr:   assert.NoError,
		},
		{
			name:      "Empty UUID in message",
			evalQuery: defaultEvalQuery,
			input: map[string]interface{}{
				"UUID": "",
			},
			wantErr: assert.Error,
		},
		{
			name:      "Non-empty UUID in message",
			evalQuery: defaultEvalQuery,
			input: map[string]interface{}{
				"EditorialDesk": "/FT/Newsletters",
			},
			wantErr: assert.NoError,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := &Evaluator{
				evalQuery: &tt.evalQuery,
			}
			err = e.EvaluateMsgAccessLevel(tt.input)
			if !tt.wantErr(t, err, fmt.Sprintf("EvaluateMsgAccessLevel(%v)", tt.input)) {
				return
			}
		})
	}
}
