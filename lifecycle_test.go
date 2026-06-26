package evals_test

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strings"
	"sync/atomic"
	"testing"

	evals "github.com/Kludex/pydantic-evals-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// ltEchoTask returns its input unchanged.
func ltEchoTask(_ context.Context, in string) (string, error) { return in, nil }

// ltFailingTask always returns an error.
func ltFailingTask(_ context.Context, _ string) (string, error) {
	return "", errors.New("task boom")
}

// ltFuncEvaluator adapts a function into an Evaluator with a custom report name.
type ltFuncEvaluator struct {
	name string
	fn   func(ctx context.Context, ec *evals.EvaluatorContext[string, string, any]) (evals.Output, error)
}

func (f ltFuncEvaluator) Evaluate(ctx context.Context, ec *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	return f.fn(ctx, ec)
}

func (f ltFuncEvaluator) EvaluationName() string { return f.name }

// ltConstEvaluator yields a fixed Output and report name.
type ltConstEvaluator struct {
	name string
	out  evals.Output
}

func (c ltConstEvaluator) Evaluate(_ context.Context, _ *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	return c.out, nil
}

func (c ltConstEvaluator) EvaluationName() string { return c.name }

func ltConstDataset(t *testing.T, name string, out evals.Output, evalName string) *evals.Dataset[string, string, any] {
	t.Helper()
	s := evals.For[string, string, any]()
	ds, err := evals.NewDataset[string, string, any](
		name,
		[]evals.Case[string, string, any]{s.Case("hello").Name("c1").Build()},
		ltConstEvaluator{name: evalName, out: out},
	)
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}
	return ds
}

// ltRecordingLifecycle captures lifecycle invocations and can be configured to
// fail at each stage.
type ltRecordingLifecycle struct {
	evals.BaseLifecycle[string, string, any]

	setupErr    error
	prepareErr  error
	teardownErr error

	prepareMetric    bool
	prepareAttribute bool

	gotResult   *evals.ReportCase[string, string, any]
	gotFailure  *evals.ReportCaseFailure[string, string, any]
	teardownRan bool
}

func (l *ltRecordingLifecycle) Setup(_ context.Context) error { return l.setupErr }

func (l *ltRecordingLifecycle) PrepareContext(_ context.Context, ec *evals.EvaluatorContext[string, string, any]) (*evals.EvaluatorContext[string, string, any], error) {
	if l.prepareErr != nil {
		return nil, l.prepareErr
	}
	if l.prepareMetric {
		ec.Metrics["prepared"] = 7
	}
	if l.prepareAttribute {
		ec.Attributes["prepared_attr"] = "yes"
	}
	return ec, nil
}

func (l *ltRecordingLifecycle) Teardown(_ context.Context, result *evals.ReportCase[string, string, any], failure *evals.ReportCaseFailure[string, string, any]) error {
	l.teardownRan = true
	l.gotResult = result
	l.gotFailure = failure
	return l.teardownErr
}

func TestLifecycleBaseNoOpSucceeds(t *testing.T) {
	s := evals.For[string, string, any]()
	ds := s.Dataset("base", s.Case("hello").Name("c1").Expect("hello")).With(s.EqualsExpected())

	report, err := ds.EvaluateWithLifecycle(context.Background(), ltEchoTask,
		func(c evals.Case[string, string, any]) evals.Lifecycle[string, string, any] {
			return evals.BaseLifecycle[string, string, any]{}
		})
	if err != nil {
		t.Fatalf("EvaluateWithLifecycle: %v", err)
	}
	if len(report.Failures) != 0 {
		t.Fatalf("expected no failures, got %+v", report.Failures)
	}
	if len(report.Cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(report.Cases))
	}
	got := report.Cases[0].Assertions["EqualsExpected"]
	if v, ok := got.Value.(evals.Bool); !ok || !bool(v) {
		t.Fatalf("expected passing EqualsExpected assertion, got %+v", got)
	}
}

func TestLifecycleSetupError(t *testing.T) {
	ds := ltConstDataset(t, "setup", evals.Assertion(true), "Always")

	report, err := ds.EvaluateWithLifecycle(context.Background(), ltEchoTask,
		func(c evals.Case[string, string, any]) evals.Lifecycle[string, string, any] {
			return &ltRecordingLifecycle{setupErr: errors.New("no setup")}
		})
	if err != nil {
		t.Fatalf("EvaluateWithLifecycle: %v", err)
	}
	if len(report.Cases) != 0 {
		t.Fatalf("expected no successful cases, got %d", len(report.Cases))
	}
	if len(report.Failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(report.Failures))
	}
	if got := report.Failures[0].ErrorMessage; got != "setup: no setup" {
		t.Fatalf("ErrorMessage = %q, want %q", got, "setup: no setup")
	}
	if report.Failures[0].Name != "c1" {
		t.Fatalf("failure name = %q, want %q", report.Failures[0].Name, "c1")
	}
}

