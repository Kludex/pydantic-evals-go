package evals

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

func assertRender(t *testing.T, got, want string) {
	t.Helper()
	if got != want {
		t.Errorf("rendered table mismatch\n--- got ---\n%s\n--- want ---\n%s\n", got, want)
	}
}

// scoreCase builds a single-score, single-assertion report case with a fixed task
// duration so rendered tables are deterministic.
func scoreCase() ReportCase[string, string, any] {
	return ReportCase[string, string, any]{
		Name:         "simple_case",
		Inputs:       "What is the capital of France?",
		Output:       "Paris",
		Scores:       map[string]EvaluationResult{"MyEvaluator": {Name: "MyEvaluator", Value: Float(1.0)}},
		Assertions:   map[string]EvaluationResult{"IsInstance": {Name: "IsInstance", Value: Bool(true)}},
		TaskDuration: 1500 * time.Millisecond,
	}
}

func TestDefaultRenderOptions(t *testing.T) {
	o := DefaultRenderOptions()
	want := RenderOptions{IncludeDurations: true, IncludeAverages: true}
	if o != want {
		t.Fatalf("DefaultRenderOptions() = %+v, want %+v", o, want)
	}
}

func TestRenderDefault(t *testing.T) {
	r := &EvaluationReport[string, string, any]{Name: "task", Cases: []ReportCase[string, string, any]{scoreCase()}}

	want := `                 Evaluation Summary: task
┏━━━━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━━━┳━━━━━━━━━━┓
┃ Case ID     ┃ Scores            ┃ Assertions ┃ Duration ┃
┡━━━━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━━━━╇━━━━━━━━━━┩
│ simple_case │ MyEvaluator: 1.00 │ ✔          │     1.5s │
├─────────────┼───────────────────┼────────────┼──────────┤
│ Averages    │ MyEvaluator: 1.00 │ 100.0% ✔   │     1.5s │
└─────────────┴───────────────────┴────────────┴──────────┘`

	assertRender(t, r.Render(), want)
}

func TestRenderIncludeInputOutputExpectedMetadata(t *testing.T) {
	c := scoreCase()
	c.HasExpectedOutput = true
	c.ExpectedOutput = "Paris"
	c.HasMetadata = true
	c.Metadata = "fr"
	r := &EvaluationReport[string, string, any]{Name: "task", Cases: []ReportCase[string, string, any]{c}}

	want := `                                                     Evaluation Summary: task
┏━━━━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━┳━━━━━━━━━━━━━━━━━┳━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━━━┳━━━━━━━━━━┓
┃ Case ID     ┃ Inputs                         ┃ Metadata ┃ Expected Output ┃ Outputs ┃ Scores            ┃ Assertions ┃ Duration ┃
┡━━━━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━━╇━━━━━━━━━━━━━━━━━╇━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━━━━╇━━━━━━━━━━┩
│ simple_case │ What is the capital of France? │ fr       │ Paris           │ Paris   │ MyEvaluator: 1.00 │ ✔          │     1.5s │
├─────────────┼────────────────────────────────┼──────────┼─────────────────┼─────────┼───────────────────┼────────────┼──────────┤
│ Averages    │                                │          │                 │         │ MyEvaluator: 1.00 │ 100.0% ✔   │     1.5s │
└─────────────┴────────────────────────────────┴──────────┴─────────────────┴─────────┴───────────────────┴────────────┴──────────┘`

	got := r.Render(RenderOptions{
		IncludeInput:          true,
		IncludeMetadata:       true,
		IncludeExpectedOutput: true,
		IncludeOutput:         true,
		IncludeDurations:      true,
		IncludeAverages:       true,
	})
	assertRender(t, got, want)
}

func TestRenderMissingMetadataAndExpectedShowDash(t *testing.T) {
	r := &EvaluationReport[string, string, any]{Name: "task", Cases: []ReportCase[string, string, any]{scoreCase()}}

	want := `                          Evaluation Summary: task
┏━━━━━━━━━━━━━┳━━━━━━━━━━┳━━━━━━━━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━━━┓
┃ Case ID     ┃ Metadata ┃ Expected Output ┃ Scores            ┃ Assertions ┃
┡━━━━━━━━━━━━━╇━━━━━━━━━━╇━━━━━━━━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━━━━┩
│ simple_case │ -        │ -               │ MyEvaluator: 1.00 │ ✔          │
└─────────────┴──────────┴─────────────────┴───────────────────┴────────────┘`

	got := r.Render(RenderOptions{IncludeMetadata: true, IncludeExpectedOutput: true})
	assertRender(t, got, want)
}

