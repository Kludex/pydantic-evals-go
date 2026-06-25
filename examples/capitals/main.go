// Command capitals demonstrates evaluating a simple function with pydantic-evals-go.
package main

import (
	"context"
	"strings"

	evals "github.com/pydantic/pydantic-evals-go"
)

// matchAnswer scores 1.0 for an exact match, 0.8 for a case-insensitive
// substring match, and 0.0 otherwise.
type matchAnswer struct{}

func (matchAnswer) Evaluate(_ context.Context, ec *evals.EvaluatorContext[string, string, any]) (evals.EvaluatorOutput, error) {
	switch {
	case ec.Output == ec.ExpectedOutput:
		return evals.ScalarValue(evals.Float(1.0)), nil
	case strings.Contains(strings.ToLower(ec.Output), strings.ToLower(ec.ExpectedOutput)):
		return evals.ScalarValue(evals.Float(0.8)), nil
	default:
		return evals.ScalarValue(evals.Float(0.0)), nil
	}
}

func (matchAnswer) Spec() evals.EvaluatorSpec { return evals.NewSpec("MatchAnswer") }

func main() {
	c := evals.NewCase[string, string, any](
		"What is the capital of France?",
		evals.WithCaseName[string, string, any]("capital_question"),
		evals.WithExpectedOutput[string, string, any]("Paris"),
	)

	dataset, err := evals.NewDataset[string, string, any](
		"capital_eval",
		[]evals.Case[string, string, any]{c},
		evals.IsInstance[string, string, any]{TypeName: "string"},
		matchAnswer{},
	)
	if err != nil {
		panic(err)
	}

	answer := func(_ context.Context, question string) (string, error) {
		return "Paris", nil
	}

	report, err := dataset.Evaluate(context.Background(), answer)
	if err != nil {
		panic(err)
	}

	report.Print(evals.RenderOptions{
		IncludeInput:     true,
		IncludeOutput:    true,
		IncludeDurations: true,
		IncludeAverages:  true,
	})
}
