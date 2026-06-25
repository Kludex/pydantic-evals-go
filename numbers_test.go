package evals

import (
	"context"
	"strings"
	"testing"
)

// numbersScoreEvaluator is a public-API evaluator that emits a fixed set of
// named scalar scores, used to drive the report number-formatting code paths.
type numbersScoreEvaluator struct {
	scores ScalarMapOutput
}

func (e numbersScoreEvaluator) Evaluate(_ context.Context, _ *EvaluatorContext[string, string, string]) (EvaluatorOutput, error) {
	return e.scores, nil
}

func (e numbersScoreEvaluator) Spec() EvaluatorSpec { return NewSpec("numbersScore") }

// numbersIdentity returns the input unchanged.
func numbersIdentity(_ context.Context, in string) (string, error) { return in, nil }

// numbersReport evaluates a single named case with the given evaluators and the
// given metric-recording task, returning the resulting report.
func numbersReport(t *testing.T, caseName string, metrics map[string]float64, evaluators ...Evaluator[string, string, string]) *EvaluationReport[string, string, string] {
	t.Helper()
	ds, err := NewDataset[string, string, string]("ds", []Case[string, string, string]{
		NewCase[string, string, string]("input", WithCaseName[string, string, string](caseName)),
	}, evaluators...)
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}
	task := func(ctx context.Context, in string) (string, error) {
		for name, amount := range metrics {
			IncrementMetric(ctx, name, amount)
		}
		return in, nil
	}
	rep, err := ds.Evaluate(context.Background(), task, WithName[string, string, string]("exp"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	return rep
}

// renderNoDurations renders deterministically by excluding the timing column.
func renderNoDurations(r *EvaluationReport[string, string, string]) string {
	return r.Render(RenderOptions{IncludeAverages: true})
}

func TestRenderIntScoreNoDecimals(t *testing.T) {
	rep := numbersReport(t, "c1", nil, numbersScoreEvaluator{scores: ScalarMapOutput{"acc": Int(3)}})
	want := `Evaluation Summary: exp
┏━━━━━━━━━━┳━━━━━━━━━━━┓
┃ Case ID  ┃ Scores    ┃
┡━━━━━━━━━━╇━━━━━━━━━━━┩
│ c1       │ acc: 3    │
├──────────┼───────────┤
│ Averages │ acc: 3.00 │
└──────────┴───────────┘`
	if got := renderNoDurations(rep); got != want {
		t.Fatalf("rendered table mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderLargeIntScoreThousands(t *testing.T) {
	rep := numbersReport(t, "c1", nil, numbersScoreEvaluator{scores: ScalarMapOutput{"n": Int(1234567)}})
	got := renderNoDurations(rep)
	if !strings.Contains(got, "n: 1,234,567") {
		t.Fatalf("expected case score with thousands separators, got:\n%s", got)
	}
	if !strings.Contains(got, "n: 1,234,567.0") {
		t.Fatalf("expected aggregate score rendered as float, got:\n%s", got)
	}
}

func TestRenderFloatScoreSignificantFigures(t *testing.T) {
	// Each value exercises a distinct branch of default_render_number.
	cases := []struct {
		name string
		val  float64
		want string
	}{
		{"a", 1.5, "a: 1.50"},           // >= 1, two decimals to reach 3 sig figs
		{"b", 12.0, "b: 12.0"},          // >= 10, one decimal floor
		{"c", 123.0, "c: 123.0"},        // >= 100, decimals floored to 1
		{"d", 0.5, "d: 0.500"},          // between 0 and 1, exponent -1
		{"e", 0.05, "e: 0.0500"},        // between 0 and 1, exponent -2
		{"z", 0.0, "z: 0.000"},          // exact zero special case
		{"big", 1500.0, "big: 1,500.0"}, // >= 1000 keeps thousands separator
	}
	scores := ScalarMapOutput{}
	for _, c := range cases {
		scores[c.name] = Float(c.val)
	}
	rep := numbersReport(t, "c1", nil, numbersScoreEvaluator{scores: scores})
	got := renderNoDurations(rep)
	for _, c := range cases {
		if !strings.Contains(got, c.want) {
			t.Errorf("expected score line %q, got:\n%s", c.want, got)
		}
	}
}

func TestRenderNegativeFloatScore(t *testing.T) {
	rep := numbersReport(t, "c1", nil, numbersScoreEvaluator{scores: ScalarMapOutput{"x": Float(-1.5)}})
	got := renderNoDurations(rep)
	if !strings.Contains(got, "x: -1.50") {
		t.Fatalf("expected negative float score, got:\n%s", got)
	}
	if !strings.Contains(got, "Averages") || !strings.Contains(got, "x: -1.50") {
		t.Fatalf("expected aggregate to also render negative float, got:\n%s", got)
	}
}

func TestRenderNegativeLargeFloatScoreThousands(t *testing.T) {
	rep := numbersReport(t, "c1", nil, numbersScoreEvaluator{scores: ScalarMapOutput{"x": Float(-1234.0)}})
	got := renderNoDurations(rep)
	if !strings.Contains(got, "x: -1,234") {
		t.Fatalf("expected negative thousands-separated float score, got:\n%s", got)
	}
}

func TestRenderWholeMetricAsIntThousands(t *testing.T) {
	rep := numbersReport(t, "c1", map[string]float64{"tokens": 1234567},
		numbersScoreEvaluator{scores: ScalarMapOutput{"acc": Int(1)}})
	want := `           Evaluation Summary: exp
┏━━━━━━━━━━┳━━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━━┓
┃ Case ID  ┃ Scores    ┃ Metrics             ┃
┡━━━━━━━━━━╇━━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━━━┩
│ c1       │ acc: 1    │ tokens: 1,234,567   │
├──────────┼───────────┼─────────────────────┤
│ Averages │ acc: 1.00 │ tokens: 1,234,567.0 │
└──────────┴───────────┴─────────────────────┘`
	if got := renderNoDurations(rep); got != want {
		t.Fatalf("rendered table mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestRenderNegativeWholeMetricThousands(t *testing.T) {
	rep := numbersReport(t, "c1", map[string]float64{"delta": -4321},
		numbersScoreEvaluator{scores: ScalarMapOutput{"acc": Int(1)}})
	got := renderNoDurations(rep)
	if !strings.Contains(got, "delta: -4,321\n") && !strings.Contains(got, "delta: -4,321 ") {
		t.Fatalf("expected negative whole metric as int with thousands, got:\n%s", got)
	}
	if !strings.Contains(got, "delta: -4,321.0") {
		t.Fatalf("expected aggregate negative whole metric as float, got:\n%s", got)
	}
}

func TestRenderFractionalMetric(t *testing.T) {
	// A non-whole metric renders as a float in the case row too.
	rep := numbersReport(t, "c1", map[string]float64{"latency": 1.5},
		numbersScoreEvaluator{scores: ScalarMapOutput{"acc": Int(1)}})
	got := renderNoDurations(rep)
	if !strings.Contains(got, "latency: 1.50") {
		t.Fatalf("expected fractional metric rendered as float, got:\n%s", got)
	}
}

func TestRenderAggregateAlwaysFloat(t *testing.T) {
	// An all-1.0 integer score averages to 1.00 (aggregate is always a float).
	rep, err := buildMultiCaseReport(t)
	if err != nil {
		t.Fatal(err)
	}
	got := renderNoDurations(rep)
	if !strings.Contains(got, "acc: 1.00") {
		t.Fatalf("expected aggregate score 1.00, got:\n%s", got)
	}
}

func TestRenderAggregateFractionalAverage(t *testing.T) {
	// Two cases scoring 0 and 1 average to 0.500 in the aggregate row.
	ds, err := NewDataset[string, string, string]("ds", []Case[string, string, string]{
		NewCase[string, string, string]("a", WithCaseName[string, string, string]("a"),
			WithCaseEvaluators(Evaluator[string, string, string](numbersScoreEvaluator{scores: ScalarMapOutput{"acc": Int(0)}}))),
		NewCase[string, string, string]("b", WithCaseName[string, string, string]("b"),
			WithCaseEvaluators(Evaluator[string, string, string](numbersScoreEvaluator{scores: ScalarMapOutput{"acc": Int(1)}}))),
	})
	if err != nil {
		t.Fatal(err)
	}
	rep, err := ds.Evaluate(context.Background(), numbersIdentity, WithName[string, string, string]("exp"))
	if err != nil {
		t.Fatal(err)
	}
	got := renderNoDurations(rep)
	if !strings.Contains(got, "acc: 0.500") {
		t.Fatalf("expected aggregate average 0.500, got:\n%s", got)
	}
}

func TestRenderAggregateMetricWithDecimals(t *testing.T) {
	// Two cases record whole metrics 3 and 1 (both present), so the aggregate
	// averages to a whole-but-float value of 2.00.
	ds, err := NewDataset[string, string, string]("ds", []Case[string, string, string]{
		NewCase[string, string, string]("3", WithCaseName[string, string, string]("a")),
		NewCase[string, string, string]("1", WithCaseName[string, string, string]("b")),
	}, numbersScoreEvaluator{scores: ScalarMapOutput{"acc": Int(1)}})
	if err != nil {
		t.Fatal(err)
	}
	task := func(ctx context.Context, in string) (string, error) {
		if in == "3" {
			IncrementMetric(ctx, "m", 3)
		} else {
			IncrementMetric(ctx, "m", 1)
		}
		return in, nil
	}
	rep, err := ds.Evaluate(context.Background(), task, WithName[string, string, string]("exp"))
	if err != nil {
		t.Fatal(err)
	}
	got := renderNoDurations(rep)
	// The case rows render the whole metrics as ints.
	if !strings.Contains(got, "m: 3 ") {
		t.Fatalf("expected case metric 3 as int, got:\n%s", got)
	}
	if !strings.Contains(got, "m: 1 ") {
		t.Fatalf("expected case metric 1 as int, got:\n%s", got)
	}
	// The average across the two cases (3 and 1) is 2.00, always a float.
	if !strings.Contains(got, "m: 2.00") {
		t.Fatalf("expected aggregate metric 2.00, got:\n%s", got)
	}
}

func TestRenderDurationCellPresence(t *testing.T) {
	rep := numbersReport(t, "c1", nil, numbersScoreEvaluator{scores: ScalarMapOutput{"acc": Int(1)}})

	single := rep.Render(RenderOptions{IncludeDurations: true, IncludeAverages: true})
	if !strings.Contains(single, "Duration") {
		t.Fatalf("expected a Duration header, got:\n%s", single)
	}
	if !hasDurationUnit(single) {
		t.Fatalf("expected a duration unit (µs/ms/s) cell, got:\n%s", single)
	}

	both := rep.Render(RenderOptions{IncludeDurations: true, IncludeTotalDuration: true, IncludeAverages: true})
	if !strings.Contains(both, "Durations") {
		t.Fatalf("expected pluralized Durations header, got:\n%s", both)
	}
	if !strings.Contains(both, "task:") || !strings.Contains(both, "total:") {
		t.Fatalf("expected task/total duration prefixes, got:\n%s", both)
	}
}

func TestRenderDurationAggregatePresence(t *testing.T) {
	// The aggregate row carries its own duration cell when averages are shown.
	rep := numbersReport(t, "c1", nil, numbersScoreEvaluator{scores: ScalarMapOutput{"acc": Int(1)}})
	out := rep.Render(RenderOptions{IncludeDurations: true, IncludeAverages: true})
	lines := strings.Split(out, "\n")
	var avgLine string
	for _, l := range lines {
		if strings.Contains(l, "Averages") {
			avgLine = l
			break
		}
	}
	if avgLine == "" {
		t.Fatalf("no Averages row found, got:\n%s", out)
	}
	if !hasDurationUnit(avgLine) {
		t.Fatalf("expected aggregate duration unit on Averages row, got line: %q", avgLine)
	}
}

func buildMultiCaseReport(t *testing.T) (*EvaluationReport[string, string, string], error) {
	t.Helper()
	ds, err := NewDataset[string, string, string]("ds", []Case[string, string, string]{
		NewCase[string, string, string]("a", WithCaseName[string, string, string]("a")),
		NewCase[string, string, string]("b", WithCaseName[string, string, string]("b")),
	}, numbersScoreEvaluator{scores: ScalarMapOutput{"acc": Int(1)}})
	if err != nil {
		return nil, err
	}
	return ds.Evaluate(context.Background(), numbersIdentity, WithName[string, string, string]("exp"))
}

func hasDurationUnit(s string) bool {
	return strings.Contains(s, "µs") || strings.Contains(s, "ms") || strings.Contains(s, "s")
}