func TestRenderIncludeReasonsScoresLabelsAssertions(t *testing.T) {
	c := ReportCase[string, string, any]{
		Name:   "case_1",
		Scores: map[string]EvaluationResult{"score": {Name: "score", Value: Float(0.5), Reason: "partial"}},
		Labels: map[string]EvaluationResult{"label": {Name: "label", Value: Label("good"), Reason: "looks ok"}},
		Assertions: map[string]EvaluationResult{
			"assert": {Name: "assert", Value: Bool(true), Reason: "matched"},
		},
		TaskDuration: 12 * time.Millisecond,
	}
	r := &EvaluationReport[string, string, any]{Name: "task", Cases: []ReportCase[string, string, any]{c}}

	want := `                        Evaluation Summary: task
┏━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━┓
┃ Case ID ┃ Scores            ┃ Labels             ┃ Assertions        ┃
┡━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━┩
│ case_1  │ score: 0.500      │ label: good        │ assert: ✔         │
│         │   Reason: partial │   Reason: looks ok │   Reason: matched │
│         │                   │                    │                   │
│         │                   │                    │                   │
└─────────┴───────────────────┴────────────────────┴───────────────────┘`

	got := r.Render(RenderOptions{IncludeReasons: true})
	assertRender(t, got, want)
}

func TestRenderIncludeTotalDuration(t *testing.T) {
	c := scoreCase()
	c.TaskDuration = 12 * time.Millisecond
	c.TotalDuration = 250 * time.Millisecond
	r := &EvaluationReport[string, string, any]{Name: "task", Cases: []ReportCase[string, string, any]{c}}

	want := `                    Evaluation Summary: task
┏━━━━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━━━┳━━━━━━━━━━━━━━━━┓
┃ Case ID     ┃ Scores            ┃ Assertions ┃      Durations ┃
┡━━━━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━━━━╇━━━━━━━━━━━━━━━━┩
│ simple_case │ MyEvaluator: 1.00 │ ✔          │   task: 12.0ms │
│             │                   │            │ total: 250.0ms │
├─────────────┼───────────────────┼────────────┼────────────────┤
│ Averages    │ MyEvaluator: 1.00 │ 100.0% ✔   │   task: 12.0ms │
│             │                   │            │ total: 250.0ms │
└─────────────┴───────────────────┴────────────┴────────────────┘`

	got := r.Render(RenderOptions{IncludeDurations: true, IncludeTotalDuration: true, IncludeAverages: true})
	assertRender(t, got, want)
}

func TestRenderMetricsColumn(t *testing.T) {
	c := ReportCase[string, string, any]{
		Name:         "case_1",
		Metrics:      map[string]float64{"tokens": 42, "cost": 0.5},
		TaskDuration: 1500 * time.Millisecond,
	}
	r := &EvaluationReport[string, string, any]{Name: "task", Cases: []ReportCase[string, string, any]{c}}

	want := `       Evaluation Summary: task
┏━━━━━━━━━━┳━━━━━━━━━━━━━━┳━━━━━━━━━━┓
┃ Case ID  ┃ Metrics      ┃ Duration ┃
┡━━━━━━━━━━╇━━━━━━━━━━━━━━╇━━━━━━━━━━┩
│ case_1   │ cost: 0.500  │     1.5s │
│          │ tokens: 42   │          │
├──────────┼──────────────┼──────────┤
│ Averages │ cost: 0.500  │     1.5s │
│          │ tokens: 42.0 │          │
└──────────┴──────────────┴──────────┘`

	assertRender(t, r.Render(), want)
}

