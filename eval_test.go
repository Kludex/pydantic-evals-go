package evals_test

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sort"
	"testing"
	"time"

	evals "github.com/Kludex/pydantic-evals-go"
)

// ceEcho returns its input unchanged.
func ceEcho(_ context.Context, in string) (string, error) { return in, nil }

// ceLenScore is a simple evaluator returning a Score of the output length.
type ceLenScore struct{}

func (ceLenScore) Evaluate(_ context.Context, ec *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	return evals.Score(float64(len(ec.Output))), nil
}

// cePass always returns a passing assertion. It implements no optional
// interface, so its report name defaults to the Go type name "cePass".
type cePass struct{}

func (cePass) Evaluate(_ context.Context, _ *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	return evals.Assertion(true), nil
}

// ceNamed overrides its report name via EvaluationName.
type ceNamed struct{}

func (ceNamed) Evaluate(_ context.Context, _ *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	return evals.Assertion(true), nil
}
func (ceNamed) EvaluationName() string { return "Custom" }

// ceEmptyNamed returns an empty EvaluationName, which falls back to the type name.
type ceEmptyNamed struct{}

func (ceEmptyNamed) Evaluate(_ context.Context, _ *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	return evals.Assertion(true), nil
}
func (ceEmptyNamed) EvaluationName() string { return "" }

// ceVersioned tags its results with a version.
type ceVersioned struct{}

func (ceVersioned) Evaluate(_ context.Context, _ *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	return evals.Assertion(true), nil
}
func (ceVersioned) EvaluatorVersion() string { return "v3" }

// ceVersionedFail returns an error and is versioned, so the failure carries a version.
type ceVersionedFail struct{}

func (ceVersionedFail) Evaluate(_ context.Context, _ *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	return nil, errors.New("boom")
}
func (ceVersionedFail) EvaluatorVersion() string { return "v9" }

// ceNilOutput returns a nil Output, which is an error path.
type ceNilOutput struct{}

func (ceNilOutput) Evaluate(_ context.Context, _ *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	return nil, nil
}

// ceInf returns a non-finite score.
type ceInf struct{}

func (ceInf) Evaluate(_ context.Context, _ *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	return evals.Score(math.Inf(1)), nil
}

// ceNaN returns a NaN score.
type ceNaN struct{}

func (ceNaN) Evaluate(_ context.Context, _ *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	return evals.Score(math.NaN()), nil
}

// ceErr returns a plain error.
type ceErr struct{}

func (ceErr) Evaluate(_ context.Context, _ *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	return nil, errors.New("eval failed")
}

// ceDup emits an assertion under a fixed shared name to test suffixing.
type ceDup struct{ pass bool }

func (d ceDup) Evaluate(_ context.Context, _ *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	return evals.Assertion(d.pass), nil
}
func (ceDup) EvaluationName() string { return "Dup" }

// ceKinds returns several named results of every kind, plus reasons.
type ceKinds struct{}

func (ceKinds) Evaluate(_ context.Context, _ *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	return evals.Named(
		"score", evals.Score(0.5).WithReason("half"),
		"intScore", evals.ScoreInt(7),
		"assert", evals.Assertion(true),
		"label", evals.Category("good"),
	), nil
}

// ceNoResult records nothing.
type ceNoResult struct{}

func (ceNoResult) Evaluate(_ context.Context, _ *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	return evals.NoResult(), nil
}

// ceMetrics surfaces a recorded metric and attribute via the context.
type ceMetrics struct{}

func (ceMetrics) Evaluate(_ context.Context, ec *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	return evals.Named(
		"calls", evals.ScoreInt(int(ec.Metrics["calls"])),
		"hasAttr", evals.Assertion(ec.Attributes["model"] == "gpt"),
	), nil
}

// ceReasonScore returns a single result with a reason.
type ceReasonScore struct{}

func (ceReasonScore) Evaluate(_ context.Context, _ *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	return evals.Score(1).WithReason("because"), nil
}