func TestLifecyclePrepareContextError(t *testing.T) {
	ds := ltConstDataset(t, "prep", evals.Assertion(true), "Always")

	report, err := ds.EvaluateWithLifecycle(context.Background(), ltEchoTask,
		func(c evals.Case[string, string, any]) evals.Lifecycle[string, string, any] {
			return &ltRecordingLifecycle{prepareErr: errors.New("no prep")}
		})
	if err != nil {
		t.Fatalf("EvaluateWithLifecycle: %v", err)
	}
	if len(report.Cases) != 0 {
		t.Fatalf("expected no successful cases, got %d", len(report.Cases))
	}
	if len(report.Failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(report.Failures))
	}
	if got := report.Failures[0].ErrorMessage; got != "prepare context: no prep" {
		t.Fatalf("ErrorMessage = %q, want %q", got, "prepare context: no prep")
	}
}

func TestLifecyclePrepareContextEnrichesContext(t *testing.T) {
	// The evaluator observes the enriched metric/attribute, and they also land
	// on the ReportCase.
	seenMetric := -1.0
	seenAttr := ""
	ev := ltFuncEvaluator{
		name: "Inspector",
		fn: func(_ context.Context, ec *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
			seenMetric = ec.Metrics["prepared"]
			if v, ok := ec.Attributes["prepared_attr"].(string); ok {
				seenAttr = v
			}
			return evals.Assertion(true), nil
		},
	}
	s := evals.For[string, string, any]()
	ds, err := evals.NewDataset[string, string, any](
		"enrich",
		[]evals.Case[string, string, any]{s.Case("hello").Name("c1").Build()},
		ev,
	)
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}

	report, err := ds.EvaluateWithLifecycle(context.Background(), ltEchoTask,
		func(c evals.Case[string, string, any]) evals.Lifecycle[string, string, any] {
			return &ltRecordingLifecycle{prepareMetric: true, prepareAttribute: true}
		})
	if err != nil {
		t.Fatalf("EvaluateWithLifecycle: %v", err)
	}
	if seenMetric != 7 {
		t.Fatalf("evaluator saw metric %v, want 7", seenMetric)
	}
	if seenAttr != "yes" {
		t.Fatalf("evaluator saw attribute %q, want %q", seenAttr, "yes")
	}
	if len(report.Cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(report.Cases))
	}
	rc := report.Cases[0]
	if rc.Metrics["prepared"] != 7 {
		t.Fatalf("ReportCase metric prepared = %v, want 7", rc.Metrics["prepared"])
	}
	if v, _ := rc.Attributes["prepared_attr"].(string); v != "yes" {
		t.Fatalf("ReportCase attribute prepared_attr = %q, want %q", v, "yes")
	}
}

func TestLifecycleTeardownErrorOnSuccess(t *testing.T) {
	ds := ltConstDataset(t, "teardown", evals.Assertion(true), "Always")

	lc := &ltRecordingLifecycle{teardownErr: errors.New("cleanup failed")}
	report, err := ds.EvaluateWithLifecycle(context.Background(), ltEchoTask,
		func(c evals.Case[string, string, any]) evals.Lifecycle[string, string, any] {
			return lc
		})
	if err != nil {
		t.Fatalf("EvaluateWithLifecycle: %v", err)
	}
	if len(report.Cases) != 0 {
		t.Fatalf("expected the success to be discarded, got %d cases", len(report.Cases))
	}
	if len(report.Failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(report.Failures))
	}
	if got := report.Failures[0].ErrorMessage; got != "teardown: cleanup failed" {
		t.Fatalf("ErrorMessage = %q, want %q", got, "teardown: cleanup failed")
	}
	// Teardown observed the otherwise-successful result before the error.
	if lc.gotResult == nil {
		t.Fatalf("teardown should have observed a non-nil result")
	}
	if lc.gotFailure != nil {
		t.Fatalf("teardown should have observed a nil failure, got %+v", lc.gotFailure)
	}
}

