package evals_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	evals "github.com/Kludex/pydantic-evals-go"
)

// bxReport is a fixed-duration report used by the render tests so that the
// rendered box-drawing tables are fully deterministic.
func bxReport(t *testing.T) *evals.EvaluationReport[string, string, any] {
	t.Helper()
	return &evals.EvaluationReport[string, string, any]{
		Name: "task",
		Cases: []evals.ReportCase[string, string, any]{
			{
				Name:   "case1",
				Inputs: "in1",
				Output: "out1",
				Scores: map[string]evals.EvaluationResult{
					"sc": {Name: "sc", Value: evals.Float(0.5)},
				},
				Labels: map[string]evals.EvaluationResult{
					"lab": {Name: "lab", Value: evals.Label("good")},
				},
				Metrics: map[string]float64{"m": 3},
				Assertions: map[string]evals.EvaluationResult{
					"ok": {Name: "ok", Value: evals.Bool(true)},
				},
				TaskDuration:  10 * time.Millisecond,
				TotalDuration: 20 * time.Millisecond,
			},
		},
	}
}

func TestBxDefaultRenderOptions(t *testing.T) {
	o := evals.DefaultRenderOptions()
	want := evals.RenderOptions{IncludeDurations: true, IncludeAverages: true}
	if o != want {
		t.Fatalf("DefaultRenderOptions() = %+v, want %+v", o, want)
	}
}

