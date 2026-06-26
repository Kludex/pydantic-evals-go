package evals_test

import (
	"context"
	"errors"
	"sync"
	"testing"

	evals "github.com/Kludex/pydantic-evals-go"
)

// reportScorer emits a score, an assertion and a label under fixed names, with
// values driven by the evaluator's configuration.
type reportScorer struct {
	score float64
	pass  bool
	label string
}

func (e reportScorer) EvaluationName() string { return "scorer" }

func (e reportScorer) Evaluate(_ context.Context, _ *evals.EvaluatorContext[int, int, any]) (evals.Output, error) {
	return evals.Named(
		"score", evals.Score(e.score),
		"passed", evals.Assertion(e.pass),
		"label", evals.Category(e.label),
	), nil
}

// reportIntScorer emits an integer score only, under a distinct report name.
type reportIntScorer struct{ value int }

func (e reportIntScorer) EvaluationName() string { return "intscore" }

func (e reportIntScorer) Evaluate(_ context.Context, _ *evals.EvaluatorContext[int, int, any]) (evals.Output, error) {
	return evals.ScoreInt(e.value), nil
}

// reportOutputLabel emits a label whose value depends on the task output, so
// repeated runs whose output varies produce a distribution of labels. The metric
// it reads is recorded by the task via IncrementMetric.
type reportOutputLabel struct{}

func (reportOutputLabel) EvaluationName() string { return "x" }

func (reportOutputLabel) Evaluate(_ context.Context, ec *evals.EvaluatorContext[int, int, any]) (evals.Output, error) {
	value := "even"
	if ec.Output%2 != 0 {
		value = "odd"
	}
	return evals.Named("label", evals.Category(value)), nil
}

func identityTask(_ context.Context, in int) (int, error) { return in, nil }

// metricTask returns its input unchanged after recording a "tokens" metric on the
// task run, so the resulting ReportCase carries that metric for aggregation.
func metricTask(amount float64) evals.TaskFunc[int, int] {
	return func(ctx context.Context, in int) (int, error) {
		evals.IncrementMetric(ctx, "tokens", amount)
		return in, nil
	}
}

func mustDataset(t *testing.T, name string, cases []evals.Case[int, int, any], evaluators ...evals.Evaluator[int, int, any]) *evals.Dataset[int, int, any] {
	t.Helper()
	ds, err := evals.NewDataset[int, int, any](name, cases, evaluators...)
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}
	return ds
}

func mustEvaluate(t *testing.T, ds *evals.Dataset[int, int, any], task evals.TaskFunc[int, int], cfg ...evals.Config) *evals.EvaluationReport[int, int, any] {
	t.Helper()
	report, err := ds.Evaluate(context.Background(), task, cfg...)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	return report
}

func groupByName(t *testing.T, groups []evals.ReportCaseGroup[int, int, any], name string) evals.ReportCaseGroup[int, int, any] {
	t.Helper()
	for _, g := range groups {
		if g.Name == name {
			return g
		}
	}
	t.Fatalf("group %q not found in %d groups", name, len(groups))
	return evals.ReportCaseGroup[int, int, any]{}
}

func TestAveragesNilForNoCases(t *testing.T) {
	ds := mustDataset(t, "empty", nil)
	report := mustEvaluate(t, ds, identityTask)
	if report.Averages() != nil {
		t.Fatalf("expected nil averages for a report with no cases, got %+v", report.Averages())
	}
	if report.CaseGroups() != nil {
		t.Fatalf("expected nil case groups for an empty single-run report")
	}
}

func TestAveragesSingleRun(t *testing.T) {
	cases := []evals.Case[int, int, any]{
		evals.NewCase[int, int, any](1, evals.WithCaseName[int, int, any]("a"),
			evals.WithCaseEvaluators[int, int, any](reportScorer{score: 0.0, pass: true, label: "x"})),
		evals.NewCase[int, int, any](2, evals.WithCaseName[int, int, any]("b"),
			evals.WithCaseEvaluators[int, int, any](reportScorer{score: 1.0, pass: false, label: "y"})),
	}
	ds := mustDataset(t, "single", cases)

	report := mustEvaluate(t, ds, metricTask(3))

	if report.CaseGroups() != nil {
		t.Fatalf("expected nil case groups for a single-run report")
	}

	avg := report.Averages()
	if avg == nil {
		t.Fatalf("expected non-nil averages")
	}
	if avg.Name != "Averages" {
		t.Fatalf("expected aggregate name %q, got %q", "Averages", avg.Name)
	}

	if got := avg.Scores["score"]; got != 0.5 {
		t.Fatalf("expected averaged score 0.5, got %v", got)
	}
	if got := avg.Metrics["tokens"]; got != 3 {
		t.Fatalf("expected averaged metric 3, got %v", got)
	}

	if avg.Assertions == nil {
		t.Fatalf("expected non-nil assertions pass rate")
	}
	if *avg.Assertions != 0.5 {
		t.Fatalf("expected assertions pass rate 0.5, got %v", *avg.Assertions)
	}

	dist := avg.Labels["label"]
	if dist["x"] != 0.5 || dist["y"] != 0.5 {
		t.Fatalf("expected label distribution {x:0.5, y:0.5}, got %v", dist)
	}

	if avg.TaskDuration < 0 || avg.TotalDuration < 0 {
		t.Fatalf("expected non-negative durations, got task=%v total=%v", avg.TaskDuration, avg.TotalDuration)
	}
}