// ceCtxCheck asserts that every EvaluatorContext field was populated.
type ceCtxCheck struct{ t *testing.T }

func (e ceCtxCheck) Evaluate(_ context.Context, ec *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	e.t.Helper()
	if ec.Name != "only" {
		e.t.Fatalf("name = %q", ec.Name)
	}
	if ec.Inputs != "hello" || ec.Output != "hello" {
		e.t.Fatalf("inputs/output = %q/%q", ec.Inputs, ec.Output)
	}
	if !ec.HasExpectedOutput || ec.ExpectedOutput != "hello" {
		e.t.Fatalf("expected = %q (%v)", ec.ExpectedOutput, ec.HasExpectedOutput)
	}
	if !ec.HasMetadata || ec.Metadata != "m" {
		e.t.Fatalf("metadata = %v (%v)", ec.Metadata, ec.HasMetadata)
	}
	if ec.Duration < 0 {
		e.t.Fatalf("duration = %v", ec.Duration)
	}
	return evals.Assertion(true), nil
}

// ceLifecycle enriches the evaluator context.
type ceLifecycle struct {
	evals.BaseLifecycle[string, string, any]
}

func (ceLifecycle) PrepareContext(_ context.Context, ec *evals.EvaluatorContext[string, string, any]) (*evals.EvaluatorContext[string, string, any], error) {
	ec.Attributes["model"] = "gpt"
	ec.Metrics["calls"] = 4
	return ec, nil
}

// ceFailingSetup fails during Setup.
type ceFailingSetup struct {
	evals.BaseLifecycle[string, string, any]
}

func (ceFailingSetup) Setup(context.Context) error { return errors.New("nope") }

func ceSuite() evals.Suite[string, string, any] {
	return evals.For[string, string, any]()
}

func TestBuilderEvaluateBasic(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("greet",
		s.Case("hi").Name("first").Expect("hi").Eval(s.EqualsExpected()),
		s.Case("yo").Name("second").Expect("yo"),
	).With(ceLenScore{}, cePass{})

	if ds.Name != "greet" {
		t.Fatalf("dataset name = %q", ds.Name)
	}
	if len(ds.Cases) != 2 {
		t.Fatalf("cases = %d", len(ds.Cases))
	}

	report, err := ds.Evaluate(context.Background(), ceEcho)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if report.Name != "task" {
		t.Fatalf("report name = %q", report.Name)
	}
	if len(report.Cases) != 2 || len(report.Failures) != 0 {
		t.Fatalf("cases=%d failures=%d", len(report.Cases), len(report.Failures))
	}

	byName := map[string]evals.ReportCase[string, string, any]{}
	for _, c := range report.Cases {
		byName[c.Name] = c
	}
	first, ok := byName["first"]
	if !ok {
		t.Fatalf("missing first case")
	}
	if first.Output != "hi" {
		t.Fatalf("output = %q", first.Output)
	}
	if got := first.Scores["ceLenScore"].Value.String(); got != "2" {
		t.Fatalf("ceLenScore = %q", got)
	}
	if _, ok := first.Assertions["cePass"]; !ok {
		t.Fatalf("missing cePass assertion: %v", first.Assertions)
	}
	if _, ok := first.Assertions["EqualsExpected"]; !ok {
		t.Fatalf("missing EqualsExpected: %v", first.Assertions)
	}
}