func TestLifecycleTeardownErrorAfterTaskFailure(t *testing.T) {
	ds := ltConstDataset(t, "teardown-fail", evals.Assertion(true), "Always")

	lc := &ltRecordingLifecycle{teardownErr: errors.New("cleanup failed")}
	report, err := ds.EvaluateWithLifecycle(context.Background(), ltFailingTask,
		func(c evals.Case[string, string, any]) evals.Lifecycle[string, string, any] {
			return lc
		})
	if err != nil {
		t.Fatalf("EvaluateWithLifecycle: %v", err)
	}
	if len(report.Failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(report.Failures))
	}
	// The teardown error replaces the original task failure.
	if got := report.Failures[0].ErrorMessage; got != "teardown: cleanup failed" {
		t.Fatalf("ErrorMessage = %q, want %q", got, "teardown: cleanup failed")
	}
	// Teardown observed the task failure (result nil, failure non-nil).
	if lc.gotResult != nil {
		t.Fatalf("teardown should have observed a nil result, got %+v", lc.gotResult)
	}
	if lc.gotFailure == nil {
		t.Fatalf("teardown should have observed a non-nil failure")
	}
	if lc.gotFailure.ErrorMessage != "task boom" {
		t.Fatalf("teardown failure message = %q, want %q", lc.gotFailure.ErrorMessage, "task boom")
	}
}

func TestLifecycleTeardownObservesSuccess(t *testing.T) {
	ds := ltConstDataset(t, "observe-success", evals.Score(0.5), "Scorer")

	lc := &ltRecordingLifecycle{}
	report, err := ds.EvaluateWithLifecycle(context.Background(), ltEchoTask,
		func(c evals.Case[string, string, any]) evals.Lifecycle[string, string, any] {
			return lc
		})
	if err != nil {
		t.Fatalf("EvaluateWithLifecycle: %v", err)
	}
	if len(report.Cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(report.Cases))
	}
	if !lc.teardownRan {
		t.Fatalf("teardown did not run")
	}
	if lc.gotResult == nil {
		t.Fatalf("teardown should observe a non-nil result")
	}
	if lc.gotFailure != nil {
		t.Fatalf("teardown should observe a nil failure, got %+v", lc.gotFailure)
	}
	if lc.gotResult.Name != "c1" {
		t.Fatalf("observed result name = %q, want %q", lc.gotResult.Name, "c1")
	}
}

func TestLifecycleTeardownObservesFailure(t *testing.T) {
	ds := ltConstDataset(t, "observe-failure", evals.Assertion(true), "Always")

	lc := &ltRecordingLifecycle{}
	report, err := ds.EvaluateWithLifecycle(context.Background(), ltFailingTask,
		func(c evals.Case[string, string, any]) evals.Lifecycle[string, string, any] {
			return lc
		})
	if err != nil {
		t.Fatalf("EvaluateWithLifecycle: %v", err)
	}
	if len(report.Failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(report.Failures))
	}
	if !lc.teardownRan {
		t.Fatalf("teardown did not run")
	}
	if lc.gotResult != nil {
		t.Fatalf("teardown should observe a nil result, got %+v", lc.gotResult)
	}
	if lc.gotFailure == nil {
		t.Fatalf("teardown should observe a non-nil failure")
	}
	if lc.gotFailure.ErrorMessage != "task boom" {
		t.Fatalf("observed failure message = %q, want %q", lc.gotFailure.ErrorMessage, "task boom")
	}
}

func TestLifecycleFreshInstancePerCase(t *testing.T) {
	// A counter incremented in the factory proves each case gets a new instance.
	s := evals.For[string, string, any]()
	ds := s.Dataset("fresh",
		s.Case("a").Name("c1"),
		s.Case("b").Name("c2"),
		s.Case("c").Name("c3"),
	)

	var created atomic.Int64
	_, err := ds.EvaluateWithLifecycle(context.Background(), ltEchoTask,
		func(c evals.Case[string, string, any]) evals.Lifecycle[string, string, any] {
			created.Add(1)
			return &ltRecordingLifecycle{}
		})
	if err != nil {
		t.Fatalf("EvaluateWithLifecycle: %v", err)
	}
	if created.Load() != 3 {
		t.Fatalf("factory called %d times, want 3 (one per case)", created.Load())
	}
}

func TestLifecycleFreshInstancePerRepeat(t *testing.T) {
	s := evals.For[string, string, any]()
	ds := s.Dataset("fresh-repeat", s.Case("a").Name("c1"))

	var created atomic.Int64
	_, err := ds.EvaluateWithLifecycle(context.Background(), ltEchoTask,
		func(c evals.Case[string, string, any]) evals.Lifecycle[string, string, any] {
			created.Add(1)
			return &ltRecordingLifecycle{}
		},
		evals.Config{Repeat: 4})
	if err != nil {
		t.Fatalf("EvaluateWithLifecycle: %v", err)
	}
	if created.Load() != 4 {
		t.Fatalf("factory called %d times, want 4 (one per repeat)", created.Load())
	}
}

