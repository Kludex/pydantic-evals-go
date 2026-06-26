package evals_test

import (
	"context"
	"strings"
	"testing"
	"time"

	evals "github.com/Kludex/pydantic-evals-go"
)

// numScorer is a minimal named evaluator that returns a fixed Output, used to
// drive number/duration formatting through Render via the public construction
// helpers (Score, ScoreInt, Assertion, Category, Named).
type numScorer struct {
	name string
	out  evals.Output
}

func (e numScorer) Evaluate(_ context.Context, _ *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	return e.out, nil
}

func (e numScorer) EvaluationName() string { return e.name }

// numEcho is the trivial task used by formatting tests: it returns its input.
func numEcho(_ context.Context, in string) (string, error) { return in, nil }

// numRender runs a single-case dataset with the given evaluators and renders the
// report with averages included.
func numRender(t *testing.T, evaluators ...evals.Evaluator[string, string, any]) string {
	t.Helper()
	return numRenderTask(t, numEcho, evaluators...)
}

func numRenderTask(t *testing.T, task evals.TaskFunc[string, string], evaluators ...evals.Evaluator[string, string, any]) string {
	t.Helper()
	s := evals.For[string, string, any]()
	ds := s.Dataset("nums", s.Case("a").Name("c1").Eval(evaluators...))
	rep, err := ds.Evaluate(context.Background(), task, evals.Config{TaskName: "task"})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	return rep.Render(evals.RenderOptions{IncludeAverages: true})
}

// TestNumbersIntegerScoreNoDecimals checks ScoreInt renders without decimals in
// the case row, while the aggregate (Averages) row always renders as a float.
func TestNumbersIntegerScoreNoDecimals(t *testing.T) {
	got := numRender(t, numScorer{name: "int_ev", out: evals.ScoreInt(42)})
	want := ` Evaluation Summary: task
┏━━━━━━━━━━┳━━━━━━━━━━━━━━┓
┃ Case ID  ┃ Scores       ┃
┡━━━━━━━━━━╇━━━━━━━━━━━━━━┩
│ c1       │ int_ev: 42   │
├──────────┼──────────────┤
│ Averages │ int_ev: 42.0 │
└──────────┴──────────────┘`
	if got != want {
		t.Fatalf("integer score rendering mismatch.\n got:\n%s\nwant:\n%s", got, want)
	}
}

// TestNumbersFloatScoreFormatting checks the float formatting rules from
// render_numbers.py: at least one decimal and at least three significant figures.
func TestNumbersFloatScoreFormatting(t *testing.T) {
	got := numRender(t,
		numScorer{name: "a_sig", out: evals.Score(12.0)},
		numScorer{name: "b_big", out: evals.Score(123.0)},
		numScorer{name: "c_half", out: evals.Score(0.5)},
		numScorer{name: "d_small", out: evals.Score(0.05)},
		numScorer{name: "e_tiny", out: evals.Score(0.005)},
		numScorer{name: "f_zero", out: evals.Score(0.0)},
		numScorer{name: "g_clamp", out: evals.Score(1234.5)},
	)
	want := `   Evaluation Summary: task
┏━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━┓
┃ Case ID  ┃ Scores           ┃
┡━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━┩
│ c1       │ a_sig: 12.0      │
│          │ b_big: 123.0     │
│          │ c_half: 0.500    │
│          │ d_small: 0.0500  │
│          │ e_tiny: 0.00500  │
│          │ f_zero: 0.000    │
│          │ g_clamp: 1,234.5 │
├──────────┼──────────────────┤
│ Averages │ a_sig: 12.0      │
│          │ b_big: 123.0     │
│          │ c_half: 0.500    │
│          │ d_small: 0.0500  │
│          │ e_tiny: 0.00500  │
│          │ f_zero: 0.000    │
│          │ g_clamp: 1,234.5 │
└──────────┴──────────────────┘`
	if got != want {
		t.Fatalf("float score rendering mismatch.\n got:\n%s\nwant:\n%s", got, want)
	}
}