func TestNewCaseNewDatasetPath(t *testing.T) {
	c := evals.NewCase[string, string, any]("in",
		evals.WithCaseName[string, string, any]("c1"),
		evals.WithExpectedOutput[string, string, any]("in"),
		evals.WithMetadata[string, string, any](map[string]any{"k": "v"}),
		evals.WithCaseEvaluators[string, string, any](cePass{}),
	)
	if c.Name != "c1" || !c.HasExpectedOutput || !c.HasMetadata {
		t.Fatalf("case option flags wrong: %+v", c)
	}

	ds, err := evals.NewDataset("d", []evals.Case[string, string, any]{c}, ceLenScore{})
	if err != nil {
		t.Fatalf("new dataset: %v", err)
	}
	report, err := ds.Evaluate(context.Background(), ceEcho)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(report.Cases) != 1 {
		t.Fatalf("cases = %d", len(report.Cases))
	}
	got := report.Cases[0]
	if _, ok := got.Assertions["cePass"]; !ok {
		t.Fatalf("missing case evaluator result")
	}
	if _, ok := got.Scores["ceLenScore"]; !ok {
		t.Fatalf("missing dataset evaluator result")
	}
}

func TestNewDatasetDuplicateNameError(t *testing.T) {
	cases := []evals.Case[string, string, any]{
		evals.NewCase[string, string, any]("a", evals.WithCaseName[string, string, any]("dup")),
		evals.NewCase[string, string, any]("b", evals.WithCaseName[string, string, any]("dup")),
	}
	_, err := evals.NewDataset("d", cases)
	if err == nil {
		t.Fatal("expected duplicate name error")
	}
	if err.Error() != `duplicate case name: "dup"` {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestNewDatasetUnnamedCasesAllowed(t *testing.T) {
	cases := []evals.Case[string, string, any]{
		evals.NewCase[string, string, any]("a"),
		evals.NewCase[string, string, any]("b"),
	}
	ds, err := evals.NewDataset("d", cases)
	if err != nil {
		t.Fatalf("unnamed cases should be allowed: %v", err)
	}
	report, err := ds.Evaluate(context.Background(), ceEcho)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	names := map[string]bool{}
	for _, c := range report.Cases {
		names[c.Name] = true
	}
	if !names["Case 1"] || !names["Case 2"] {
		t.Fatalf("expected generic names, got %v", names)
	}
}

func TestSuiteDatasetPanicsOnDuplicate(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic on duplicate case name")
		}
		err, ok := r.(error)
		if !ok || err.Error() != `duplicate case name: "x"` {
			t.Fatalf("panic = %v", r)
		}
	}()
	s := ceSuite()
	s.Dataset("d", s.Case("a").Name("x"), s.Case("b").Name("x"))
}

func TestAddCaseAndDuplicate(t *testing.T) {
	ds, err := evals.NewDataset("d", []evals.Case[string, string, any]{
		evals.NewCase[string, string, any]("a", evals.WithCaseName[string, string, any]("c1")),
	})
	if err != nil {
		t.Fatalf("new dataset: %v", err)
	}
	if err := ds.AddCase(evals.NewCase[string, string, any]("b", evals.WithCaseName[string, string, any]("c2"))); err != nil {
		t.Fatalf("add case: %v", err)
	}
	if err := ds.AddCase(evals.NewCase[string, string, any]("c")); err != nil {
		t.Fatalf("add unnamed case: %v", err)
	}
	err = ds.AddCase(evals.NewCase[string, string, any]("d", evals.WithCaseName[string, string, any]("c1")))
	if err == nil || err.Error() != `duplicate case name: "c1"` {
		t.Fatalf("expected duplicate error, got %v", err)
	}
	if len(ds.Cases) != 3 {
		t.Fatalf("cases = %d", len(ds.Cases))
	}
}