func TestScalarStringAllKinds(t *testing.T) {
	tests := []struct {
		name string
		in   evals.Scalar
		want string
	}{
		{"bool true", evals.Bool(true), "True"},
		{"bool false", evals.Bool(false), "False"},
		{"int", evals.Int(42), "42"},
		{"int negative", evals.Int(-3), "-3"},
		{"float", evals.Float(1.5), "1.5"},
		{"float whole", evals.Float(2), "2"},
		{"label", evals.Label("neutral"), "neutral"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.in.String(); got != tc.want {
				t.Fatalf("String() = %q, want %q", got, tc.want)
			}
		})
	}
}

// ltEvalForOutput runs a single-case dataset whose only evaluator returns out
// and returns the resulting ReportCase.
func ltEvalForOutput(t *testing.T, out evals.Output, evalName string) evals.ReportCase[string, string, any] {
	t.Helper()
	ds := ltConstDataset(t, "out", out, evalName)
	report, err := ds.Evaluate(context.Background(), ltEchoTask)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(report.Failures) != 0 {
		t.Fatalf("unexpected failures: %+v", report.Failures)
	}
	if len(report.Cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(report.Cases))
	}
	return report.Cases[0]
}

func TestOutputScore(t *testing.T) {
	rc := ltEvalForOutput(t, evals.Score(0.75).WithReason("close enough"), "Scorer")
	r, ok := rc.Scores["Scorer"]
	if !ok {
		t.Fatalf("Scorer score missing; scores=%+v", rc.Scores)
	}
	if v, ok := r.Value.(evals.Float); !ok || float64(v) != 0.75 {
		t.Fatalf("score value = %+v, want Float(0.75)", r.Value)
	}
	if r.Reason != "close enough" {
		t.Fatalf("reason = %q, want %q", r.Reason, "close enough")
	}
}

func TestOutputScoreInt(t *testing.T) {
	rc := ltEvalForOutput(t, evals.ScoreInt(3), "IntScorer")
	r, ok := rc.Scores["IntScorer"]
	if !ok {
		t.Fatalf("IntScorer score missing; scores=%+v", rc.Scores)
	}
	if v, ok := r.Value.(evals.Int); !ok || int(v) != 3 {
		t.Fatalf("score value = %+v, want Int(3)", r.Value)
	}
	if r.Value.String() != "3" {
		t.Fatalf("rendered int = %q, want %q", r.Value.String(), "3")
	}
}

func TestOutputAssertion(t *testing.T) {
	rc := ltEvalForOutput(t, evals.Assertion(false).WithReason("did not match"), "Checker")
	r, ok := rc.Assertions["Checker"]
	if !ok {
		t.Fatalf("Checker assertion missing; assertions=%+v", rc.Assertions)
	}
	if v, ok := r.Value.(evals.Bool); !ok || bool(v) {
		t.Fatalf("assertion value = %+v, want Bool(false)", r.Value)
	}
	if r.Reason != "did not match" {
		t.Fatalf("reason = %q, want %q", r.Reason, "did not match")
	}
}

func TestOutputCategory(t *testing.T) {
	rc := ltEvalForOutput(t, evals.Category("positive"), "Classifier")
	r, ok := rc.Labels["Classifier"]
	if !ok {
		t.Fatalf("Classifier label missing; labels=%+v", rc.Labels)
	}
	if v, ok := r.Value.(evals.Label); !ok || string(v) != "positive" {
		t.Fatalf("label value = %+v, want Label(positive)", r.Value)
	}
}

func TestOutputNamed(t *testing.T) {
	out := evals.Named(
		"length", evals.Score(0.8).WithReason("close"),
		"sentiment", evals.Category("neutral"),
		"ok", evals.Assertion(true),
	)
	rc := ltEvalForOutput(t, out, "Multi")

	score, ok := rc.Scores["length"]
	if !ok || float64(score.Value.(evals.Float)) != 0.8 {
		t.Fatalf("length score = %+v, want Float(0.8)", score)
	}
	if score.Reason != "close" {
		t.Fatalf("length reason = %q, want %q", score.Reason, "close")
	}
	label, ok := rc.Labels["sentiment"]
	if !ok || string(label.Value.(evals.Label)) != "neutral" {
		t.Fatalf("sentiment label = %+v, want Label(neutral)", label)
	}
	assertion, ok := rc.Assertions["ok"]
	if !ok || !bool(assertion.Value.(evals.Bool)) {
		t.Fatalf("ok assertion = %+v, want Bool(true)", assertion)
	}
}