func TestRenderLabelDistributionInAverages(t *testing.T) {
	c1 := ReportCase[string, string, any]{
		Name:         "c1",
		Labels:       map[string]EvaluationResult{"quality": {Name: "quality", Value: Label("good")}},
		TaskDuration: 1500 * time.Millisecond,
	}
	c2 := ReportCase[string, string, any]{
		Name:         "c2",
		Labels:       map[string]EvaluationResult{"quality": {Name: "quality", Value: Label("bad")}},
		TaskDuration: 1500 * time.Millisecond,
	}
	r := &EvaluationReport[string, string, any]{Name: "task", Cases: []ReportCase[string, string, any]{c1, c2}}

	want := `                 Evaluation Summary: task
┏━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━┓
┃ Case ID  ┃ Labels                           ┃ Duration ┃
┡━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━━┩
│ c1       │ quality: good                    │     1.5s │
├──────────┼──────────────────────────────────┼──────────┤
│ c2       │ quality: bad                     │     1.5s │
├──────────┼──────────────────────────────────┼──────────┤
│ Averages │ quality: bad: 50.0%, good: 50.0% │     1.5s │
└──────────┴──────────────────────────────────┴──────────┘`

	assertRender(t, r.Render(), want)
}

func TestRenderEvaluatorFailuresColumn(t *testing.T) {
	c := ReportCase[string, string, any]{
		Name:         "case_1",
		Scores:       map[string]EvaluationResult{"ok": {Name: "ok", Value: Float(1.0)}},
		TaskDuration: 1500 * time.Millisecond,
		EvaluatorFailures: []EvaluatorFailure{
			{Name: "Boom", ErrorMessage: "kaboom"},
			{Name: "Silent"},
		},
	}
	r := &EvaluationReport[string, string, any]{Name: "task", Cases: []ReportCase[string, string, any]{c}}

	want := `               Evaluation Summary: task
┏━━━━━━━━━━┳━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━┓
┃ Case ID  ┃ Scores   ┃ Evaluator Failures ┃ Duration ┃
┡━━━━━━━━━━╇━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━━┩
│ case_1   │ ok: 1.00 │ Boom: kaboom       │     1.5s │
│          │          │ Silent             │          │
├──────────┼──────────┼────────────────────┼──────────┤
│ Averages │ ok: 1.00 │                    │     1.5s │
└──────────┴──────────┴────────────────────┴──────────┘`

	assertRender(t, r.Render(), want)
}

// TestRenderEvaluatorFailuresViaEvaluate drives an evaluator that errors through
// the public Evaluate path, rendering with the duration column disabled so the
// non-deterministic timings do not appear in the asserted string.
func TestRenderEvaluatorFailuresViaEvaluate(t *testing.T) {
	ds, err := NewDataset[string, string, any](
		"task",
		[]Case[string, string, any]{NewCase[string, string, any]("in", WithCaseName[string, string, any]("only"))},
		boomEvaluator{},
	)
	if err != nil {
		t.Fatal(err)
	}
	report, err := ds.Evaluate(context.Background(), func(_ context.Context, in string) (string, error) {
		return in, nil
	})
	if err != nil {
		t.Fatal(err)
	}

	want := `    Evaluation Summary: task
┏━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━┓
┃ Case ID ┃ Evaluator Failures ┃
┡━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━━┩
│ only    │ Boom: it failed    │
└─────────┴────────────────────┘`

	assertRender(t, report.Render(RenderOptions{}), want)
}

type boomEvaluator struct{}

func (boomEvaluator) Evaluate(_ context.Context, _ *EvaluatorContext[string, string, any]) (EvaluatorOutput, error) {
	return nil, errors.New("it failed")
}
func (boomEvaluator) Spec() EvaluatorSpec { return NewSpec("Boom") }

func TestRenderTitleOverride(t *testing.T) {
	r := &EvaluationReport[string, string, any]{Name: "task", Cases: []ReportCase[string, string, any]{scoreCase()}}

	want := `                       Custom Title
┏━━━━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━━━┳━━━━━━━━━━┓
┃ Case ID     ┃ Scores            ┃ Assertions ┃ Duration ┃
┡━━━━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━━━━╇━━━━━━━━━━┩
│ simple_case │ MyEvaluator: 1.00 │ ✔          │     1.5s │
└─────────────┴───────────────────┴────────────┴──────────┘`

	got := r.Render(RenderOptions{IncludeDurations: true, Title: "Custom Title"})
	assertRender(t, got, want)
}

func TestRenderOmitTitle(t *testing.T) {
	r := &EvaluationReport[string, string, any]{Name: "task", Cases: []ReportCase[string, string, any]{scoreCase()}}

	want := `┏━━━━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━━━┳━━━━━━━━━━┓
┃ Case ID     ┃ Scores            ┃ Assertions ┃ Duration ┃
┡━━━━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━━━━╇━━━━━━━━━━┩
│ simple_case │ MyEvaluator: 1.00 │ ✔          │     1.5s │
└─────────────┴───────────────────┴────────────┴──────────┘`

	got := r.Render(RenderOptions{IncludeDurations: true, OmitTitle: true, Title: "ignored"})
	assertRender(t, got, want)
}

