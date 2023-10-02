package processor

import (
	"context"
	"fmt"

	"github.com/open-policy-agent/opa/rego"
)

type Evaluator struct {
	evalQuery *rego.PreparedEvalQuery
}

func CreateEvaluator(query string, moduleLocation []string) (*Evaluator, error) {
	evalQuery, err := rego.New(
		rego.Query(query),
		rego.Load(moduleLocation, nil),
	).PrepareForEval(context.TODO())

	if err != nil {
		return nil, err
	}

	return &Evaluator{evalQuery: &evalQuery}, nil
}

func (e *Evaluator) EvaluateMsgAccessLevel(input map[string]interface{}) error {
	eval, err := e.evalQuery.Eval(context.TODO(), rego.EvalInput(input))
	if err != nil {
		return err
	}

	if eval[0].Expressions[0].Value != "" {
		return fmt.Errorf("%s", eval[0].Expressions[0].Value)
	}

	return nil
}