func TestOutputNamedPanicsOddArgs(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on odd argument count")
		}
	}()
	evals.Named("only-name")
}

func TestOutputNamedPanicsNonStringName(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on non-string name")
		}
	}()
	evals.Named(123, evals.Score(1))
}

func TestOutputNamedPanicsNonSingleValue(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Fatalf("expected panic on non-single value")
		}
	}()
	evals.Named("nested", evals.Named())
}

func TestOutputNoResult(t *testing.T) {
	rc := ltEvalForOutput(t, evals.NoResult(), "Empty")
	if len(rc.Scores) != 0 || len(rc.Labels) != 0 || len(rc.Assertions) != 0 {
		t.Fatalf("expected no results, got scores=%+v labels=%+v assertions=%+v", rc.Scores, rc.Labels, rc.Assertions)
	}
	if len(rc.EvaluatorFailures) != 0 {
		t.Fatalf("expected no evaluator failures, got %+v", rc.EvaluatorFailures)
	}
}

// --- OTel tracing tests ---

// ltInstallRecorder installs a recording tracer provider and restores the
// previous one on cleanup, returning the recorder.
func ltInstallRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	prev := otel.GetTracerProvider()
	sr := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr)))
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return sr
}

// ltSpanByName returns the first ended span with the given name.
func ltSpanByName(t *testing.T, spans []sdktrace.ReadOnlySpan, name string) sdktrace.ReadOnlySpan {
	t.Helper()
	for _, s := range spans {
		if s.Name() == name {
			return s
		}
	}
	t.Fatalf("span %q not found; have %v", name, ltSpanNames(spans))
	return nil
}

func ltSpanNames(spans []sdktrace.ReadOnlySpan) []string {
	names := make([]string, len(spans))
	for i, s := range spans {
		names[i] = s.Name()
	}
	return names
}

func ltAttr(t *testing.T, s sdktrace.ReadOnlySpan, key string) attribute.Value {
	t.Helper()
	for _, kv := range s.Attributes() {
		if string(kv.Key) == key {
			return kv.Value
		}
	}
	t.Fatalf("attribute %q not found on span %q", key, s.Name())
	return attribute.Value{}
}

func ltHasAttr(s sdktrace.ReadOnlySpan, key string) bool {
	for _, kv := range s.Attributes() {
		if string(kv.Key) == key {
			return true
		}
	}
	return false
}

func ltHasExceptionEvent(s sdktrace.ReadOnlySpan) bool {
	for _, e := range s.Events() {
		if e.Name == "exception" {
			return true
		}
	}
	return false
}

