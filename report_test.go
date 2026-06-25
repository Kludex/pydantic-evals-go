package evals

import (
	"context"
	"fmt"
	"testing"
)

// reportEmitEvaluator returns a fixed set of named scalar results, letting a test
// control exactly which assertions/scores/labels a case produces.
type reportEmitEvaluator struct {
	name    string
	results map[string]Scalar
}

func (e reportEmitEvaluator) Evaluate(_ context.Context, _ *EvaluatorContext[string, string, string]) (EvaluatorOutput, error) {
	return ScalarMapOutput(e.results), nil
}

func (e reportEmitEvaluator) Spec() EvaluatorSpec { return NewSpec(e.name) }

// reportMetricEvaluator emits a single score equal to the value of a recorded metric,
// so a test can confirm metrics survive into the evaluator context.
type reportMetricEvaluator struct {
	metricName string
	resultName string
}

func (e reportMetricEvaluator) Evaluate(_ context.Context, ec *EvaluatorContext[string, string, string]) (EvaluatorOutput, error) {
	return ScalarMapOutput{e.resultName: Float(ec.Metrics[e.metricName])}, nil
}

func (e reportMetricEvaluator) Spec() EvaluatorSpec { return NewSpec("metric") }

func reportOKTask(_ context.Context, in string) (string, error) { return in, nil }

func newReportDataset(t *testing.T, cases []Case[string, string, string], evals ...Evaluator[string, string, string]) *Dataset[string, string, string] {
	t.Helper()
	ds, err := NewDataset("ds", cases, evals...)
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}
	return ds
}

func runReport(t *testing.T, ds *Dataset[string, string, string], task TaskFunc[string, string], opts ...EvaluateOption[string, string, string]) *EvaluationReport[string, string, string] {
	t.Helper()
	report, err := ds.Evaluate(context.Background(), task, opts...)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	return report
}

func TestAveragesNoCases(t *testing.T) {
	ds := newReportDataset(t, nil)
	report := runReport(t, ds, reportOKTask)

	if report.CaseGroups() != nil {
		t.Fatalf("CaseGroups should be nil for an empty single-run report")
	}
	if avg := report.Averages(); avg != nil {
		t.Fatalf("Averages should be nil with no cases, got %+v", avg)
	}
}

func TestAveragesSingleRun(t *testing.T) {
	cases := []Case[string, string, string]{
		NewCase[string, string, string]("hi", WithCaseName[string, string, string]("a"),
			WithCaseEvaluators[string, string, string](reportEmitEvaluator{
				name: "e",
				results: map[string]Scalar{
					"pass":  Bool(true),
					"score": Int(2),
					"kind":  Label("x"),
				},
			})),
		NewCase[string, string, string]("yo", WithCaseName[string, string, string]("b"),
			WithCaseEvaluators[string, string, string](reportEmitEvaluator{
				name: "e",
				results: map[string]Scalar{
					"pass":  Bool(false),
					"score": Int(4),
					"kind":  Label("y"),
				},
			})),
	}
	report := runReport(t, newReportDataset(t, cases), func(ctx context.Context, in string) (string, error) {
		IncrementMetric(ctx, "calls", 1)
		return in, nil
	})

	if report.CaseGroups() != nil {
		t.Fatalf("CaseGroups should be nil for a single-run report")
	}

	avg := report.Averages()
	if avg == nil {
		t.Fatal("Averages should be non-nil with cases")
	}
	if avg.Name != "Averages" {
		t.Fatalf("aggregate name = %q, want %q", avg.Name, "Averages")
	}
	if got := avg.Scores["score"]; got != 3 {
		t.Fatalf("avg score = %v, want 3", got)
	}
	if got := avg.Metrics["calls"]; got != 1 {
		t.Fatalf("avg metric calls = %v, want 1", got)
	}
	// One pass, one fail -> pass rate 0.5.
	if avg.Assertions == nil {
		t.Fatal("Assertions should be non-nil when assertions are present")
	}
	if *avg.Assertions != 0.5 {
		t.Fatalf("assertion pass rate = %v, want 0.5", *avg.Assertions)
	}
	// Each label value occurs once across two cases -> 0.5 fraction each.
	dist := avg.Labels["kind"]
	if dist["x"] != 0.5 || dist["y"] != 0.5 {
		t.Fatalf("label distribution = %v, want x=0.5,y=0.5", dist)
	}
	if avg.TaskDuration < 0 {
		t.Fatalf("TaskDuration should be non-negative, got %v", avg.TaskDuration)
	}
	if avg.TotalDuration < 0 {
		t.Fatalf("TotalDuration should be non-negative, got %v", avg.TotalDuration)
	}
}