func TestRenderIncludeAveragesFalse(t *testing.T) {
	r := &EvaluationReport[string, string, any]{Name: "task", Cases: []ReportCase[string, string, any]{scoreCase()}}

	want := `                 Evaluation Summary: task
┏━━━━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━━━┳━━━━━━━━━━┓
┃ Case ID     ┃ Scores            ┃ Assertions ┃ Duration ┃
┡━━━━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━━━━╇━━━━━━━━━━┩
│ simple_case │ MyEvaluator: 1.00 │ ✔          │     1.5s │
└─────────────┴───────────────────┴────────────┴──────────┘`

	got := r.Render(RenderOptions{IncludeDurations: true, IncludeAverages: false})
	assertRender(t, got, want)
}

func TestRenderEmptyReport(t *testing.T) {
	r := &EvaluationReport[string, string, any]{Name: "task"}

	want := `Evaluation Summary: task
┏━━━━━━━━━┳━━━━━━━━━━┓
┃ Case ID ┃ Duration ┃
┡━━━━━━━━━╇━━━━━━━━━━┩
└─────────┴──────────┘`

	assertRender(t, r.Render(), want)
}

func TestRenderEmptyReportWithColumns(t *testing.T) {
	r := &EvaluationReport[string, string, any]{Name: "task"}

	want := `Evaluation Summary: task
┏━━━━━━━━━┳━━━━━━━━┓
┃ Case ID ┃ Inputs ┃
┡━━━━━━━━━╇━━━━━━━━┩
└─────────┴────────┘`

	got := r.Render(RenderOptions{IncludeInput: true})
	assertRender(t, got, want)
}

func TestRenderMapInputsAndOutputDeterministicKeyOrdering(t *testing.T) {
	c := ReportCase[map[string]any, map[string]any, any]{
		Name:         "m",
		Inputs:       map[string]any{"b": 2, "a": 1},
		Output:       map[string]any{"z": "last", "a": "first"},
		TaskDuration: 1500 * time.Millisecond,
	}
	r := &EvaluationReport[map[string]any, map[string]any, any]{
		Name:  "task",
		Cases: []ReportCase[map[string]any, map[string]any, any]{c},
	}

	want := `                 Evaluation Summary: task
┏━━━━━━━━━┳━━━━━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━┓
┃ Case ID ┃ Inputs       ┃ Outputs             ┃ Duration ┃
┡━━━━━━━━━╇━━━━━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━━┩
│ m       │ {a: 1, b: 2} │ {a: first, z: last} │     1.5s │
└─────────┴──────────────┴─────────────────────┴──────────┘`

	got := r.Render(RenderOptions{IncludeInput: true, IncludeOutput: true, IncludeDurations: true})
	assertRender(t, got, want)
}

// TestRenderDocsExample reproduces the README example end-to-end through the
// public Evaluate API and verifies the score, averages and assertion cells. The
// report name defaults to the task name ("task"), and durations are omitted so
// the asserted table is deterministic.
func TestRenderDocsExample(t *testing.T) {
	c := NewCase[string, string, any](
		"What is the capital of France?",
		WithCaseName[string, string, any]("simple_case"),
		WithExpectedOutput[string, string, any]("Paris"),
	)
	ds, err := NewDataset[string, string, any](
		"capital_eval",
		[]Case[string, string, any]{c},
		IsInstance[string, string, any]{TypeName: "string"},
		myEvaluator{},
	)
	if err != nil {
		t.Fatal(err)
	}
	report, err := ds.Evaluate(context.Background(), func(_ context.Context, _ string) (string, error) {
		return "Paris", nil
	})
	if err != nil {
		t.Fatal(err)
	}

	want := `                                 Evaluation Summary: task
┏━━━━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━━━┓
┃ Case ID     ┃ Inputs                         ┃ Outputs ┃ Scores            ┃ Assertions ┃
┡━━━━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━━━━┩
│ simple_case │ What is the capital of France? │ Paris   │ MyEvaluator: 1.00 │ ✔          │
├─────────────┼────────────────────────────────┼─────────┼───────────────────┼────────────┤
│ Averages    │                                │         │ MyEvaluator: 1.00 │ 100.0% ✔   │
└─────────────┴────────────────────────────────┴─────────┴───────────────────┴────────────┘`

	got := report.Render(RenderOptions{IncludeInput: true, IncludeOutput: true, IncludeAverages: true})
	assertRender(t, got, want)
}