func TestTracingSpanTree(t *testing.T) {
	sr := ltInstallRecorder(t)

	s := evals.For[string, string, any]()
	ds := s.Dataset("greetings", s.Case("hello").Name("c1").Expect("hello")).With(s.EqualsExpected())

	_, err := ds.Evaluate(context.Background(), ltEchoTask, evals.Config{
		Name:     "exp",
		TaskName: "greeter",
		Metadata: map[string]any{"team": "qa"},
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	spans := sr.Ended()

	root := ltSpanByName(t, spans, "evaluate exp")
	if got := ltAttr(t, root, "gen_ai.operation.name").AsString(); got != "experiment" {
		t.Fatalf("gen_ai.operation.name = %q, want %q", got, "experiment")
	}
	if got := ltAttr(t, root, "dataset_name").AsString(); got != "greetings" {
		t.Fatalf("dataset_name = %q, want %q", got, "greetings")
	}
	if got := ltAttr(t, root, "n_cases").AsInt64(); got != 1 {
		t.Fatalf("n_cases = %d, want 1", got)
	}
	if got := ltAttr(t, root, "assertion_pass_rate").AsFloat64(); got != 1 {
		t.Fatalf("assertion_pass_rate = %v, want 1", got)
	}
	var meta map[string]any
	if err := json.Unmarshal([]byte(ltAttr(t, root, "metadata").AsString()), &meta); err != nil {
		t.Fatalf("metadata not JSON: %v", err)
	}
	if meta["team"] != "qa" {
		t.Fatalf("metadata team = %v, want qa", meta["team"])
	}
	if root.InstrumentationScope().Name != "pydantic-evals" {
		t.Fatalf("scope = %q, want %q", root.InstrumentationScope().Name, "pydantic-evals")
	}

	caseSpan := ltSpanByName(t, spans, "case: c1")
	if !caseSpan.Parent().Equal(root.SpanContext()) {
		t.Fatalf("case span parent is not the experiment span")
	}
	var assertions map[string]any
	if err := json.Unmarshal([]byte(ltAttr(t, caseSpan, "assertions").AsString()), &assertions); err != nil {
		t.Fatalf("assertions not JSON: %v", err)
	}
	if assertions["EqualsExpected"] != true {
		t.Fatalf("assertions[EqualsExpected] = %v, want true", assertions["EqualsExpected"])
	}
	for _, key := range []string{"output", "scores", "labels", "metrics", "attributes"} {
		if !ltHasAttr(caseSpan, key) {
			t.Fatalf("case span missing attribute %q", key)
		}
	}
	if got := ltAttr(t, caseSpan, "output").AsString(); got != `"hello"` {
		t.Fatalf("output = %q, want %q", got, `"hello"`)
	}

	taskSpan := ltSpanByName(t, spans, "execute greeter")
	if !taskSpan.Parent().Equal(caseSpan.SpanContext()) {
		t.Fatalf("task span parent is not the case span")
	}
	if got := ltAttr(t, taskSpan, "task").AsString(); got != "greeter" {
		t.Fatalf("task = %q, want greeter", got)
	}

	evalSpan := ltSpanByName(t, spans, "evaluator: EqualsExpected")
	if !evalSpan.Parent().Equal(caseSpan.SpanContext()) {
		t.Fatalf("evaluator span parent is not the case span")
	}
	if got := ltAttr(t, evalSpan, "evaluator_name").AsString(); got != "EqualsExpected" {
		t.Fatalf("evaluator_name = %q, want EqualsExpected", got)
	}
}

func TestTracingScalarFlow(t *testing.T) {
	sr := ltInstallRecorder(t)

	out := evals.Named(
		"int_score", evals.ScoreInt(5),
		"float_score", evals.Score(0.25),
		"flag", evals.Assertion(true),
		"category", evals.Category("good"),
	)
	ds := ltConstDataset(t, "scalars", out, "Multi")
	if _, err := ds.Evaluate(context.Background(), ltEchoTask); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	caseSpan := ltSpanByName(t, sr.Ended(), "case: c1")

	var scores map[string]any
	if err := json.Unmarshal([]byte(ltAttr(t, caseSpan, "scores").AsString()), &scores); err != nil {
		t.Fatalf("scores not JSON: %v", err)
	}
	if scores["int_score"] != float64(5) {
		t.Fatalf("scores[int_score] = %v, want 5", scores["int_score"])
	}
	if scores["float_score"] != 0.25 {
		t.Fatalf("scores[float_score] = %v, want 0.25", scores["float_score"])
	}

	var labels map[string]any
	if err := json.Unmarshal([]byte(ltAttr(t, caseSpan, "labels").AsString()), &labels); err != nil {
		t.Fatalf("labels not JSON: %v", err)
	}
	if labels["category"] != "good" {
		t.Fatalf("labels[category] = %v, want good", labels["category"])
	}

	var assertions map[string]any
	if err := json.Unmarshal([]byte(ltAttr(t, caseSpan, "assertions").AsString()), &assertions); err != nil {
		t.Fatalf("assertions not JSON: %v", err)
	}
	if assertions["flag"] != true {
		t.Fatalf("assertions[flag] = %v, want true", assertions["flag"])
	}
}

func TestTracingSourceCaseNameOnRepeat(t *testing.T) {
	sr := ltInstallRecorder(t)

	s := evals.For[string, string, any]()
	ds := s.Dataset("repeat", s.Case("hi").Name("c1"))
	if _, err := ds.Evaluate(context.Background(), ltEchoTask, evals.Config{Repeat: 2}); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	spans := sr.Ended()
	first := ltSpanByName(t, spans, "case: c1 [1/2]")
	if got := ltAttr(t, first, "logfire.experiment.source_case_name").AsString(); got != "c1" {
		t.Fatalf("source_case_name = %q, want c1", got)
	}
	second := ltSpanByName(t, spans, "case: c1 [2/2]")
	if got := ltAttr(t, second, "logfire.experiment.source_case_name").AsString(); got != "c1" {
		t.Fatalf("source_case_name = %q, want c1", got)
	}

	root := ltSpanByName(t, spans, "evaluate task")
	if got := ltAttr(t, root, "logfire.experiment.repeat").AsInt64(); got != 2 {
		t.Fatalf("repeat attr = %d, want 2", got)
	}
}

func TestTracingTaskFailureStatus(t *testing.T) {
	sr := ltInstallRecorder(t)

	ds := ltConstDataset(t, "task-fail", evals.Assertion(true), "Always")
	if _, err := ds.Evaluate(context.Background(), ltFailingTask); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	spans := sr.Ended()

	caseSpan := ltSpanByName(t, spans, "case: c1")
	if caseSpan.Status().Code != codes.Error {
		t.Fatalf("case span status = %v, want Error", caseSpan.Status().Code)
	}
	if !ltHasExceptionEvent(caseSpan) {
		t.Fatalf("case span missing exception event")
	}

	taskSpan := ltSpanByName(t, spans, "execute task")
	if taskSpan.Status().Code != codes.Error {
		t.Fatalf("task span status = %v, want Error", taskSpan.Status().Code)
	}
	if !ltHasExceptionEvent(taskSpan) {
		t.Fatalf("task span missing exception event")
	}
	// The failing case never produced an evaluation result, so no
	// assertion_pass_rate lands on the experiment span.
	root := ltSpanByName(t, spans, "evaluate task")
	if ltHasAttr(root, "assertion_pass_rate") {
		t.Fatalf("experiment span should have no assertion_pass_rate when all cases fail")
	}
}

func TestTracingEvaluatorErrorStatus(t *testing.T) {
	sr := ltInstallRecorder(t)

	ev := ltFuncEvaluator{
		name: "Boom",
		fn: func(_ context.Context, _ *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
			return nil, errors.New("evaluator exploded")
		},
	}
	s := evals.For[string, string, any]()
	ds, err := evals.NewDataset[string, string, any](
		"eval-fail",
		[]evals.Case[string, string, any]{s.Case("hi").Name("c1").Build()},
		ev,
	)
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}
	report, err := ds.Evaluate(context.Background(), ltEchoTask)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(report.Cases) != 1 {
		t.Fatalf("expected the case to still succeed, got %d cases", len(report.Cases))
	}
	if len(report.Cases[0].EvaluatorFailures) != 1 {
		t.Fatalf("expected 1 evaluator failure, got %+v", report.Cases[0].EvaluatorFailures)
	}

	evalSpan := ltSpanByName(t, sr.Ended(), "evaluator: Boom")
	if evalSpan.Status().Code != codes.Error {
		t.Fatalf("evaluator span status = %v, want Error", evalSpan.Status().Code)
	}
	if !ltHasExceptionEvent(evalSpan) {
		t.Fatalf("evaluator span missing exception event")
	}
}

func TestTracingUnmarshalableAttribute(t *testing.T) {
	sr := ltInstallRecorder(t)

	task := func(ctx context.Context, in string) (string, error) {
		evals.SetAttribute(ctx, "unserializable", make(chan int))
		return in, nil
	}
	ds := ltConstDataset(t, "weird-attr", evals.Assertion(true), "Always")
	report, err := ds.Evaluate(context.Background(), task)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(report.Cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(report.Cases))
	}

	// The attributes JSON falls back to fmt rendering rather than crashing.
	caseSpan := ltSpanByName(t, sr.Ended(), "case: c1")
	got := ltAttr(t, caseSpan, "attributes").AsString()
	if !strings.Contains(got, "unserializable") {
		t.Fatalf("attributes attr = %q, want it to mention the key", got)
	}
}