func TestAddEvaluatorAndForCase(t *testing.T) {
	ds, err := evals.NewDataset("d", []evals.Case[string, string, any]{
		evals.NewCase[string, string, any]("a", evals.WithCaseName[string, string, any]("c1")),
		evals.NewCase[string, string, any]("bb", evals.WithCaseName[string, string, any]("c2")),
	})
	if err != nil {
		t.Fatalf("new dataset: %v", err)
	}
	ds.AddEvaluator(ceLenScore{})
	if err := ds.AddEvaluatorForCase("c1", cePass{}); err != nil {
		t.Fatalf("add for case: %v", err)
	}
	if err := ds.AddEvaluatorForCase("missing", cePass{}); err == nil {
		t.Fatal("expected not-found error")
	} else if err.Error() != `case "missing" not found in the dataset` {
		t.Fatalf("error = %q", err.Error())
	}

	report, err := ds.Evaluate(context.Background(), ceEcho)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	byName := map[string]evals.ReportCase[string, string, any]{}
	for _, c := range report.Cases {
		byName[c.Name] = c
	}
	if _, ok := byName["c1"].Assertions["cePass"]; !ok {
		t.Fatalf("c1 should have case evaluator")
	}
	if _, ok := byName["c2"].Assertions["cePass"]; ok {
		t.Fatalf("c2 should not have c1's case evaluator")
	}
	if _, ok := byName["c2"].Scores["ceLenScore"]; !ok {
		t.Fatalf("c2 should have dataset evaluator")
	}
}

func TestConfigNameTaskNameMetadata(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("a").Name("only"))
	meta := map[string]any{"run": "smoke"}
	report, err := ds.Evaluate(context.Background(), ceEcho, evals.Config{
		Name:           "exp1",
		TaskName:       "mytask",
		MaxConcurrency: 1,
		Metadata:       meta,
	})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if report.Name != "exp1" {
		t.Fatalf("report name = %q", report.Name)
	}
	if report.ExperimentMetadata["run"] != "smoke" {
		t.Fatalf("metadata = %v", report.ExperimentMetadata)
	}
}

func TestConfigNameDefaultsToTaskName(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("a").Name("only"))
	report, err := ds.Evaluate(context.Background(), ceEcho, evals.Config{TaskName: "scorer"})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if report.Name != "scorer" {
		t.Fatalf("report name = %q", report.Name)
	}
}

func TestConfigRepeatNegativeError(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("a").Name("only"))
	_, err := ds.Evaluate(context.Background(), ceEcho, evals.Config{Repeat: -1})
	if err == nil || err.Error() != "repeat must be >= 0, got -1" {
		t.Fatalf("expected repeat error, got %v", err)
	}
}

func TestConfigRepeatZeroIsOne(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("a").Name("only"))
	report, err := ds.Evaluate(context.Background(), ceEcho, evals.Config{Repeat: 0})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(report.Cases) != 1 {
		t.Fatalf("repeat 0 should run once, got %d", len(report.Cases))
	}
	if report.CaseGroups() != nil {
		t.Fatalf("single run should have nil groups")
	}
}

func TestConfigRepeatGroups(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("a").Name("only").Eval(cePass{}))
	report, err := ds.Evaluate(context.Background(), ceEcho, evals.Config{Repeat: 3})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(report.Cases) != 3 {
		t.Fatalf("expected 3 runs, got %d", len(report.Cases))
	}
	names := []string{}
	for _, c := range report.Cases {
		names = append(names, c.Name)
		if c.SourceCaseName != "only" {
			t.Fatalf("source case name = %q", c.SourceCaseName)
		}
	}
	sort.Strings(names)
	want := []string{"only [1/3]", "only [2/3]", "only [3/3]"}
	if fmt.Sprint(names) != fmt.Sprint(want) {
		t.Fatalf("names = %v", names)
	}
	groups := report.CaseGroups()
	if len(groups) != 1 || groups[0].Name != "only" || len(groups[0].Runs) != 3 {
		t.Fatalf("groups = %+v", groups)
	}
}

func TestZeroConfig(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("a").Name("only"))
	report, err := ds.Evaluate(context.Background(), ceEcho, evals.Config{})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if report.Name != "task" || len(report.Cases) != 1 {
		t.Fatalf("zero config: name=%q cases=%d", report.Name, len(report.Cases))
	}
}