func TestBxRenderDefault(t *testing.T) {
	rep := bxReport(t)
	const want = "                           Evaluation Summary: task\n" +
		"┏━━━━━━━━━━┳━━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━┳━━━━━━━━━━━━┳━━━━━━━━━━┓\n" +
		"┃ Case ID  ┃ Scores    ┃ Labels            ┃ Metrics ┃ Assertions ┃ Duration ┃\n" +
		"┡━━━━━━━━━━╇━━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━╇━━━━━━━━━━━━╇━━━━━━━━━━┩\n" +
		"│ case1    │ sc: 0.500 │ lab: good         │ m: 3    │ ✔          │   10.0ms │\n" +
		"├──────────┼───────────┼───────────────────┼─────────┼────────────┼──────────┤\n" +
		"│ Averages │ sc: 0.500 │ lab: good: 100.0% │ m: 3.00 │ 100.0% ✔   │   10.0ms │\n" +
		"└──────────┴───────────┴───────────────────┴─────────┴────────────┴──────────┘"
	if got := rep.Render(); got != want {
		t.Fatalf("Render() default mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestBxRenderAllColumnsWithReasons(t *testing.T) {
	rep := &evals.EvaluationReport[string, string, any]{
		Name: "myexp",
		Cases: []evals.ReportCase[string, string, any]{
			{
				Name:              "case1",
				Inputs:            "in1",
				Metadata:          map[string]any{"k": "v"},
				HasMetadata:       true,
				ExpectedOutput:    "exp1",
				HasExpectedOutput: true,
				Output:            "out1",
				Scores: map[string]evals.EvaluationResult{
					"sc": {Name: "sc", Value: evals.Float(0.5), Reason: "because"},
				},
				Labels: map[string]evals.EvaluationResult{
					"lab": {Name: "lab", Value: evals.Label("good"), Reason: "labreason"},
				},
				Metrics: map[string]float64{"m": 3},
				Assertions: map[string]evals.EvaluationResult{
					"ok": {Name: "ok", Value: evals.Bool(true), Reason: "passed"},
				},
				TaskDuration:  10 * time.Millisecond,
				TotalDuration: 20 * time.Millisecond,
			},
		},
	}
	opts := evals.RenderOptions{
		IncludeInput:          true,
		IncludeMetadata:       true,
		IncludeExpectedOutput: true,
		IncludeOutput:         true,
		IncludeDurations:      true,
		IncludeTotalDuration:  true,
		IncludeAverages:       true,
		IncludeReasons:        true,
	}
	const want = "                                                             Evaluation Summary: myexp\n" +
		"┏━━━━━━━━━━┳━━━━━━━━┳━━━━━━━━━━┳━━━━━━━━━━━━━━━━━┳━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━┳━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━━━━━━┓\n" +
		"┃ Case ID  ┃ Inputs ┃ Metadata ┃ Expected Output ┃ Outputs ┃ Scores            ┃ Labels              ┃ Metrics ┃ Assertions       ┃     Durations ┃\n" +
		"┡━━━━━━━━━━╇━━━━━━━━╇━━━━━━━━━━╇━━━━━━━━━━━━━━━━━╇━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━╇━━━━━━━━━━━━━━━━━━╇━━━━━━━━━━━━━━━┩\n" +
		"│ case1    │ in1    │ {k: v}   │ exp1            │ out1    │ sc: 0.500         │ lab: good           │ m: 3    │ ok: ✔            │  task: 10.0ms │\n" +
		"│          │        │          │                 │         │   Reason: because │   Reason: labreason │         │   Reason: passed │ total: 20.0ms │\n" +
		"│          │        │          │                 │         │                   │                     │         │                  │               │\n" +
		"│          │        │          │                 │         │                   │                     │         │                  │               │\n" +
		"├──────────┼────────┼──────────┼─────────────────┼─────────┼───────────────────┼─────────────────────┼─────────┼──────────────────┼───────────────┤\n" +
		"│ Averages │        │          │                 │         │ sc: 0.500         │ lab: good: 100.0%   │ m: 3.00 │ 100.0% ✔         │  task: 10.0ms │\n" +
		"│          │        │          │                 │         │                   │                     │         │                  │ total: 20.0ms │\n" +
		"└──────────┴────────┴──────────┴─────────────────┴─────────┴───────────────────┴─────────────────────┴─────────┴──────────────────┴───────────────┘"
	if got := rep.Render(opts); got != want {
		t.Fatalf("Render() all-columns mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestBxRenderReasonsDefaultColumns(t *testing.T) {
	rep := &evals.EvaluationReport[string, string, any]{
		Name: "task",
		Cases: []evals.ReportCase[string, string, any]{
			{
				Name:   "c",
				Output: "o",
				Scores: map[string]evals.EvaluationResult{
					"sc": {Name: "sc", Value: evals.Float(0.5), Reason: "scorereason"},
				},
				Labels: map[string]evals.EvaluationResult{
					"lab": {Name: "lab", Value: evals.Label("good"), Reason: "labelreason"},
				},
				TaskDuration: time.Millisecond,
			},
		},
	}
	const want = "                       Evaluation Summary: task\n" +
		"┏━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━┓\n" +
		"┃ Case ID  ┃ Scores                ┃ Labels                ┃ Duration ┃\n" +
		"┡━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━━┩\n" +
		"│ c        │ sc: 0.500             │ lab: good             │    1.0ms │\n" +
		"│          │   Reason: scorereason │   Reason: labelreason │          │\n" +
		"│          │                       │                       │          │\n" +
		"├──────────┼───────────────────────┼───────────────────────┼──────────┤\n" +
		"│ Averages │ sc: 0.500             │ lab: good: 100.0%     │    1.0ms │\n" +
		"└──────────┴───────────────────────┴───────────────────────┴──────────┘"
	if got := rep.Render(evals.RenderOptions{IncludeReasons: true, IncludeDurations: true, IncludeAverages: true}); got != want {
		t.Fatalf("Render() score/label reasons mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestBxRenderEvaluatorFailures(t *testing.T) {
	rep := &evals.EvaluationReport[string, string, any]{
		Name: "task",
		Cases: []evals.ReportCase[string, string, any]{
			{
				Name:   "case1",
				Output: "out1",
				Assertions: map[string]evals.EvaluationResult{
					"ok": {Name: "ok", Value: evals.Bool(false)},
				},
				EvaluatorFailures: []evals.EvaluatorFailure{
					{Name: "Boom", ErrorMessage: "kaboom"},
				},
				TaskDuration: 5 * time.Millisecond,
			},
		},
	}
	const want = "                Evaluation Summary: task\n" +
		"┏━━━━━━━━━━┳━━━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━┓\n" +
		"┃ Case ID  ┃ Assertions ┃ Evaluator Failures ┃ Duration ┃\n" +
		"┡━━━━━━━━━━╇━━━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━━┩\n" +
		"│ case1    │ ✗          │ Boom: kaboom       │    5.0ms │\n" +
		"├──────────┼────────────┼────────────────────┼──────────┤\n" +
		"│ Averages │ 0.0% ✔     │                    │    5.0ms │\n" +
		"└──────────┴────────────┴────────────────────┴──────────┘"
	if got := rep.Render(); got != want {
		t.Fatalf("Render() evaluator-failures mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestBxRenderEmpty(t *testing.T) {
	rep := &evals.EvaluationReport[string, string, any]{Name: "empty"}
	const want = "Evaluation Summary: empty\n" +
		"┏━━━━━━━━━┳━━━━━━━━━━┓\n" +
		"┃ Case ID ┃ Duration ┃\n" +
		"┡━━━━━━━━━╇━━━━━━━━━━┩\n" +
		"└─────────┴──────────┘"
	if got := rep.Render(); got != want {
		t.Fatalf("Render() empty mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestBxRenderTitleOverrideAndOmit(t *testing.T) {
	rep := &evals.EvaluationReport[string, string, any]{
		Name: "task",
		Cases: []evals.ReportCase[string, string, any]{
			{Name: "case1", Output: "out1", TaskDuration: 5 * time.Millisecond},
		},
	}

	const wantOmit = "┏━━━━━━━━━┳━━━━━━━━━━┓\n" +
		"┃ Case ID ┃ Duration ┃\n" +
		"┡━━━━━━━━━╇━━━━━━━━━━┩\n" +
		"│ case1   │    5.0ms │\n" +
		"└─────────┴──────────┘"
	if got := rep.Render(evals.RenderOptions{OmitTitle: true, IncludeDurations: true}); got != wantOmit {
		t.Fatalf("Render() OmitTitle mismatch:\n got %q\nwant %q", got, wantOmit)
	}

	const wantTitle = "        Custom\n" +
		"┏━━━━━━━━━┳━━━━━━━━━━┓\n" +
		"┃ Case ID ┃ Duration ┃\n" +
		"┡━━━━━━━━━╇━━━━━━━━━━┩\n" +
		"│ case1   │    5.0ms │\n" +
		"└─────────┴──────────┘"
	if got := rep.Render(evals.RenderOptions{Title: "Custom", IncludeDurations: true}); got != wantTitle {
		t.Fatalf("Render() Title override mismatch:\n got %q\nwant %q", got, wantTitle)
	}
}

func TestBxRenderNoAverages(t *testing.T) {
	rep := &evals.EvaluationReport[string, string, any]{
		Name: "task",
		Cases: []evals.ReportCase[string, string, any]{
			{Name: "case1", Output: "out1", TaskDuration: 5 * time.Millisecond},
		},
	}
	const want = "Evaluation Summary: task\n" +
		"┏━━━━━━━━━┓\n" +
		"┃ Case ID ┃\n" +
		"┡━━━━━━━━━┩\n" +
		"│ case1   │\n" +
		"└─────────┘"
	if got := rep.Render(evals.RenderOptions{IncludeAverages: false}); got != want {
		t.Fatalf("Render() IncludeAverages=false mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestBxRenderTotalDurationLines(t *testing.T) {
	rep := &evals.EvaluationReport[string, string, any]{
		Name: "task",
		Cases: []evals.ReportCase[string, string, any]{
			{Name: "case1", Output: "out1", TaskDuration: 10 * time.Millisecond, TotalDuration: 20 * time.Millisecond},
		},
	}
	const want = "  Evaluation Summary: task\n" +
		"┏━━━━━━━━━━┳━━━━━━━━━━━━━━━┓\n" +
		"┃ Case ID  ┃     Durations ┃\n" +
		"┡━━━━━━━━━━╇━━━━━━━━━━━━━━━┩\n" +
		"│ case1    │  task: 10.0ms │\n" +
		"│          │ total: 20.0ms │\n" +
		"├──────────┼───────────────┤\n" +
		"│ Averages │  task: 10.0ms │\n" +
		"│          │ total: 20.0ms │\n" +
		"└──────────┴───────────────┘"
	got := rep.Render(evals.RenderOptions{IncludeDurations: true, IncludeTotalDuration: true, IncludeAverages: true})
	if got != want {
		t.Fatalf("Render() total-duration mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestBxRenderScoreIntColumn(t *testing.T) {
	rep := &evals.EvaluationReport[string, string, any]{
		Name: "task",
		Cases: []evals.ReportCase[string, string, any]{
			{
				Name:   "c",
				Output: "o",
				Scores: map[string]evals.EvaluationResult{
					"i": {Name: "i", Value: evals.Int(5)},
				},
				TaskDuration: time.Millisecond,
			},
		},
	}
	const want = "    Evaluation Summary: task\n" +
		"┏━━━━━━━━━━┳━━━━━━━━━┳━━━━━━━━━━┓\n" +
		"┃ Case ID  ┃ Scores  ┃ Duration ┃\n" +
		"┡━━━━━━━━━━╇━━━━━━━━━╇━━━━━━━━━━┩\n" +
		"│ c        │ i: 5    │    1.0ms │\n" +
		"├──────────┼─────────┼──────────┤\n" +
		"│ Averages │ i: 5.00 │    1.0ms │\n" +
		"└──────────┴─────────┴──────────┘"
	if got := rep.Render(); got != want {
		t.Fatalf("Render() int score mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestBxRenderMetricsThousands(t *testing.T) {
	rep := &evals.EvaluationReport[string, string, any]{
		Name: "task",
		Cases: []evals.ReportCase[string, string, any]{
			{Name: "c", Output: "o", Metrics: map[string]float64{"tokens": 1500}, TaskDuration: time.Millisecond},
		},
	}
	const want = "        Evaluation Summary: task\n" +
		"┏━━━━━━━━━━┳━━━━━━━━━━━━━━━━━┳━━━━━━━━━━┓\n" +
		"┃ Case ID  ┃ Metrics         ┃ Duration ┃\n" +
		"┡━━━━━━━━━━╇━━━━━━━━━━━━━━━━━╇━━━━━━━━━━┩\n" +
		"│ c        │ tokens: 1,500   │    1.0ms │\n" +
		"├──────────┼─────────────────┼──────────┤\n" +
		"│ Averages │ tokens: 1,500.0 │    1.0ms │\n" +
		"└──────────┴─────────────────┴──────────┘"
	if got := rep.Render(); got != want {
		t.Fatalf("Render() metrics mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestBxRenderLabelDistribution(t *testing.T) {
	rep := &evals.EvaluationReport[string, string, any]{
		Name: "task",
		Cases: []evals.ReportCase[string, string, any]{
			{Name: "c1", Output: "o", Labels: map[string]evals.EvaluationResult{"cat": {Name: "cat", Value: evals.Label("a")}}, TaskDuration: time.Millisecond},
			{Name: "c2", Output: "o", Labels: map[string]evals.EvaluationResult{"cat": {Name: "cat", Value: evals.Label("b")}}, TaskDuration: time.Millisecond},
		},
	}
	const want = "            Evaluation Summary: task\n" +
		"┏━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━┓\n" +
		"┃ Case ID  ┃ Labels                  ┃ Duration ┃\n" +
		"┡━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━━┩\n" +
		"│ c1       │ cat: a                  │    1.0ms │\n" +
		"├──────────┼─────────────────────────┼──────────┤\n" +
		"│ c2       │ cat: b                  │    1.0ms │\n" +
		"├──────────┼─────────────────────────┼──────────┤\n" +
		"│ Averages │ cat: a: 50.0%, b: 50.0% │    1.0ms │\n" +
		"└──────────┴─────────────────────────┴──────────┘"
	if got := rep.Render(); got != want {
		t.Fatalf("Render() label distribution mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestBxRenderMapInputsOutputs(t *testing.T) {
	rep := &evals.EvaluationReport[map[string]any, map[string]any, any]{
		Name: "task",
		Cases: []evals.ReportCase[map[string]any, map[string]any, any]{
			{
				Name:   "c",
				Inputs: map[string]any{"b": 2, "a": 1},
				Output: map[string]any{"z": 9, "y": 8},
				Assertions: map[string]evals.EvaluationResult{
					"ok": {Name: "ok", Value: evals.Bool(true)},
				},
				TaskDuration: time.Millisecond,
			},
		},
	}
	const want = "               Evaluation Summary: task\n" +
		"┏━━━━━━━━━┳━━━━━━━━━━━━━━┳━━━━━━━━━━━━━━┳━━━━━━━━━━━━┓\n" +
		"┃ Case ID ┃ Inputs       ┃ Outputs      ┃ Assertions ┃\n" +
		"┡━━━━━━━━━╇━━━━━━━━━━━━━━╇━━━━━━━━━━━━━━╇━━━━━━━━━━━━┩\n" +
		"│ c       │ {a: 1, b: 2} │ {y: 8, z: 9} │ ✔          │\n" +
		"└─────────┴──────────────┴──────────────┴────────────┘"
	got := rep.Render(evals.RenderOptions{IncludeInput: true, IncludeOutput: true})
	if got != want {
		t.Fatalf("Render() map inputs/outputs mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestBxRenderMissingCellsAndScalarInputs(t *testing.T) {
	rep := &evals.EvaluationReport[evals.Scalar, string, any]{
		Name: "task",
		Cases: []evals.ReportCase[evals.Scalar, string, any]{
			{
				Name:   "c1",
				Inputs: evals.Label("scalarinput"),
				Output: "o1",
				Scores: map[string]evals.EvaluationResult{
					"sc": {Name: "sc", Value: evals.Float(0)},
				},
				Metrics: map[string]float64{"neg": -1500},
				Assertions: map[string]evals.EvaluationResult{
					"a": {Name: "a", Value: evals.Bool(true)},
				},
				EvaluatorFailures: []evals.EvaluatorFailure{{Name: "NoMsg"}},
				TaskDuration:      0,
			},
			{
				Name:         "c2",
				Output:       "o2",
				TaskDuration: 250 * time.Microsecond,
			},
		},
	}
	opts := evals.RenderOptions{
		IncludeInput:    true,
		IncludeMetadata: true,
		IncludeReasons:  true,
		IncludeAverages: true,
	}
	const want = "                                     Evaluation Summary: task\n" +
		"┏━━━━━━━━━━┳━━━━━━━━━━━━━┳━━━━━━━━━━┳━━━━━━━━━━━┳━━━━━━━━━━━━━━━┳━━━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━┓\n" +
		"┃ Case ID  ┃ Inputs      ┃ Metadata ┃ Scores    ┃ Metrics       ┃ Assertions ┃ Evaluator Failures ┃\n" +
		"┡━━━━━━━━━━╇━━━━━━━━━━━━━╇━━━━━━━━━━╇━━━━━━━━━━━╇━━━━━━━━━━━━━━━╇━━━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━━┩\n" +
		"│ c1       │ scalarinput │ -        │ sc: 0.000 │ neg: -1,500   │ a: ✔       │ NoMsg              │\n" +
		"│          │             │          │           │               │            │                    │\n" +
		"├──────────┼─────────────┼──────────┼───────────┼───────────────┼────────────┼────────────────────┤\n" +
		"│ c2       │ -           │ -        │ -         │ -             │ -          │ -                  │\n" +
		"├──────────┼─────────────┼──────────┼───────────┼───────────────┼────────────┼────────────────────┤\n" +
		"│ Averages │             │          │ sc: 0.000 │ neg: -1,500.0 │ 100.0% ✔   │                    │\n" +
		"└──────────┴─────────────┴──────────┴───────────┴───────────────┴────────────┴────────────────────┘"
	if got := rep.Render(opts); got != want {
		t.Fatalf("Render() missing-cells mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestBxRenderDurationUnits(t *testing.T) {
	rep := &evals.EvaluationReport[string, string, any]{
		Name: "task",
		Cases: []evals.ReportCase[string, string, any]{
			{Name: "zero", Output: "o", TaskDuration: 0},
			{Name: "us", Output: "o", TaskDuration: 1500 * time.Nanosecond},
			{Name: "subus", Output: "o", TaskDuration: 500 * time.Nanosecond},
			{Name: "sec", Output: "o", TaskDuration: 2 * time.Second},
		},
	}
	const want = "Evaluation Summary: task\n" +
		"┏━━━━━━━━━┳━━━━━━━━━━┓\n" +
		"┃ Case ID ┃ Duration ┃\n" +
		"┡━━━━━━━━━╇━━━━━━━━━━┩\n" +
		"│ zero    │       0s │\n" +
		"├─────────┼──────────┤\n" +
		"│ us      │      2µs │\n" +
		"├─────────┼──────────┤\n" +
		"│ subus   │    0.5µs │\n" +
		"├─────────┼──────────┤\n" +
		"│ sec     │     2.0s │\n" +
		"└─────────┴──────────┘"
	if got := rep.Render(evals.RenderOptions{IncludeDurations: true}); got != want {
		t.Fatalf("Render() duration units mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestBxRenderLargeFloatScore(t *testing.T) {
	rep := &evals.EvaluationReport[string, string, any]{
		Name: "task",
		Cases: []evals.ReportCase[string, string, any]{
			{Name: "c", Output: "o", Scores: map[string]evals.EvaluationResult{"big": {Name: "big", Value: evals.Float(123)}}, TaskDuration: time.Millisecond},
		},
	}
	const want = "      Evaluation Summary: task\n" +
		"┏━━━━━━━━━━┳━━━━━━━━━━━━┳━━━━━━━━━━┓\n" +
		"┃ Case ID  ┃ Scores     ┃ Duration ┃\n" +
		"┡━━━━━━━━━━╇━━━━━━━━━━━━╇━━━━━━━━━━┩\n" +
		"│ c        │ big: 123.0 │    1.0ms │\n" +
		"├──────────┼────────────┼──────────┤\n" +
		"│ Averages │ big: 123.0 │    1.0ms │\n" +
		"└──────────┴────────────┴──────────┘"
	if got := rep.Render(); got != want {
		t.Fatalf("Render() large float score mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestBxFprintWritesRenderPlusNewline(t *testing.T) {
	rep := bxReport(t)
	var buf bytes.Buffer
	rep.Fprint(&buf)
	want := rep.Render() + "\n"
	if got := buf.String(); got != want {
		t.Fatalf("Fprint default mismatch:\n got %q\nwant %q", got, want)
	}

	buf.Reset()
	opts := evals.RenderOptions{IncludeInput: true, IncludeOutput: true}
	rep.Fprint(&buf, opts)
	want = rep.Render(opts) + "\n"
	if got := buf.String(); got != want {
		t.Fatalf("Fprint with opts mismatch:\n got %q\nwant %q", got, want)
	}
}

func TestBxPrintDoesNotPanic(t *testing.T) {
	rep := bxReport(t)
	rep.Print()
	rep.Print(evals.RenderOptions{IncludeInput: true})
}

func TestBxRenderDocsExample(t *testing.T) {
	s := evals.For[string, string, any]()
	c := s.Case("What is the capital of France?").Name("simple_case").Expect("Paris")
	ds := s.Dataset("docs", c).With(bxScoreEvaluator{}, s.IsInstance("string"))
	rep, err := ds.Evaluate(context.Background(), func(_ context.Context, _ string) (string, error) {
		return "Paris", nil
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	out := rep.Render(evals.RenderOptions{IncludeAverages: true})
	const want = "            Evaluation Summary: task\n" +
		"┏━━━━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━━━┓\n" +
		"┃ Case ID     ┃ Scores            ┃ Assertions ┃\n" +
		"┡━━━━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━━━━┩\n" +
		"│ simple_case │ MyEvaluator: 1.00 │ ✔          │\n" +
		"├─────────────┼───────────────────┼────────────┤\n" +
		"│ Averages    │ MyEvaluator: 1.00 │ 100.0% ✔   │\n" +
		"└─────────────┴───────────────────┴────────────┘"
	if out != want {
		t.Fatalf("docs example render mismatch:\n got %q\nwant %q", out, want)
	}

	for _, sub := range []string{"MyEvaluator: 1.00", "✔", "100.0% ✔"} {
		if !strings.Contains(out, sub) {
			t.Fatalf("docs example render missing %q in:\n%s", sub, out)
		}
	}
	avg := rep.Averages()
	if avg == nil {
		t.Fatal("Averages() = nil, want non-nil")
	}
	if got := avg.Scores["MyEvaluator"]; got != 1.0 {
		t.Fatalf("Averages score = %v, want 1.0", got)
	}
}

type bxScoreEvaluator struct{}

func (bxScoreEvaluator) Evaluate(_ context.Context, _ *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	return evals.Score(1.0), nil
}

func (bxScoreEvaluator) EvaluationName() string { return "MyEvaluator" }