func TestTracingNoProviderNoOp(t *testing.T) {
	// Without installing a recording provider, the global default tracer is used
	// and Evaluate still works end-to-end.
	ds := ltConstDataset(t, "noop", evals.Assertion(true), "Always")
	report, err := ds.Evaluate(context.Background(), ltEchoTask)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(report.Cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(report.Cases))
	}
}

func TestTracingCaseMetadataAttribute(t *testing.T) {
	sr := ltInstallRecorder(t)

	s := evals.For[string, string, map[string]any]()
	ds, err := evals.NewDataset[string, string, map[string]any](
		"with-meta",
		[]evals.Case[string, string, map[string]any]{
			s.Case("hi").Name("c1").Meta(map[string]any{"lang": "en"}).Build(),
		},
		ltMetaEvaluator{},
	)
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}
	if _, err := ds.Evaluate(context.Background(), ltEchoTask); err != nil {
		t.Fatalf("Evaluate: %v", err)
	}

	caseSpan := ltSpanByName(t, sr.Ended(), "case: c1")
	var meta map[string]any
	if err := json.Unmarshal([]byte(ltAttr(t, caseSpan, "metadata").AsString()), &meta); err != nil {
		t.Fatalf("metadata not JSON: %v", err)
	}
	if meta["lang"] != "en" {
		t.Fatalf("case metadata lang = %v, want en", meta["lang"])
	}
}