func TestTaskErrorBecomesFailure(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("a").Name("boom"))
	failing := func(_ context.Context, _ string) (string, error) {
		return "", errors.New("task exploded")
	}
	report, err := ds.Evaluate(context.Background(), failing)
	if err != nil {
		t.Fatalf("evaluate should not error on task failure: %v", err)
	}
	if len(report.Cases) != 0 || len(report.Failures) != 1 {
		t.Fatalf("cases=%d failures=%d", len(report.Cases), len(report.Failures))
	}
	f := report.Failures[0]
	if f.Name != "boom" {
		t.Fatalf("failure name = %q", f.Name)
	}
	if f.ErrorMessage != "task exploded" {
		t.Fatalf("error message = %q", f.ErrorMessage)
	}
	if f.ErrorType != "errorString" {
		t.Fatalf("error type = %q", f.ErrorType)
	}
}

func TestWrappedTaskErrorType(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("a").Name("boom"))
	failing := func(_ context.Context, _ string) (string, error) {
		return "", fmt.Errorf("wrapped: %w", errors.New("inner"))
	}
	report, err := ds.Evaluate(context.Background(), failing)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	f := report.Failures[0]
	if f.ErrorType != "wrapError" {
		t.Fatalf("error type = %q", f.ErrorType)
	}
	if f.ErrorMessage != "wrapped: inner" {
		t.Fatalf("error message = %q", f.ErrorMessage)
	}
}

func TestEvaluatorResultKinds(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("a").Name("only").Eval(ceKinds{}))
	report, err := ds.Evaluate(context.Background(), ceEcho)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	c := report.Cases[0]
	if got := c.Scores["score"]; got.Value.String() != "0.5" || got.Reason != "half" {
		t.Fatalf("score = %+v", got)
	}
	if got := c.Scores["intScore"].Value.String(); got != "7" {
		t.Fatalf("intScore = %q", got)
	}
	if _, ok := c.Assertions["assert"]; !ok {
		t.Fatalf("missing assert: %v", c.Assertions)
	}
	if got := c.Labels["label"].Value.String(); got != "good" {
		t.Fatalf("label = %q", got)
	}
}

func TestSingleResultWithReason(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("a").Name("only").Eval(ceReasonScore{}))
	report, err := ds.Evaluate(context.Background(), ceEcho)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	r := report.Cases[0].Scores["ceReasonScore"]
	if r.Value.String() != "1" || r.Reason != "because" {
		t.Fatalf("result = %+v", r)
	}
}

func TestNoResultEvaluator(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("a").Name("only").Eval(ceNoResult{}))
	report, err := ds.Evaluate(context.Background(), ceEcho)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	c := report.Cases[0]
	if len(c.Scores)+len(c.Labels)+len(c.Assertions) != 0 {
		t.Fatalf("expected no results, got %+v / %+v / %+v", c.Scores, c.Labels, c.Assertions)
	}
	if len(c.EvaluatorFailures) != 0 {
		t.Fatalf("no-result should not fail: %+v", c.EvaluatorFailures)
	}
}

func TestNilOutputIsFailure(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("a").Name("only").Eval(ceNilOutput{}))
	report, err := ds.Evaluate(context.Background(), ceEcho)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	c := report.Cases[0]
	if len(c.EvaluatorFailures) != 1 {
		t.Fatalf("failures = %+v", c.EvaluatorFailures)
	}
	f := c.EvaluatorFailures[0]
	if f.Name != "ceNilOutput" {
		t.Fatalf("failure name = %q", f.Name)
	}
	if f.ErrorMessage != `evaluator "ceNilOutput" returned a nil output` {
		t.Fatalf("error message = %q", f.ErrorMessage)
	}
}