func TestAveragesAssertionsNilWhenNoAssertions(t *testing.T) {
	cases := []evals.Case[int, int, any]{
		evals.NewCase[int, int, any](1, evals.WithCaseName[int, int, any]("a"),
			evals.WithCaseEvaluators[int, int, any](reportIntScorer{value: 3})),
		evals.NewCase[int, int, any](2, evals.WithCaseName[int, int, any]("b"),
			evals.WithCaseEvaluators[int, int, any](reportIntScorer{value: 5})),
	}
	ds := mustDataset(t, "noassert", cases)

	avg := mustEvaluate(t, ds, identityTask).Averages()
	if avg == nil {
		t.Fatalf("expected non-nil averages")
	}
	if avg.Assertions != nil {
		t.Fatalf("expected nil assertions when no assertions were emitted, got %v", *avg.Assertions)
	}
	if got := avg.Scores["intscore"]; got != 4 {
		t.Fatalf("expected averaged int score 4, got %v", got)
	}
	if len(avg.Labels) != 0 {
		t.Fatalf("expected no labels, got %v", avg.Labels)
	}
}

func TestCaseGroupsPopulatedForRepeat(t *testing.T) {
	cases := []evals.Case[int, int, any]{
		evals.NewCase[int, int, any](1, evals.WithCaseName[int, int, any]("only"),
			evals.WithMetadata[int, int, any]("m"),
			evals.WithExpectedOutput[int, int, any](1),
			evals.WithCaseEvaluators[int, int, any](reportScorer{score: 1.0, pass: true, label: "ok"})),
	}
	ds := mustDataset(t, "repeated", cases)

	report := mustEvaluate(t, ds, metricTask(1), evals.Config{Repeat: 3})

	groups := report.CaseGroups()
	if len(groups) != 1 {
		t.Fatalf("expected exactly 1 group, got %d", len(groups))
	}
	g := groups[0]
	if g.Name != "only" {
		t.Fatalf("expected group name %q, got %q", "only", g.Name)
	}
	if len(g.Runs) != 3 {
		t.Fatalf("expected 3 runs in the group, got %d", len(g.Runs))
	}
	if len(g.Failures) != 0 {
		t.Fatalf("expected no failures in the group, got %d", len(g.Failures))
	}

	if g.Inputs != 1 {
		t.Fatalf("expected group inputs taken from first run (1), got %v", g.Inputs)
	}
	if !g.HasMetadata || g.Metadata != "m" {
		t.Fatalf("expected group metadata %q from first run, got hasMeta=%v meta=%v", "m", g.HasMetadata, g.Metadata)
	}
	if !g.HasExpectedOutput || g.ExpectedOutput != 1 {
		t.Fatalf("expected group expected output 1 from first run, got hasExp=%v exp=%v", g.HasExpectedOutput, g.ExpectedOutput)
	}

	for i, run := range g.Runs {
		if run.SourceCaseName != "only" {
			t.Fatalf("run %d: expected source case name %q, got %q", i, "only", run.SourceCaseName)
		}
	}

	sum := g.Summary
	if sum.Name != "Averages" {
		t.Fatalf("expected summary name %q, got %q", "Averages", sum.Name)
	}
	if sum.Scores["score"] != 1.0 {
		t.Fatalf("expected summary score 1.0, got %v", sum.Scores["score"])
	}
	if sum.Metrics["tokens"] != 1 {
		t.Fatalf("expected summary metric 1, got %v", sum.Metrics["tokens"])
	}
	if sum.Assertions == nil || *sum.Assertions != 1.0 {
		t.Fatalf("expected summary assertions pass rate 1.0, got %v", sum.Assertions)
	}
	if sum.Labels["label"]["ok"] != 1.0 {
		t.Fatalf("expected summary label ok=1.0, got %v", sum.Labels["label"])
	}

	avg := report.Averages()
	if avg == nil {
		t.Fatalf("expected non-nil averages for repeat experiment")
	}
	if avg.Scores["score"] != 1.0 {
		t.Fatalf("expected averaged score 1.0, got %v", avg.Scores["score"])
	}
	if avg.Metrics["tokens"] != 1 {
		t.Fatalf("expected averaged metric 1, got %v", avg.Metrics["tokens"])
	}
	if avg.Assertions == nil || *avg.Assertions != 1.0 {
		t.Fatalf("expected averaged assertions 1.0, got %v", avg.Assertions)
	}
	if avg.Labels["label"]["ok"] != 1.0 {
		t.Fatalf("expected averaged label ok=1.0, got %v", avg.Labels["label"])
	}
	if avg.TaskDuration < 0 || avg.TotalDuration < 0 {
		t.Fatalf("expected non-negative averaged durations")
	}
}