func TestAveragesNoAssertions(t *testing.T) {
	cases := []Case[string, string, string]{
		NewCase[string, string, string]("hi", WithCaseName[string, string, string]("a"),
			WithCaseEvaluators[string, string, string](reportEmitEvaluator{
				name:    "e",
				results: map[string]Scalar{"score": Float(1.5)},
			})),
	}
	avg := runReport(t, newReportDataset(t, cases), reportOKTask).Averages()
	if avg == nil {
		t.Fatal("Averages should be non-nil")
	}
	if avg.Assertions != nil {
		t.Fatalf("Assertions should be nil when no assertions present, got %v", *avg.Assertions)
	}
	if got := avg.Scores["score"]; got != 1.5 {
		t.Fatalf("avg score = %v, want 1.5", got)
	}
}

func TestCaseGroupsMultiRun(t *testing.T) {
	cases := []Case[string, string, string]{
		NewCase[string, string, string]("in-a",
			WithCaseName[string, string, string]("a"),
			WithMetadata[string, string, string]("meta-a"),
			WithExpectedOutput[string, string, string]("exp-a"),
			WithCaseEvaluators[string, string, string](reportEmitEvaluator{
				name:    "e",
				results: map[string]Scalar{"pass": Bool(true), "score": Int(10)},
			})),
		NewCase[string, string, string]("in-b",
			WithCaseName[string, string, string]("b"),
			WithCaseEvaluators[string, string, string](reportEmitEvaluator{
				name:    "e",
				results: map[string]Scalar{"pass": Bool(true), "score": Int(20)},
			})),
	}
	report := runReport(t, newReportDataset(t, cases), reportOKTask, WithRepeat[string, string, string](2))

	groups := report.CaseGroups()
	if groups == nil {
		t.Fatal("CaseGroups should be populated for a multi-run report")
	}
	if len(groups) != 2 {
		t.Fatalf("len(groups) = %d, want 2", len(groups))
	}

	g := groups[0]
	if g.Name != "a" {
		t.Fatalf("first group name = %q, want %q", g.Name, "a")
	}
	if len(g.Runs) != 2 {
		t.Fatalf("group a runs = %d, want 2", len(g.Runs))
	}
	if len(g.Failures) != 0 {
		t.Fatalf("group a failures = %d, want 0", len(g.Failures))
	}
	// Group identity fields come from the first run.
	if g.Inputs != "in-a" {
		t.Fatalf("group Inputs = %q, want %q", g.Inputs, "in-a")
	}
	if !g.HasMetadata || g.Metadata != "meta-a" {
		t.Fatalf("group Metadata = %q (has=%v), want %q", g.Metadata, g.HasMetadata, "meta-a")
	}
	if !g.HasExpectedOutput || g.ExpectedOutput != "exp-a" {
		t.Fatalf("group ExpectedOutput = %q (has=%v), want %q", g.ExpectedOutput, g.HasExpectedOutput, "exp-a")
	}

	// Summary averages the two identical runs of group a.
	if g.Summary.Name != "Averages" {
		t.Fatalf("summary name = %q, want %q", g.Summary.Name, "Averages")
	}
	if got := g.Summary.Scores["score"]; got != 10 {
		t.Fatalf("group a summary score = %v, want 10", got)
	}
	if g.Summary.Assertions == nil || *g.Summary.Assertions != 1 {
		t.Fatalf("group a summary assertions = %v, want 1", g.Summary.Assertions)
	}

	// Per-run report case names carry the run index and source case name.
	if got := report.Cases[0].SourceCaseName; got != "a" {
		t.Fatalf("first case SourceCaseName = %q, want %q", got, "a")
	}
	if got := report.Cases[0].Name; got != "a [1/2]" {
		t.Fatalf("first case Name = %q, want %q", got, "a [1/2]")
	}
}

