// Command capitals demonstrates evaluating a simple function with the evals library.
package main

import (
	"context"
	"strings"

	evals "github.com/Kludex/pydantic-evals-go"
)

// matchAnswer scores 1.0 for an exact match, 0.8 for a case-insensitive
// substring match, and 0.0 otherwise.
type matchAnswer struct{}

func (matchAnswer) Evaluate(_ context.Context, ec *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	switch {
	case ec.Output == ec.ExpectedOutput:
		return evals.Score(1.0), nil
	case strings.Contains(strings.ToLower(ec.Output), strings.ToLower(ec.ExpectedOutput)):
		return evals.Score(0.8), nil
	default:
		return evals.Score(0.0), nil
	}
}

func main() {
	s := evals.For[string, string, any]()

	dataset := s.Dataset("capital_eval",
		s.Case("What is the capital of France?").Name("capital_question").Expect("Paris"),
	).With(s.IsInstance("string"), matchAnswer{})

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