func TestCaseGroupsLabelDistributionSplit(t *testing.T) {
	// Two runs of the same source case; the task output alternates so the
	// evaluator produces a different label each run, giving each label 0.5.
	var mu sync.Mutex
	var n int
	alternatingTask := func(_ context.Context, _ int) (int, error) {
		mu.Lock()
		n++
		out := n
		mu.Unlock()
		return out, nil
	}
	cases := []evals.Case[int, int, any]{
		evals.NewCase[int, int, any](0, evals.WithCaseName[int, int, any]("alt"),
			evals.WithCaseEvaluators[int, int, any](reportOutputLabel{})),
	}
	ds := mustDataset(t, "alt", cases)

	report := mustEvaluate(t, ds, alternatingTask, evals.Config{Repeat: 2, MaxConcurrency: 1})

	groups := report.CaseGroups()
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	dist := groups[0].Summary.Labels["label"]
	if dist["odd"] != 0.5 || dist["even"] != 0.5 {
		t.Fatalf("expected label distribution {odd:0.5, even:0.5}, got %v", dist)
	}

	avg := report.Averages()
	if avg.Labels["label"]["odd"] != 0.5 || avg.Labels["label"]["even"] != 0.5 {
		t.Fatalf("expected averaged label distribution {odd:0.5, even:0.5}, got %v", avg.Labels["label"])
	}
}

func TestCaseGroupsMixedRunsAndFailures(t *testing.T) {
	// The task fails exactly the first time it sees input 1, so the "flaky" group
	// (input 1, repeated twice) ends with one failure and one successful run,
	// while the "solid" group (input 2) is all successes.
	var mu sync.Mutex
	failed := false
	flakyTask := func(_ context.Context, in int) (int, error) {
		mu.Lock()
		defer mu.Unlock()
		if in == 1 && !failed {
			failed = true
			return 0, errors.New("boom")
		}
		return in, nil
	}

	cases := []evals.Case[int, int, any]{
		evals.NewCase[int, int, any](1, evals.WithCaseName[int, int, any]("flaky"),
			evals.WithCaseEvaluators[int, int, any](reportScorer{score: 1.0, pass: true, label: "g"})),
		evals.NewCase[int, int, any](2, evals.WithCaseName[int, int, any]("solid"),
			evals.WithCaseEvaluators[int, int, any](reportScorer{score: 0.0, pass: false, label: "b"})),
	}
	ds := mustDataset(t, "mixed", cases)

	report := mustEvaluate(t, ds, flakyTask, evals.Config{Repeat: 2, MaxConcurrency: 1})

	groups := report.CaseGroups()
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}

	flaky := groupByName(t, groups, "flaky")
	solid := groupByName(t, groups, "solid")

	if len(flaky.Runs) != 1 {
		t.Fatalf("expected flaky group to have 1 successful run, got %d", len(flaky.Runs))
	}
	if len(flaky.Failures) != 1 {
		t.Fatalf("expected flaky group to have 1 failure, got %d", len(flaky.Failures))
	}
	if flaky.Failures[0].ErrorMessage != "boom" {
		t.Fatalf("expected failure message %q, got %q", "boom", flaky.Failures[0].ErrorMessage)
	}
	if flaky.Failures[0].SourceCaseName != "flaky" {
		t.Fatalf("expected failure source case name %q, got %q", "flaky", flaky.Failures[0].SourceCaseName)
	}

	if len(solid.Runs) != 2 || len(solid.Failures) != 0 {
		t.Fatalf("expected solid group to have 2 runs and 0 failures, got runs=%d failures=%d", len(solid.Runs), len(solid.Failures))
	}

	// flaky's summary derives from its single successful run.
	if flaky.Summary.Scores["score"] != 1.0 {
		t.Fatalf("expected flaky summary score 1.0, got %v", flaky.Summary.Scores["score"])
	}
	// The overall averages span both groups: scores 1.0 and 0.0 -> 0.5.
	avg := report.Averages()
	if avg == nil {
		t.Fatalf("expected non-nil averages")
	}
	if avg.Scores["score"] != 0.5 {
		t.Fatalf("expected averaged score 0.5 across groups, got %v", avg.Scores["score"])
	}
	if avg.Assertions == nil || *avg.Assertions != 0.5 {
		t.Fatalf("expected averaged assertions 0.5, got %v", avg.Assertions)
	}
}

