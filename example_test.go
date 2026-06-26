package evals_test

import (
	"context"
	"fmt"
	"strings"

	evals "github.com/pydantic/pydantic-evals-go"
)

// renderOpts hides durations so example output is deterministic. The title is
// omitted because its centering depends on terminal width.
var renderOpts = evals.RenderOptions{
	IncludeInput:    true,
	IncludeOutput:   true,
	IncludeAverages: true,
	OmitTitle:       true,
}

// Example shows the shortest path to an evaluation: bind the types once with
// For, build a dataset, run a task, and print the report.
func Example() {
	s := evals.For[string, string, any]()

	dataset := s.Dataset("capitals",
		s.Case("What is the capital of France?").Name("france").Expect("Paris"),
	).With(s.EqualsExpected())

	answer := func(_ context.Context, q string) (string, error) {
		return "Paris", nil
	}

	report, err := dataset.Evaluate(context.Background(), answer)
	if err != nil {
		panic(err)
	}
	report.Print(renderOpts)
	// Output:
	// ┏━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━┳━━━━━━━━━━━━┓
	// ┃ Case ID  ┃ Inputs                         ┃ Outputs ┃ Assertions ┃
	// ┡━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━╇━━━━━━━━━━━━┩
	// │ france   │ What is the capital of France? │ Paris   │ ✔          │
	// ├──────────┼────────────────────────────────┼─────────┼────────────┤
	// │ Averages │                                │         │ 100.0% ✔   │
	// └──────────┴────────────────────────────────┴─────────┴────────────┘
}

// matchAnswer is a custom evaluator returning a graded score.
type matchAnswer struct{}

func (matchAnswer) Evaluate(_ context.Context, ec *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	switch {
	case ec.Output == ec.ExpectedOutput:
		return evals.Score(1.0), nil
	case strings.Contains(strings.ToLower(ec.Output), strings.ToLower(ec.ExpectedOutput)):
		return evals.Score(0.8).WithReason("substring match"), nil
	default:
		return evals.Score(0.0), nil
	}
}

// ExampleScore demonstrates a custom evaluator that returns a numeric score.
// The evaluator's report name defaults to its Go type name ("matchAnswer").
func ExampleScore() {
	s := evals.For[string, string, any]()
	dataset := s.Dataset("quiz",
		s.Case("capital of France?").Name("france").Expect("Paris"),
	).With(matchAnswer{})

	report, _ := dataset.Evaluate(context.Background(),
		func(_ context.Context, q string) (string, error) { return "Paris", nil })

	for _, c := range report.Cases {
		fmt.Printf("%s: %v\n", c.Name, c.Scores["matchAnswer"].Value)
	}
	// Output:
	// france: 1
}

// ExampleNamed shows one evaluator returning several named results at once.
func ExampleNamed() {
	type quality struct{}
	_ = quality{}

	eval := evals.For[string, string, any]()
	dataset := eval.Dataset("multi",
		eval.Case("hello").Name("c1"),
	).With(namedEvaluator{})

	report, _ := dataset.Evaluate(context.Background(),
		func(_ context.Context, in string) (string, error) { return in, nil })

	c := report.Cases[0]
	fmt.Println("score:", c.Scores["length"].Value)
	fmt.Println("label:", c.Labels["shape"].Value)
	// Output:
	// score: 5
	// label: short
}

type namedEvaluator struct{}

func (namedEvaluator) Evaluate(_ context.Context, ec *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	shape := "short"
	if len(ec.Output) > 10 {
		shape = "long"
	}
	return evals.Named(
		"length", evals.ScoreInt(len(ec.Output)),
		"shape", evals.Category(shape),
	), nil
}

// ExampleIncrementMetric records a metric from inside the task; it surfaces on
// the report and is averaged across cases.
func ExampleIncrementMetric() {
	s := evals.For[string, string, any]()
	dataset := s.Dataset("metered", s.Case("hello").Name("c1"))

	report, _ := dataset.Evaluate(context.Background(),
		func(ctx context.Context, in string) (string, error) {
			evals.IncrementMetric(ctx, "chars", float64(len(in)))
			return in, nil
		})

	fmt.Println("chars:", report.Cases[0].Metrics["chars"])
	// Output:
	// chars: 5
}

// ExampleDataset_Save serializes a dataset to YAML, using the short form for
// each evaluator.
func ExampleDataset_Save() {
	s := evals.For[string, string, any]()
	dataset := s.Dataset("greetings",
		s.Case("hi").Name("c1").Expect("hello"),
	).With(s.EqualsExpected(), s.IsInstance("string"))

	out, _ := dataset.Save(evals.SaveOptions{Format: "yaml"})
	fmt.Print(string(out))
	// Output:
	// name: greetings
	// cases:
	//   - name: c1
	//     inputs: hi
	//     expected_output: hello
	// evaluators:
	//   - EqualsExpected
	//   - IsInstance: string
}

// ExampleLoadDataset rebuilds a dataset from YAML through a registry of the
// built-in evaluators.
func ExampleLoadDataset() {
	data := []byte(`
name: greetings
cases:
  - name: c1
    inputs: hi
    expected_output: hello
evaluators:
  - EqualsExpected
`)

	reg := evals.NewRegistry[string, string, any]()
	reg.RegisterDefaults()

	dataset, err := evals.LoadDataset(data, reg, evals.LoadOptions[string, string, any]{})
	if err != nil {
		panic(err)
	}
	fmt.Printf("%s: %d case(s), %d evaluator(s)\n", dataset.Name, len(dataset.Cases), len(dataset.Evaluators))
	// Output:
	// greetings: 1 case(s), 1 evaluator(s)
}

// ExampleConfig_repeat runs each case several times and reads the aggregate.
func ExampleConfig_repeat() {
	s := evals.For[string, string, any]()
	dataset := s.Dataset("repeated",
		s.Case("hi").Name("c1").Expect("hi"),
	).With(s.EqualsExpected())

	report, _ := dataset.Evaluate(context.Background(),
		func(_ context.Context, in string) (string, error) { return in, nil },
		evals.Config{Repeat: 3})

	groups := report.CaseGroups()
	fmt.Printf("%d run(s) of %q\n", len(groups[0].Runs), groups[0].Name)
	// Output:
	// 3 run(s) of "c1"
}