// TestNumbersLargeWholeMetricThousands checks a large whole metric value gets
// thousands separators in the case row, and renders as a float in the aggregate.
func TestNumbersLargeWholeMetricThousands(t *testing.T) {
	task := func(ctx context.Context, in string) (string, error) {
		evals.IncrementMetric(ctx, "tokens", 1234567)
		return in, nil
	}
	got := numRenderTask(t, task, numScorer{name: "sc", out: evals.Score(1.5)})
	want := `          Evaluation Summary: task
┏━━━━━━━━━━┳━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━━┓
┃ Case ID  ┃ Scores   ┃ Metrics             ┃
┡━━━━━━━━━━╇━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━━━┩
│ c1       │ sc: 1.50 │ tokens: 1,234,567   │
├──────────┼──────────┼─────────────────────┤
│ Averages │ sc: 1.50 │ tokens: 1,234,567.0 │
└──────────┴──────────┴─────────────────────┘`
	if got != want {
		t.Fatalf("large whole metric rendering mismatch.\n got:\n%s\nwant:\n%s", got, want)
	}
}

// TestNumbersThousandsEdgeCases exercises withThousands branches not covered by
// the 7-digit case: a value whose integer part length is a multiple of three
// (lead == 0) and negative values (both int and float scalars), via a directly
// constructed report whose aggregate forces float rendering.
func TestNumbersThousandsEdgeCases(t *testing.T) {
	rep := &evals.EvaluationReport[string, string, any]{
		Name: "exp",
		Cases: []evals.ReportCase[string, string, any]{
			{
				Name:   "c1",
				Output: "x",
				Scores: map[string]evals.EvaluationResult{
					"lead0":     {Name: "lead0", Value: evals.Int(123456)},
					"neg_int":   {Name: "neg_int", Value: evals.Int(-1234567)},
					"neg_float": {Name: "neg_float", Value: evals.Float(-1234.5)},
				},
				Metrics: map[string]float64{
					"lead0_metric": 123456,
					"neg_metric":   -1234567,
				},
			},
		},
	}
	got := rep.Render(evals.RenderOptions{IncludeAverages: true})
	want := `                    Evaluation Summary: exp
┏━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━━━━━━━┓
┃ Case ID  ┃ Scores                ┃ Metrics                  ┃
┡━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━━━━━━━━┩
│ c1       │ lead0: 123,456        │ lead0_metric: 123,456    │
│          │ neg_float: -1,234.5   │ neg_metric: -1,234,567   │
│          │ neg_int: -1,234,567   │                          │
├──────────┼───────────────────────┼──────────────────────────┤
│ Averages │ lead0: 123,456.0      │ lead0_metric: 123,456.0  │
│          │ neg_float: -1,234.5   │ neg_metric: -1,234,567.0 │
│          │ neg_int: -1,234,567.0 │                          │
└──────────┴───────────────────────┴──────────────────────────┘`
	if got != want {
		t.Fatalf("thousands edge-case rendering mismatch.\n got:\n%s\nwant:\n%s", got, want)
	}
}

// TestNumbersAssertionPercentage checks the aggregate assertion pass-rate renders
// as a percentage with one decimal place (formatPercentage).
func TestNumbersAssertionPercentage(t *testing.T) {
	s := evals.For[string, string, any]()
	passEval := evals.Evaluator[string, string, any](numAssertOnGood{})
	ds := s.Dataset("a",
		s.Case("good").Name("g").Eval(passEval),
		s.Case("bad").Name("b").Eval(passEval),
	)
	rep, err := ds.Evaluate(context.Background(), numEcho, evals.Config{TaskName: "task"})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	got := rep.Render(evals.RenderOptions{IncludeAverages: true})
	want := `Evaluation Summary: task
┏━━━━━━━━━━┳━━━━━━━━━━━━┓
┃ Case ID  ┃ Assertions ┃
┡━━━━━━━━━━╇━━━━━━━━━━━━┩
│ g        │ ✔          │
├──────────┼────────────┤
│ b        │ ✗          │
├──────────┼────────────┤
│ Averages │ 50.0% ✔    │
└──────────┴────────────┘`
	if got != want {
		t.Fatalf("assertion percentage rendering mismatch.\n got:\n%s\nwant:\n%s", got, want)
	}
}

// numAssertOnGood passes when the input equals "good".
type numAssertOnGood struct{}

func (numAssertOnGood) Evaluate(_ context.Context, ec *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	return evals.Assertion(ec.Inputs == "good"), nil
}

func (numAssertOnGood) EvaluationName() string { return "passes" }