func TestCaseGroupsGroupWithRunsAndFailures(t *testing.T) {
	// The task fails on the second run of case a only, so group a has both a
	// successful run and a failure.
	var runs int
	task := func(ctx context.Context, in string) (string, error) {
		runs++
		if in == "boom" && runs == 2 {
			return "", fmt.Errorf("kaboom")
		}
		return in, nil
	}

	cases := []Case[string, string, string]{
		NewCase[string, string, string]("boom",
			WithCaseName[string, string, string]("a"),
			WithCaseEvaluators[string, string, string](reportEmitEvaluator{
				name:    "e",
				results: map[string]Scalar{"pass": Bool(true), "score": Int(5)},
			})),
	}
	report := runReport(t, newReportDataset(t, cases), task,
		WithRepeat[string, string, string](2), WithMaxConcurrency[string, string, string](1))

	groups := report.CaseGroups()
	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(groups))
	}
	g := groups[0]
	if len(g.Runs) != 1 {
		t.Fatalf("group runs = %d, want 1", len(g.Runs))
	}
	if len(g.Failures) != 1 {
		t.Fatalf("group failures = %d, want 1", len(g.Failures))
	}
	if g.Failures[0].SourceCaseName != "a" {
		t.Fatalf("failure SourceCaseName = %q, want %q", g.Failures[0].SourceCaseName, "a")
	}
	// Identity still comes from the (single) run.
	if g.Inputs != "boom" {
		t.Fatalf("group Inputs = %q, want %q", g.Inputs, "boom")
	}
	// Summary averages only the successful run.
	if got := g.Summary.Scores["score"]; got != 5 {
		t.Fatalf("summary score = %v, want 5", got)
	}

	// Averages goes through CaseGroups and includes this group (it has a run).
	avg := report.Averages()
	if avg == nil {
		t.Fatal("Averages should be non-nil")
	}
	if got := avg.Scores["score"]; got != 5 {
		t.Fatalf("overall avg score = %v, want 5", got)
	}
}

func TestCaseGroupsIdentityFromFirstFailure(t *testing.T) {
	// Every run of the case fails, so the group only has failures and its
	// identity must come from the first failure.
	task := func(_ context.Context, _ string) (string, error) {
		return "", fmt.Errorf("always fails")
	}
	cases := []Case[string, string, string]{
		NewCase[string, string, string]("only-fail-input",
			WithCaseName[string, string, string]("a"),
			WithMetadata[string, string, string]("fmeta"),
			WithExpectedOutput[string, string, string]("fexp")),
	}
	report := runReport(t, newReportDataset(t, cases), task, WithRepeat[string, string, string](2))

	groups := report.CaseGroups()
	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(groups))
	}
	g := groups[0]
	if len(g.Runs) != 0 {
		t.Fatalf("group runs = %d, want 0", len(g.Runs))
	}
	if len(g.Failures) != 2 {
		t.Fatalf("group failures = %d, want 2", len(g.Failures))
	}
	if g.Inputs != "only-fail-input" {
		t.Fatalf("group Inputs = %q, want %q", g.Inputs, "only-fail-input")
	}
	if !g.HasMetadata || g.Metadata != "fmeta" {
		t.Fatalf("group Metadata = %q (has=%v), want %q", g.Metadata, g.HasMetadata, "fmeta")
	}
	if !g.HasExpectedOutput || g.ExpectedOutput != "fexp" {
		t.Fatalf("group ExpectedOutput = %q (has=%v), want %q", g.ExpectedOutput, g.HasExpectedOutput, "fexp")
	}
	// A group with only failures contributes an empty-named "Averages" summary.
	if g.Summary.Name != "Averages" || g.Summary.Scores != nil {
		t.Fatalf("empty group summary = %+v", g.Summary)
	}
}

func TestAveragesMultiRunWholeGroupOnlyFailures(t *testing.T) {
	// Group a always fails; group b always succeeds. Averages must skip group a
	// entirely and reflect only group b.
	task := func(_ context.Context, in string) (string, error) {
		if in == "fail" {
			return "", fmt.Errorf("nope")
		}
		return in, nil
	}
	cases := []Case[string, string, string]{
		NewCase[string, string, string]("fail",
			WithCaseName[string, string, string]("a"),
			WithCaseEvaluators[string, string, string](reportEmitEvaluator{
				name:    "e",
				results: map[string]Scalar{"score": Int(100)},
			})),
		NewCase[string, string, string]("ok",
			WithCaseName[string, string, string]("b"),
			WithCaseEvaluators[string, string, string](reportEmitEvaluator{
				name:    "e",
				results: map[string]Scalar{"pass": Bool(true), "score": Int(7)},
			})),
	}
	report := runReport(t, newReportDataset(t, cases), task, WithRepeat[string, string, string](2))

	avg := report.Averages()
	if avg == nil {
		t.Fatal("Averages should be non-nil; group b has runs")
	}
	// Only group b's score should be reflected; group a (failures only) skipped.
	if got, ok := avg.Scores["score"]; !ok || got != 7 {
		t.Fatalf("overall avg score = %v (present=%v), want 7", got, ok)
	}
	if avg.Assertions == nil || *avg.Assertions != 1 {
		t.Fatalf("overall assertions = %v, want 1", avg.Assertions)
	}
}