func TestNonFiniteScoreIsFailure(t *testing.T) {
	tests := []struct {
		name string
		ev   evals.Evaluator[string, string, any]
		want string
	}{
		{"inf", ceInf{}, "evaluator returned a non-finite float score: +Inf"},
		{"nan", ceNaN{}, "evaluator returned a non-finite float score: NaN"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			s := ceSuite()
			ds := s.Dataset("d", s.Case("a").Name("only").Eval(tc.ev))
			report, err := ds.Evaluate(context.Background(), ceEcho)
			if err != nil {
				t.Fatalf("evaluate: %v", err)
			}
			c := report.Cases[0]
			if len(c.EvaluatorFailures) != 1 {
				t.Fatalf("failures = %+v", c.EvaluatorFailures)
			}
			if c.EvaluatorFailures[0].ErrorMessage != tc.want {
				t.Fatalf("error = %q want %q", c.EvaluatorFailures[0].ErrorMessage, tc.want)
			}
		})
	}
}

func TestEvaluatorErrorRecorded(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("a").Name("only").Eval(ceErr{}, cePass{}))
	report, err := ds.Evaluate(context.Background(), ceEcho)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	c := report.Cases[0]
	if len(c.EvaluatorFailures) != 1 {
		t.Fatalf("failures = %+v", c.EvaluatorFailures)
	}
	if c.EvaluatorFailures[0].ErrorMessage != "eval failed" {
		t.Fatalf("error = %q", c.EvaluatorFailures[0].ErrorMessage)
	}
	if c.EvaluatorFailures[0].ErrorType != "errorString" {
		t.Fatalf("error type = %q", c.EvaluatorFailures[0].ErrorType)
	}
	if _, ok := c.Assertions["cePass"]; !ok {
		t.Fatalf("other evaluator should still run: %v", c.Assertions)
	}
}

func TestDuplicateResultNameSuffixing(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("a").Name("only").Eval(
		ceDup{pass: true},
		ceDup{pass: false},
		ceDup{pass: true},
	))
	report, err := ds.Evaluate(context.Background(), ceEcho)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	a := report.Cases[0].Assertions
	if len(a) != 3 {
		t.Fatalf("assertions = %v", a)
	}
	for _, key := range []string{"Dup", "Dup_2", "Dup_3"} {
		if _, ok := a[key]; !ok {
			t.Fatalf("missing %q in %v", key, a)
		}
	}
}

func TestDefaultNameIsGoTypeName(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("a").Name("only").Eval(cePass{}))
	report, err := ds.Evaluate(context.Background(), ceEcho)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if _, ok := report.Cases[0].Assertions["cePass"]; !ok {
		t.Fatalf("default name should be type name: %v", report.Cases[0].Assertions)
	}
}

func TestEvaluationNameOverride(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("a").Name("only").Eval(ceNamed{}))
	report, err := ds.Evaluate(context.Background(), ceEcho)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	a := report.Cases[0].Assertions
	if _, ok := a["Custom"]; !ok {
		t.Fatalf("expected Custom override, got %v", a)
	}
	if _, ok := a["ceNamed"]; ok {
		t.Fatalf("type name should not be used: %v", a)
	}
}

func TestEmptyEvaluationNameFallsBackToTypeName(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("a").Name("only").Eval(ceEmptyNamed{}))
	report, err := ds.Evaluate(context.Background(), ceEcho)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if _, ok := report.Cases[0].Assertions["ceEmptyNamed"]; !ok {
		t.Fatalf("empty EvaluationName should fall back to type name: %v", report.Cases[0].Assertions)
	}
}

func TestEvaluatorVersionOnResult(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("a").Name("only").Eval(ceVersioned{}))
	report, err := ds.Evaluate(context.Background(), ceEcho)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	r, ok := report.Cases[0].Assertions["ceVersioned"]
	if !ok {
		t.Fatalf("missing result: %v", report.Cases[0].Assertions)
	}
	if r.EvaluatorVersion != "v3" {
		t.Fatalf("version = %q", r.EvaluatorVersion)
	}
}