type myEvaluator struct{}

func (myEvaluator) Evaluate(_ context.Context, _ *EvaluatorContext[string, string, any]) (EvaluatorOutput, error) {
	return ScalarValue(Float(1.0)), nil
}
func (myEvaluator) Spec() EvaluatorSpec           { return NewSpec("MyEvaluator") }
func (myEvaluator) DefaultEvaluationName() string { return "MyEvaluator" }

// TestPrintDoesNotPanic exercises Print. The package writes to an unexported
// stdout that a test cannot reassign, so the written bytes are not capturable via
// the public API; we only assert that Print does not panic.
func TestPrintDoesNotPanic(t *testing.T) {
	r := &EvaluationReport[string, string, any]{Name: "task", Cases: []ReportCase[string, string, any]{scoreCase()}}
	defer func() {
		if rec := recover(); rec != nil {
			t.Fatalf("Print panicked: %v", rec)
		}
	}()
	r.Print()
	r.Print(RenderOptions{IncludeInput: true})
}

func TestRenderAssertionFailMark(t *testing.T) {
	c := ReportCase[string, string, any]{
		Name:         "case_1",
		Assertions:   map[string]EvaluationResult{"check": {Name: "check", Value: Bool(false)}},
		TaskDuration: 1500 * time.Millisecond,
	}
	r := &EvaluationReport[string, string, any]{Name: "task", Cases: []ReportCase[string, string, any]{c}}

	want := `      Evaluation Summary: task
┏━━━━━━━━━━┳━━━━━━━━━━━━┳━━━━━━━━━━┓
┃ Case ID  ┃ Assertions ┃ Duration ┃
┡━━━━━━━━━━╇━━━━━━━━━━━━╇━━━━━━━━━━┩
│ case_1   │ ✗          │     1.5s │
├──────────┼────────────┼──────────┤
│ Averages │ 0.0% ✔     │     1.5s │
└──────────┴────────────┴──────────┘`

	assertRender(t, r.Render(), want)
}

// TestRenderReasonsNoReasonText covers the IncludeReasons branch where a result
// has no Reason, so only the marker is shown (no trailing reason line).
func TestRenderReasonsNoReasonText(t *testing.T) {
	c := ReportCase[string, string, any]{
		Name:         "case_1",
		Scores:       map[string]EvaluationResult{"s": {Name: "s", Value: Int(3)}},
		Assertions:   map[string]EvaluationResult{"a": {Name: "a", Value: Bool(true)}},
		TaskDuration: 1500 * time.Millisecond,
	}
	r := &EvaluationReport[string, string, any]{Name: "task", Cases: []ReportCase[string, string, any]{c}}

	want := `          Evaluation Summary: task
┏━━━━━━━━━┳━━━━━━━━┳━━━━━━━━━━━━┳━━━━━━━━━━┓
┃ Case ID ┃ Scores ┃ Assertions ┃ Duration ┃
┡━━━━━━━━━╇━━━━━━━━╇━━━━━━━━━━━━╇━━━━━━━━━━┩
│ case_1  │ s: 3   │ a: ✔       │     1.5s │
│         │        │            │          │
└─────────┴────────┴────────────┴──────────┘`

	got := r.Render(RenderOptions{IncludeDurations: true, IncludeReasons: true})
	assertRender(t, got, want)
}

func TestRenderIntScoreUsesThousandsSeparator(t *testing.T) {
	c := ReportCase[string, string, any]{
		Name:         "case_1",
		Scores:       map[string]EvaluationResult{"count": {Name: "count", Value: Int(1234)}},
		TaskDuration: 1500 * time.Millisecond,
	}
	r := &EvaluationReport[string, string, any]{Name: "task", Cases: []ReportCase[string, string, any]{c}}

	got := r.Render()
	if !strings.Contains(got, "count: 1,234") {
		t.Errorf("expected int score with thousands separator, got:\n%s", got)
	}
}