func TestAveragesSkipsFailureOnlyGroup(t *testing.T) {
	// "good" always succeeds; "bad" always fails. With repeat, "bad" becomes a
	// group with only failures, which Averages must skip; the overall average
	// therefore reflects only "good".
	failOnBad := func(_ context.Context, in int) (int, error) {
		if in == 99 {
			return 0, errors.New("always fails")
		}
		return in, nil
	}

	cases := []evals.Case[int, int, any]{
		evals.NewCase[int, int, any](1, evals.WithCaseName[int, int, any]("good"),
			evals.WithCaseEvaluators[int, int, any](reportScorer{score: 0.8, pass: true, label: "g"})),
		evals.NewCase[int, int, any](99, evals.WithCaseName[int, int, any]("bad"),
			evals.WithCaseEvaluators[int, int, any](reportScorer{score: 0.0, pass: false, label: "b"})),
	}
	ds := mustDataset(t, "failgroup", cases)

	report := mustEvaluate(t, ds, failOnBad, evals.Config{Repeat: 2})

	groups := report.CaseGroups()
	if len(groups) != 2 {
		t.Fatalf("expected 2 groups, got %d", len(groups))
	}
	bad := groupByName(t, groups, "bad")
	if len(bad.Runs) != 0 || len(bad.Failures) != 2 {
		t.Fatalf("expected bad group to be failure-only (0 runs, 2 failures), got runs=%d failures=%d", len(bad.Runs), len(bad.Failures))
	}
	// Failure-only group: its fields come from the first failure.
	if bad.Inputs != 99 {
		t.Fatalf("expected failure-only group inputs 99 from first failure, got %v", bad.Inputs)
	}
	// A failure-only group's summary has no runs, so it averages to an empty aggregate.
	if bad.Summary.Assertions != nil {
		t.Fatalf("expected failure-only group summary to have nil assertions, got %v", *bad.Summary.Assertions)
	}

	avg := report.Averages()
	if avg == nil {
		t.Fatalf("expected non-nil averages (the 'good' group still has runs)")
	}
	// Only the "good" group contributes -> score 0.8, assertions 1.0.
	if avg.Scores["score"] != 0.8 {
		t.Fatalf("expected averaged score 0.8 (failure-only group skipped), got %v", avg.Scores["score"])
	}
	if avg.Assertions == nil || *avg.Assertions != 1.0 {
		t.Fatalf("expected averaged assertions 1.0 (failure-only group skipped), got %v", avg.Assertions)
	}
}

func TestAveragesNilWhenAllGroupsFail(t *testing.T) {
	// Every case fails on every run, so all groups are failure-only and Averages
	// returns nil even though CaseGroups is populated.
	alwaysFail := func(_ context.Context, _ int) (int, error) {
		return 0, errors.New("nope")
	}
	cases := []evals.Case[int, int, any]{
		evals.NewCase[int, int, any](1, evals.WithCaseName[int, int, any]("a")),
		evals.NewCase[int, int, any](2, evals.WithCaseName[int, int, any]("b")),
	}
	ds := mustDataset(t, "allfail", cases)

	report := mustEvaluate(t, ds, alwaysFail, evals.Config{Repeat: 2})

	groups := report.CaseGroups()
	if len(groups) != 2 {
		t.Fatalf("expected 2 failure-only groups, got %d", len(groups))
	}
	for _, g := range groups {
		if len(g.Runs) != 0 {
			t.Fatalf("expected group %q to have no runs, got %d", g.Name, len(g.Runs))
		}
	}
	if report.Averages() != nil {
		t.Fatalf("expected nil averages when all groups are failure-only, got %+v", report.Averages())
	}
}