func TestEvaluatorVersionOnFailure(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("a").Name("only").Eval(ceVersionedFail{}))
	report, err := ds.Evaluate(context.Background(), ceEcho)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	c := report.Cases[0]
	if len(c.EvaluatorFailures) != 1 {
		t.Fatalf("failures = %+v", c.EvaluatorFailures)
	}
	f := c.EvaluatorFailures[0]
	if f.EvaluatorVersion != "v9" {
		t.Fatalf("version = %q", f.EvaluatorVersion)
	}
	if f.ErrorMessage != "boom" {
		t.Fatalf("message = %q", f.ErrorMessage)
	}
}

func TestNamedPanicOddArgs(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on odd args")
		}
	}()
	evals.Named("only")
}

func TestNamedPanicNonStringName(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on non-string name")
		}
	}()
	evals.Named(123, evals.Score(1))
}

func TestNamedPanicNonSingleValue(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatal("expected panic on non-single value")
		}
	}()
	evals.Named("x", evals.Named("y", evals.Score(1)))
}

func TestSetAttributeAndIncrementMetric(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("a").Name("only").Eval(ceMetrics{}))
	task := func(ctx context.Context, in string) (string, error) {
		evals.SetAttribute(ctx, "model", "gpt")
		evals.IncrementMetric(ctx, "calls", 2)
		evals.IncrementMetric(ctx, "calls", 1)
		return in, nil
	}
	report, err := ds.Evaluate(context.Background(), task)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	c := report.Cases[0]
	if c.Attributes["model"] != "gpt" {
		t.Fatalf("attributes = %v", c.Attributes)
	}
	if c.Metrics["calls"] != 3 {
		t.Fatalf("metrics = %v", c.Metrics)
	}
	if c.Scores["calls"].Value.String() != "3" {
		t.Fatalf("eval saw metric = %q", c.Scores["calls"].Value.String())
	}
	if _, ok := c.Assertions["hasAttr"]; !ok {
		t.Fatalf("eval should see attribute")
	}
}

func TestIncrementMetricNetZeroDropped(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("a").Name("only"))
	task := func(ctx context.Context, in string) (string, error) {
		evals.IncrementMetric(ctx, "noop", 0)
		evals.IncrementMetric(ctx, "real", 5)
		evals.IncrementMetric(ctx, "real", -5)
		return in, nil
	}
	report, err := ds.Evaluate(context.Background(), task)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	m := report.Cases[0].Metrics
	if _, ok := m["noop"]; ok {
		t.Fatalf("a never-non-zero metric should be dropped: %v", m)
	}
	if v, ok := m["real"]; !ok || v != 0 {
		t.Fatalf("a once-non-zero metric returning to zero is kept as 0: %v", m)
	}
}

func TestSetAttributeNoopOutsideTask(t *testing.T) {
	ctx := context.Background()
	evals.SetAttribute(ctx, "x", 1)
	evals.IncrementMetric(ctx, "y", 1)
}

func TestMaxConcurrencyBounded(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d",
		s.Case("a").Name("c1"),
		s.Case("b").Name("c2"),
		s.Case("c").Name("c3"),
		s.Case("d").Name("c4"),
	)
	report, err := ds.Evaluate(context.Background(), ceEcho, evals.Config{MaxConcurrency: 2})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(report.Cases) != 4 {
		t.Fatalf("cases = %d", len(report.Cases))
	}
}

func TestEvaluateContextCancelled(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("a").Name("only"))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := ds.Evaluate(ctx, ceEcho)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestUnnamedCaseGenericNamesPreserveOrder(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d",
		s.Case("a"),
		s.Case("b").Name("named"),
		s.Case("c"),
	)
	report, err := ds.Evaluate(context.Background(), ceEcho, evals.Config{MaxConcurrency: 1})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	names := map[string]bool{}
	for _, c := range report.Cases {
		names[c.Name] = true
	}
	for _, want := range []string{"Case 1", "named", "Case 3"} {
		if !names[want] {
			t.Fatalf("missing %q in %v", want, names)
		}
	}
}