func TestAveragesMultiRunAllGroupsOnlyFailures(t *testing.T) {
	// Every group has only failures -> Averages returns nil.
	task := func(_ context.Context, _ string) (string, error) {
		return "", fmt.Errorf("nope")
	}
	cases := []Case[string, string, string]{
		NewCase[string, string, string]("x", WithCaseName[string, string, string]("a")),
	}
	report := runReport(t, newReportDataset(t, cases), task, WithRepeat[string, string, string](2))

	if report.CaseGroups() == nil {
		t.Fatal("CaseGroups should be populated when failures carry a SourceCaseName")
	}
	if avg := report.Averages(); avg != nil {
		t.Fatalf("Averages should be nil when all groups have only failures, got %+v", avg)
	}
}

func TestAveragesMultiRunLabelDistributionAndMetrics(t *testing.T) {
	// One case, two runs producing different labels and a metric, so the
	// per-group distribution is 0.5/0.5 and the metric averages cleanly.
	var runs int
	task := func(ctx context.Context, in string) (string, error) {
		runs++
		IncrementMetric(ctx, "tokens", 4)
		return in, nil
	}
	labelFor := func() Scalar {
		if runs%2 == 1 {
			return Label("first")
		}
		return Label("second")
	}
	// Use a dataset-level evaluator that reads runs at evaluate time. To keep it
	// deterministic per run we instead encode the label in the case via repeat:
	// each run gets a distinct label by alternating.
	cases := []Case[string, string, string]{
		NewCase[string, string, string]("only",
			WithCaseName[string, string, string]("a"),
			WithCaseEvaluators[string, string, string](reportAltLabel{fn: labelFor})),
	}
	report := runReport(t, newReportDataset(t, cases), task,
		WithRepeat[string, string, string](2), WithMaxConcurrency[string, string, string](1))

	groups := report.CaseGroups()
	if len(groups) != 1 {
		t.Fatalf("len(groups) = %d, want 1", len(groups))
	}
	dist := groups[0].Summary.Labels["kind"]
	if dist["first"] != 0.5 || dist["second"] != 0.5 {
		t.Fatalf("group label distribution = %v, want first=0.5,second=0.5", dist)
	}

	avg := report.Averages()
	if avg == nil {
		t.Fatal("Averages should be non-nil")
	}
	// Averaging the single group's distribution reproduces the 0.5/0.5 split.
	adist := avg.Labels["kind"]
	if adist["first"] != 0.5 || adist["second"] != 0.5 {
		t.Fatalf("overall label distribution = %v, want first=0.5,second=0.5", adist)
	}
	if got := avg.Metrics["tokens"]; got != 4 {
		t.Fatalf("overall avg metric tokens = %v, want 4", got)
	}
	if avg.TaskDuration < 0 || avg.TotalDuration < 0 {
		t.Fatalf("durations should be non-negative: task=%v total=%v", avg.TaskDuration, avg.TotalDuration)
	}
}

// reportAltLabel emits a "kind" label decided by fn at evaluate time, used to
// give successive runs of the same case distinct labels.
type reportAltLabel struct {
	fn func() Scalar
}

func (a reportAltLabel) Evaluate(_ context.Context, _ *EvaluatorContext[string, string, string]) (EvaluatorOutput, error) {
	return ScalarMapOutput{"kind": a.fn()}, nil
}

func (a reportAltLabel) Spec() EvaluatorSpec { return NewSpec("alt") }

func TestAveragesDatasetLevelMetricEvaluator(t *testing.T) {
	// Metrics recorded during the task flow into the evaluator context and are
	// averaged into the report metrics.
	task := func(ctx context.Context, in string) (string, error) {
		IncrementMetric(ctx, "m", 3)
		return in, nil
	}
	cases := []Case[string, string, string]{
		NewCase[string, string, string]("one", WithCaseName[string, string, string]("a")),
		NewCase[string, string, string]("two", WithCaseName[string, string, string]("b")),
	}
	ds := newReportDataset(t, cases, reportMetricEvaluator{metricName: "m", resultName: "mscore"})
	report := runReport(t, ds, task)

	avg := report.Averages()
	if avg == nil {
		t.Fatal("Averages should be non-nil")
	}
	if got := avg.Metrics["m"]; got != 3 {
		t.Fatalf("avg metric m = %v, want 3", got)
	}
	if got := avg.Scores["mscore"]; got != 3 {
		t.Fatalf("avg score mscore = %v, want 3", got)
	}
}