// ltMetaEvaluator is a trivial passing evaluator over map metadata, used to
// exercise the case-span metadata attribute branch.
type ltMetaEvaluator struct{}

func (ltMetaEvaluator) Evaluate(_ context.Context, _ *evals.EvaluatorContext[string, string, map[string]any]) (evals.Output, error) {
	return evals.Assertion(true), nil
}

func TestMetricsFlowToReportAndSpan(t *testing.T) {
	sr := ltInstallRecorder(t)

	task := func(ctx context.Context, in string) (string, error) {
		evals.IncrementMetric(ctx, "tokens", 10)
		evals.IncrementMetric(ctx, "tokens", 5)
		return in, nil
	}
	ds := ltConstDataset(t, "metrics", evals.Assertion(true), "Always")
	report, err := ds.Evaluate(context.Background(), task)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(report.Cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(report.Cases))
	}
	if got := report.Cases[0].Metrics["tokens"]; got != 15 {
		t.Fatalf("ReportCase metric tokens = %v, want 15", got)
	}

	caseSpan := ltSpanByName(t, sr.Ended(), "case: c1")
	var metrics map[string]any
	if err := json.Unmarshal([]byte(ltAttr(t, caseSpan, "metrics").AsString()), &metrics); err != nil {
		t.Fatalf("metrics not JSON: %v", err)
	}
	if metrics["tokens"] != float64(15) {
		t.Fatalf("span metric tokens = %v, want 15", metrics["tokens"])
	}
}

func TestMetricsAndAttributesNoOpOutsideTask(t *testing.T) {
	// Outside a task-run context these are silent no-ops, not panics.
	evals.IncrementMetric(context.Background(), "ignored", 1)
	evals.SetAttribute(context.Background(), "ignored", "value")
}

func TestMetricsZeroIncrementNotRecorded(t *testing.T) {
	// A net-zero increment on a previously-unseen metric records nothing, while a
	// real increment to the same metric is kept.
	task := func(ctx context.Context, in string) (string, error) {
		evals.IncrementMetric(ctx, "noop_metric", 0)
		evals.IncrementMetric(ctx, "real_metric", 2)
		return in, nil
	}
	ds := ltConstDataset(t, "zero-metric", evals.Assertion(true), "Always")
	report, err := ds.Evaluate(context.Background(), task)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	metrics := report.Cases[0].Metrics
	if _, ok := metrics["noop_metric"]; ok {
		t.Fatalf("net-zero increment should not be recorded, got %+v", metrics)
	}
	if metrics["real_metric"] != 2 {
		t.Fatalf("real_metric = %v, want 2", metrics["real_metric"])
	}
}

func TestOutputNamedRejectsNonFinite(t *testing.T) {
	out := evals.Named(
		"ok", evals.Score(0.5),
		"bad", evals.Score(math.NaN()),
	)
	rc := ltEvalForOutput(t, out, "Multi")
	if len(rc.Scores) != 0 {
		t.Fatalf("expected the whole named output rejected, got scores %+v", rc.Scores)
	}
	if len(rc.EvaluatorFailures) != 1 {
		t.Fatalf("expected 1 evaluator failure, got %+v", rc.EvaluatorFailures)
	}
	if !strings.Contains(rc.EvaluatorFailures[0].ErrorMessage, "non-finite") {
		t.Fatalf("failure message = %q, want it to mention non-finite", rc.EvaluatorFailures[0].ErrorMessage)
	}
}

func TestOutputScoreRejectsNonFinite(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value float64
	}{
		{"positive inf", math.Inf(1)},
		{"negative inf", math.Inf(-1)},
		{"nan", math.NaN()},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rc := ltEvalForOutput(t, evals.Score(tc.value), "Scorer")
			if len(rc.Scores) != 0 {
				t.Fatalf("expected no score recorded, got %+v", rc.Scores)
			}
			if len(rc.EvaluatorFailures) != 1 {
				t.Fatalf("expected 1 evaluator failure, got %+v", rc.EvaluatorFailures)
			}
			if !strings.Contains(rc.EvaluatorFailures[0].ErrorMessage, "non-finite") {
				t.Fatalf("failure message = %q, want it to mention non-finite", rc.EvaluatorFailures[0].ErrorMessage)
			}
		})
	}
}