// TestRenderNilOutputShowsDash covers the nil arm of cell formatting: a nil
// output (when the output type is an interface) renders as a dash.
func TestRenderNilOutputShowsDash(t *testing.T) {
	c := ReportCase[any, any, any]{Name: "c1", Output: nil, TaskDuration: time.Second}
	r := &EvaluationReport[any, any, any]{Name: "task", Cases: []ReportCase[any, any, any]{c}}

	want := `Evaluation Summary: task
┏━━━━━━━━━┳━━━━━━━━━┓
┃ Case ID ┃ Outputs ┃
┡━━━━━━━━━╇━━━━━━━━━┩
│ c1      │ -       │
└─────────┴─────────┘`

	got := r.Render(RenderOptions{IncludeOutput: true})
	assertRender(t, got, want)
}

// TestRenderScalarOutput covers the Scalar arm of cell formatting: a Scalar
// output renders via its String method.
func TestRenderScalarOutput(t *testing.T) {
	c := ReportCase[any, Scalar, any]{Name: "c1", Output: Label("hi"), TaskDuration: time.Second}
	r := &EvaluationReport[any, Scalar, any]{Name: "task", Cases: []ReportCase[any, Scalar, any]{c}}

	want := `Evaluation Summary: task
┏━━━━━━━━━┳━━━━━━━━━┓
┃ Case ID ┃ Outputs ┃
┡━━━━━━━━━╇━━━━━━━━━┩
│ c1      │ hi      │
└─────────┴─────────┘`

	got := r.Render(RenderOptions{IncludeOutput: true})
	assertRender(t, got, want)
}

// TestRenderEvaluatorFailuresDashForCaseWithout covers the dash rendered for a
// case with no evaluator failures when another case populates the column.
func TestRenderEvaluatorFailuresDashForCaseWithout(t *testing.T) {
	c1 := ReportCase[string, string, any]{
		Name:              "c1",
		EvaluatorFailures: []EvaluatorFailure{{Name: "Boom", ErrorMessage: "x"}},
		TaskDuration:      time.Second,
	}
	c2 := ReportCase[string, string, any]{Name: "c2", TaskDuration: time.Second}
	r := &EvaluationReport[string, string, any]{Name: "task", Cases: []ReportCase[string, string, any]{c1, c2}}

	want := `          Evaluation Summary: task
┏━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━┓
┃ Case ID  ┃ Evaluator Failures ┃ Duration ┃
┡━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━━┩
│ c1       │ Boom: x            │     1.0s │
├──────────┼────────────────────┼──────────┤
│ c2       │ -                  │     1.0s │
├──────────┼────────────────────┼──────────┤
│ Averages │                    │     1.0s │
└──────────┴────────────────────┴──────────┘`

	assertRender(t, r.Render(), want)
}

// TestRenderMixedAssertionsAcrossCases covers the per-case path where the
// assertions column is present but a given case has no assertions (rendered as a
// dash) while the aggregate still reports a pass rate.
func TestRenderMixedAssertionsAcrossCases(t *testing.T) {
	c1 := ReportCase[string, string, any]{
		Name:         "c1",
		Scores:       map[string]EvaluationResult{"s": {Name: "s", Value: Float(0.5)}},
		Assertions:   map[string]EvaluationResult{"a": {Name: "a", Value: Bool(true)}},
		TaskDuration: time.Second,
	}
	c2 := ReportCase[string, string, any]{
		Name:         "c2",
		Scores:       map[string]EvaluationResult{"s": {Name: "s", Value: Float(0.5)}},
		TaskDuration: time.Second,
	}
	r := &EvaluationReport[string, string, any]{Name: "task", Cases: []ReportCase[string, string, any]{c1, c2}}

	want := `           Evaluation Summary: task
┏━━━━━━━━━━┳━━━━━━━━━━┳━━━━━━━━━━━━┳━━━━━━━━━━┓
┃ Case ID  ┃ Scores   ┃ Assertions ┃ Duration ┃
┡━━━━━━━━━━╇━━━━━━━━━━╇━━━━━━━━━━━━╇━━━━━━━━━━┩
│ c1       │ s: 0.500 │ ✔          │     1.0s │
├──────────┼──────────┼────────────┼──────────┤
│ c2       │ s: 0.500 │ -          │     1.0s │
├──────────┼──────────┼────────────┼──────────┤
│ Averages │ s: 0.500 │ 100.0% ✔   │     1.0s │
└──────────┴──────────┴────────────┴──────────┘`

	assertRender(t, r.Render(), want)
}