// TestNumbersDurationUnits covers the µs/ms/s branches of formatDuration via a
// directly constructed report, including the µs precision split at value 1.
func TestNumbersDurationUnits(t *testing.T) {
	cases := []struct {
		name string
		task time.Duration
		want string
	}{
		{"microseconds_ge_one", 2 * time.Microsecond, "task: 2µs"},
		{"microseconds_lt_one", 500 * time.Nanosecond, "task: 0.5µs"},
		{"milliseconds", 1500 * time.Microsecond, "task: 1.5ms"},
		{"seconds", 1500 * time.Millisecond, "task: 1.5s"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rep := &evals.EvaluationReport[string, string, any]{
				Name: "exp",
				Cases: []evals.ReportCase[string, string, any]{
					{
						Name:          "c1",
						Output:        "x",
						Scores:        map[string]evals.EvaluationResult{"s": {Name: "s", Value: evals.Float(1)}},
						TaskDuration:  tc.task,
						TotalDuration: tc.task,
					},
				},
			}
			got := rep.Render(evals.RenderOptions{IncludeDurations: true, IncludeTotalDuration: true})
			if !strings.Contains(got, tc.want) {
				t.Fatalf("expected duration substring %q in:\n%s", tc.want, got)
			}
		})
	}
}

// TestNumbersZeroDuration covers the "0s" branch of formatDuration. A
// zero-duration aggregate is reached by constructing a report whose only case has
// TaskDuration and TotalDuration of zero; the aggregate averages to zero too.
func TestNumbersZeroDuration(t *testing.T) {
	rep := &evals.EvaluationReport[string, string, any]{
		Name: "exp",
		Cases: []evals.ReportCase[string, string, any]{
			{
				Name:          "c1",
				Output:        "out",
				Scores:        map[string]evals.EvaluationResult{"s": {Name: "s", Value: evals.Float(0.5)}},
				Metrics:       map[string]float64{"m": 1234567},
				Assertions:    map[string]evals.EvaluationResult{"a": {Name: "a", Value: evals.Bool(true)}},
				TaskDuration:  0,
				TotalDuration: 0,
			},
		},
	}
	got := rep.Render(evals.RenderOptions{IncludeDurations: true, IncludeTotalDuration: true, IncludeAverages: true})
	want := `                     Evaluation Summary: exp
┏━━━━━━━━━━┳━━━━━━━━━━┳━━━━━━━━━━━━━━━━┳━━━━━━━━━━━━┳━━━━━━━━━━━┓
┃ Case ID  ┃ Scores   ┃ Metrics        ┃ Assertions ┃ Durations ┃
┡━━━━━━━━━━╇━━━━━━━━━━╇━━━━━━━━━━━━━━━━╇━━━━━━━━━━━━╇━━━━━━━━━━━┩
│ c1       │ s: 0.500 │ m: 1,234,567   │ ✔          │  task: 0s │
│          │          │                │            │ total: 0s │
├──────────┼──────────┼────────────────┼────────────┼───────────┤
│ Averages │ s: 0.500 │ m: 1,234,567.0 │ 100.0% ✔   │  task: 0s │
│          │          │                │            │ total: 0s │
└──────────┴──────────┴────────────────┴────────────┴───────────┘`
	if got != want {
		t.Fatalf("zero-duration rendering mismatch.\n got:\n%s\nwant:\n%s", got, want)
	}
}

// TestNumbersZeroDurationTaskOnly covers the task-only duration column ("0s",
// without the "task:"/"total:" prefixes).
func TestNumbersZeroDurationTaskOnly(t *testing.T) {
	rep := &evals.EvaluationReport[string, string, any]{
		Name: "exp",
		Cases: []evals.ReportCase[string, string, any]{
			{
				Name:         "c1",
				Output:       "x",
				Scores:       map[string]evals.EvaluationResult{"s": {Name: "s", Value: evals.Float(1)}},
				TaskDuration: 0,
			},
		},
	}
	got := rep.Render(evals.RenderOptions{IncludeDurations: true, IncludeAverages: true})
	want := `     Evaluation Summary: exp
┏━━━━━━━━━━┳━━━━━━━━━┳━━━━━━━━━━┓
┃ Case ID  ┃ Scores  ┃ Duration ┃
┡━━━━━━━━━━╇━━━━━━━━━╇━━━━━━━━━━┩
│ c1       │ s: 1.00 │       0s │
├──────────┼─────────┼──────────┤
│ Averages │ s: 1.00 │       0s │
└──────────┴─────────┴──────────┘`
	if got != want {
		t.Fatalf("task-only zero-duration rendering mismatch.\n got:\n%s\nwant:\n%s", got, want)
	}
}