func TestEvaluateWithLifecycle(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("a").Name("only").Eval(ceMetrics{}))
	newLC := func(_ evals.Case[string, string, any]) evals.Lifecycle[string, string, any] {
		return ceLifecycle{}
	}
	report, err := ds.EvaluateWithLifecycle(context.Background(), ceEcho, newLC)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	c := report.Cases[0]
	if c.Attributes["model"] != "gpt" {
		t.Fatalf("PrepareContext attribute missing: %v", c.Attributes)
	}
	if c.Metrics["calls"] != 4 {
		t.Fatalf("PrepareContext metric = %v", c.Metrics)
	}
}

func TestEvaluateWithLifecycleZeroConfigAndError(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("a").Name("only"))
	newLC := func(_ evals.Case[string, string, any]) evals.Lifecycle[string, string, any] {
		return ceFailingSetup{}
	}
	report, err := ds.EvaluateWithLifecycle(context.Background(), ceEcho, newLC, evals.Config{})
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if len(report.Failures) != 1 {
		t.Fatalf("failures = %+v", report.Failures)
	}
	if report.Failures[0].ErrorMessage != "setup: nope" {
		t.Fatalf("message = %q", report.Failures[0].ErrorMessage)
	}
}

func TestReportAverages(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d",
		s.Case("ab").Name("c1").Eval(ceLenScore{}, cePass{}),
		s.Case("cd").Name("c2").Eval(ceLenScore{}, cePass{}),
	)
	report, err := ds.Evaluate(context.Background(), ceEcho)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	avg := report.Averages()
	if avg == nil {
		t.Fatal("expected averages")
	}
	if avg.Scores["ceLenScore"] != 2 {
		t.Fatalf("avg score = %v", avg.Scores["ceLenScore"])
	}
	if avg.Assertions == nil || *avg.Assertions != 1 {
		t.Fatalf("avg assertions = %v", avg.Assertions)
	}
}

func TestAveragesNilWhenNoCases(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("a").Name("boom"))
	failing := func(_ context.Context, _ string) (string, error) {
		return "", errors.New("x")
	}
	report, err := ds.Evaluate(context.Background(), failing)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	if report.Averages() != nil {
		t.Fatalf("expected nil averages, got %+v", report.Averages())
	}
}

func TestEvaluatorContextFieldsPassedThrough(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("hello").Name("only").Expect("hello").Meta("m").Eval(ceCtxCheck{t: t}))
	if _, err := ds.Evaluate(context.Background(), ceEcho); err != nil {
		t.Fatalf("evaluate: %v", err)
	}
}

func TestCaseBuilderBuild(t *testing.T) {
	s := ceSuite()
	c := s.Case("in").Name("n").Expect("out").Meta("meta").Eval(cePass{}).Build()
	if c.Name != "n" || c.Inputs != "in" || c.ExpectedOutput != "out" {
		t.Fatalf("build = %+v", c)
	}
	if !c.HasExpectedOutput || !c.HasMetadata || c.Metadata != "meta" {
		t.Fatalf("flags = %+v", c)
	}
	if len(c.Evaluators) != 1 {
		t.Fatalf("evaluators = %d", len(c.Evaluators))
	}
}

func TestTaskDurationNonNegative(t *testing.T) {
	s := ceSuite()
	ds := s.Dataset("d", s.Case("a").Name("only"))
	slow := func(ctx context.Context, in string) (string, error) {
		time.Sleep(time.Millisecond)
		return in, nil
	}
	report, err := ds.Evaluate(context.Background(), slow)
	if err != nil {
		t.Fatalf("evaluate: %v", err)
	}
	c := report.Cases[0]
	if c.TaskDuration <= 0 {
		t.Fatalf("task duration = %v", c.TaskDuration)
	}
	if c.TotalDuration < c.TaskDuration {
		t.Fatalf("total %v < task %v", c.TotalDuration, c.TaskDuration)
	}
}
