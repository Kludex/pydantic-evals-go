package evals

import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
)

// lcRecordingEvaluator is a minimal public-API Evaluator that reports whether the
// output equals "ok" as a boolean assertion. It lets lifecycle tests observe the
// EvaluatorContext that PrepareContext produced.
type lcRecordingEvaluator struct {
	seenMetrics    map[string]float64
	seenAttributes map[string]any
}

func (e *lcRecordingEvaluator) Evaluate(_ context.Context, ec *EvaluatorContext[string, string, string]) (EvaluatorOutput, error) {
	e.seenMetrics = ec.Metrics
	e.seenAttributes = ec.Attributes
	return Reason(Bool(ec.Output == "ok"), "checked output"), nil
}

func (e *lcRecordingEvaluator) Spec() EvaluatorSpec { return NewSpec("recording") }

func lcOKTask(_ context.Context, in string) (string, error) { return in, nil }

func lcFailingTask(_ context.Context, _ string) (string, error) {
	return "", errors.New("boom")
}

// lcSpy embeds BaseLifecycle and records which hooks fired and what Teardown
// observed, while optionally injecting errors or context enrichment.
type lcSpy struct {
	BaseLifecycle[string, string, string]

	setupErr    error
	prepareErr  error
	teardownErr error
	enrich      bool

	setupCalled    bool
	prepareCalled  bool
	teardownCalled bool

	teardownResult  *ReportCase[string, string, string]
	teardownFailure *ReportCaseFailure[string, string, string]
}

func (l *lcSpy) Setup(ctx context.Context) error {
	l.setupCalled = true
	return l.setupErr
}

func (l *lcSpy) PrepareContext(ctx context.Context, ec *EvaluatorContext[string, string, string]) (*EvaluatorContext[string, string, string], error) {
	l.prepareCalled = true
	if l.prepareErr != nil {
		return nil, l.prepareErr
	}
	if l.enrich {
		ec.Metrics["prepared_metric"] = 42
		ec.Attributes["prepared_attr"] = "from_prepare"
	}
	return ec, nil
}

func (l *lcSpy) Teardown(ctx context.Context, result *ReportCase[string, string, string], failure *ReportCaseFailure[string, string, string]) error {
	l.teardownCalled = true
	l.teardownResult = result
	l.teardownFailure = failure
	return l.teardownErr
}

func lcSingleCaseDataset(t *testing.T, evaluators ...Evaluator[string, string, string]) *Dataset[string, string, string] {
	t.Helper()
	cases := []Case[string, string, string]{
		NewCase[string, string, string]("ok", WithCaseName[string, string, string]("solo")),
	}
	ds, err := NewDataset("ds", cases, evaluators...)
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}
	return ds
}

func TestBaseLifecycleNoOpDefaults(t *testing.T) {
	ds := lcSingleCaseDataset(t)

	var lc BaseLifecycle[string, string, string]
	if err := lc.Setup(context.Background()); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	ec := &EvaluatorContext[string, string, string]{Output: "ok"}
	got, err := lc.PrepareContext(context.Background(), ec)
	if err != nil {
		t.Fatalf("PrepareContext: %v", err)
	}
	if got != ec {
		t.Fatalf("PrepareContext should return the same context unchanged")
	}
	if err := lc.Teardown(context.Background(), nil, nil); err != nil {
		t.Fatalf("Teardown: %v", err)
	}

	report, err := ds.Evaluate(context.Background(), lcOKTask,
		WithLifecycle(func(c Case[string, string, string]) Lifecycle[string, string, string] {
			return BaseLifecycle[string, string, string]{}
		}),
	)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(report.Cases) != 1 || len(report.Failures) != 0 {
		t.Fatalf("expected 1 success and 0 failures, got %d/%d", len(report.Cases), len(report.Failures))
	}
	if report.Cases[0].Output != "ok" {
		t.Fatalf("unexpected output %q", report.Cases[0].Output)
	}
}

func TestLifecycleSetupError(t *testing.T) {
	ev := &lcRecordingEvaluator{}
	ds := lcSingleCaseDataset(t, ev)
	spy := &lcSpy{setupErr: errors.New("disk full")}

	report, err := ds.Evaluate(context.Background(), lcOKTask,
		WithLifecycle(func(c Case[string, string, string]) Lifecycle[string, string, string] { return spy }),
	)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(report.Cases) != 0 || len(report.Failures) != 1 {
		t.Fatalf("expected 0 success and 1 failure, got %d/%d", len(report.Cases), len(report.Failures))
	}
	if want := "setup: disk full"; report.Failures[0].ErrorMessage != want {
		t.Fatalf("ErrorMessage = %q, want %q", report.Failures[0].ErrorMessage, want)
	}
	if report.Failures[0].Name != "solo" {
		t.Fatalf("failure Name = %q, want %q", report.Failures[0].Name, "solo")
	}
	if !spy.setupCalled {
		t.Fatalf("Setup should have been called")
	}
	if spy.prepareCalled {
		t.Fatalf("PrepareContext should not run after Setup error")
	}
	if !spy.teardownCalled {
		t.Fatalf("Teardown should still run after Setup error")
	}
	if spy.teardownResult != nil {
		t.Fatalf("Teardown result should be nil after Setup error")
	}
	if spy.teardownFailure == nil || spy.teardownFailure.ErrorMessage != "setup: disk full" {
		t.Fatalf("Teardown should observe the setup failure, got %+v", spy.teardownFailure)
	}
}

func TestLifecyclePrepareContextError(t *testing.T) {
	ds := lcSingleCaseDataset(t)
	spy := &lcSpy{prepareErr: errors.New("no context")}

	report, err := ds.Evaluate(context.Background(), lcOKTask,
		WithLifecycle(func(c Case[string, string, string]) Lifecycle[string, string, string] { return spy }),
	)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(report.Cases) != 0 || len(report.Failures) != 1 {
		t.Fatalf("expected 0 success and 1 failure, got %d/%d", len(report.Cases), len(report.Failures))
	}
	if want := "prepare context: no context"; report.Failures[0].ErrorMessage != want {
		t.Fatalf("ErrorMessage = %q, want %q", report.Failures[0].ErrorMessage, want)
	}
	if !spy.setupCalled || !spy.prepareCalled || !spy.teardownCalled {
		t.Fatalf("Setup, PrepareContext and Teardown should all have run")
	}
	if spy.teardownFailure == nil || spy.teardownFailure.ErrorMessage != "prepare context: no context" {
		t.Fatalf("Teardown should observe the prepare-context failure, got %+v", spy.teardownFailure)
	}
}

func TestLifecyclePrepareContextEnriches(t *testing.T) {
	ev := &lcRecordingEvaluator{}
	ds := lcSingleCaseDataset(t, ev)
	spy := &lcSpy{enrich: true}

	report, err := ds.Evaluate(context.Background(), lcOKTask,
		WithLifecycle(func(c Case[string, string, string]) Lifecycle[string, string, string] { return spy }),
	)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(report.Cases) != 1 {
		t.Fatalf("expected 1 success, got %d (failures %d)", len(report.Cases), len(report.Failures))
	}
	rc := report.Cases[0]
	if got := rc.Metrics["prepared_metric"]; got != 42 {
		t.Fatalf("metric prepared_metric = %v, want 42", got)
	}
	if got := rc.Attributes["prepared_attr"]; got != "from_prepare" {
		t.Fatalf("attribute prepared_attr = %v, want from_prepare", got)
	}
	if ev.seenMetrics["prepared_metric"] != 42 {
		t.Fatalf("evaluator did not observe the enriched metric: %v", ev.seenMetrics)
	}
	if ev.seenAttributes["prepared_attr"] != "from_prepare" {
		t.Fatalf("evaluator did not observe the enriched attribute: %v", ev.seenAttributes)
	}
}

func TestLifecycleTeardownErrorDiscardsSuccess(t *testing.T) {
	ds := lcSingleCaseDataset(t)
	spy := &lcSpy{teardownErr: errors.New("cleanup failed")}

	report, err := ds.Evaluate(context.Background(), lcOKTask,
		WithLifecycle(func(c Case[string, string, string]) Lifecycle[string, string, string] { return spy }),
	)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(report.Cases) != 0 {
		t.Fatalf("successful result should be discarded on Teardown error, got %d cases", len(report.Cases))
	}
	if len(report.Failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(report.Failures))
	}
	if want := "teardown: cleanup failed"; report.Failures[0].ErrorMessage != want {
		t.Fatalf("ErrorMessage = %q, want %q", report.Failures[0].ErrorMessage, want)
	}
	if spy.teardownResult == nil {
		t.Fatalf("Teardown should have observed the successful result before discarding it")
	}
	if spy.teardownFailure != nil {
		t.Fatalf("Teardown should observe a nil failure when the case succeeded, got %+v", spy.teardownFailure)
	}
}

func TestLifecycleTeardownErrorAfterTaskFailure(t *testing.T) {
	ds := lcSingleCaseDataset(t)
	spy := &lcSpy{teardownErr: errors.New("cleanup failed")}

	report, err := ds.Evaluate(context.Background(), lcFailingTask,
		WithLifecycle(func(c Case[string, string, string]) Lifecycle[string, string, string] { return spy }),
	)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(report.Cases) != 0 || len(report.Failures) != 1 {
		t.Fatalf("expected 0 success and 1 failure, got %d/%d", len(report.Cases), len(report.Failures))
	}
	if want := "teardown: cleanup failed"; report.Failures[0].ErrorMessage != want {
		t.Fatalf("ErrorMessage = %q, want %q", report.Failures[0].ErrorMessage, want)
	}
	if spy.teardownResult != nil {
		t.Fatalf("Teardown result should be nil when the task failed, got %+v", spy.teardownResult)
	}
	if spy.teardownFailure == nil || spy.teardownFailure.ErrorMessage != "boom" {
		t.Fatalf("Teardown should observe the task failure, got %+v", spy.teardownFailure)
	}
}

func TestLifecycleTeardownObservesSuccess(t *testing.T) {
	ev := &lcRecordingEvaluator{}
	ds := lcSingleCaseDataset(t, ev)
	spy := &lcSpy{}

	report, err := ds.Evaluate(context.Background(), lcOKTask,
		WithLifecycle(func(c Case[string, string, string]) Lifecycle[string, string, string] { return spy }),
	)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(report.Cases) != 1 || len(report.Failures) != 0 {
		t.Fatalf("expected 1 success and 0 failures, got %d/%d", len(report.Cases), len(report.Failures))
	}
	if spy.teardownResult == nil {
		t.Fatalf("Teardown should observe a non-nil success result")
	}
	if spy.teardownResult.Name != "solo" || spy.teardownResult.Output != "ok" {
		t.Fatalf("unexpected success result: %+v", spy.teardownResult)
	}
	if spy.teardownFailure != nil {
		t.Fatalf("Teardown failure should be nil on success, got %+v", spy.teardownFailure)
	}
	res, ok := report.Cases[0].Assertions["recording"]
	if !ok {
		t.Fatalf("expected a 'recording' assertion, got %v", report.Cases[0].Assertions)
	}
	if res.Value != Bool(true) {
		t.Fatalf("assertion value = %v, want True", res.Value)
	}
	if res.Reason != "checked output" {
		t.Fatalf("assertion reason = %q, want %q", res.Reason, "checked output")
	}
}

func TestLifecycleTeardownObservesFailure(t *testing.T) {
	ds := lcSingleCaseDataset(t)
	spy := &lcSpy{}

	report, err := ds.Evaluate(context.Background(), lcFailingTask,
		WithLifecycle(func(c Case[string, string, string]) Lifecycle[string, string, string] { return spy }),
	)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(report.Cases) != 0 || len(report.Failures) != 1 {
		t.Fatalf("expected 0 success and 1 failure, got %d/%d", len(report.Cases), len(report.Failures))
	}
	if spy.teardownResult != nil {
		t.Fatalf("Teardown result should be nil on task failure, got %+v", spy.teardownResult)
	}
	if spy.teardownFailure == nil {
		t.Fatalf("Teardown should observe a non-nil failure")
	}
	if spy.teardownFailure.ErrorMessage != "boom" {
		t.Fatalf("Teardown failure message = %q, want %q", spy.teardownFailure.ErrorMessage, "boom")
	}
}

func TestLifecycleFreshInstancePerCase(t *testing.T) {
	cases := []Case[string, string, string]{
		NewCase[string, string, string]("ok", WithCaseName[string, string, string]("a")),
		NewCase[string, string, string]("ok", WithCaseName[string, string, string]("b")),
	}
	ds, err := NewDataset("ds", cases)
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}

	var factoryCalls int64
	report, err := ds.Evaluate(context.Background(), lcOKTask,
		WithLifecycle(func(c Case[string, string, string]) Lifecycle[string, string, string] {
			atomic.AddInt64(&factoryCalls, 1)
			return &lcSpy{}
		}),
	)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(report.Cases) != 2 {
		t.Fatalf("expected 2 successes, got %d", len(report.Cases))
	}
	if got := atomic.LoadInt64(&factoryCalls); got != 2 {
		t.Fatalf("factory should be called once per case (2), got %d", got)
	}
}

func TestLifecycleFreshInstancePerRepeat(t *testing.T) {
	ds := lcSingleCaseDataset(t)

	var factoryCalls int64
	instances := make(map[*lcSpy]bool)
	guard := make(chan struct{}, 1)
	guard <- struct{}{}

	report, err := ds.Evaluate(context.Background(), lcOKTask,
		WithRepeat[string, string, string](3),
		WithLifecycle(func(c Case[string, string, string]) Lifecycle[string, string, string] {
			atomic.AddInt64(&factoryCalls, 1)
			spy := &lcSpy{}
			<-guard
			instances[spy] = true
			guard <- struct{}{}
			return spy
		}),
	)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(report.Cases) != 3 {
		t.Fatalf("expected 3 repeated successes, got %d", len(report.Cases))
	}
	if got := atomic.LoadInt64(&factoryCalls); got != 3 {
		t.Fatalf("factory should be called once per repeat (3), got %d", got)
	}
	if len(instances) != 3 {
		t.Fatalf("expected 3 distinct lifecycle instances, got %d", len(instances))
	}
}

func TestLifecycleAbsentFactory(t *testing.T) {
	ds := lcSingleCaseDataset(t)
	report, err := ds.Evaluate(context.Background(), lcOKTask)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(report.Cases) != 1 || len(report.Failures) != 0 {
		t.Fatalf("expected 1 success and 0 failures, got %d/%d", len(report.Cases), len(report.Failures))
	}
}

func TestLifecycleScalarString(t *testing.T) {
	tests := []struct {
		name string
		val  Scalar
		want string
	}{
		{"bool true", Bool(true), "True"},
		{"bool false", Bool(false), "False"},
		{"int", Int(7), "7"},
		{"int negative", Int(-3), "-3"},
		{"float", Float(1.5), "1.5"},
		{"float integral", Float(2), "2"},
		{"label", Label("good"), "good"},
		{"label empty", Label(""), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.val.String(); got != tt.want {
				t.Fatalf("String() = %q, want %q", got, tt.want)
			}
		})
	}
}

// lcEmitter returns a configurable EvaluatorOutput so that ScalarValue and Reason
// are exercised end-to-end and the stored Scalar/Reason are asserted.
type lcEmitter struct {
	output EvaluatorOutput
	name   string
}

func (e lcEmitter) Evaluate(_ context.Context, _ *EvaluatorContext[string, string, string]) (EvaluatorOutput, error) {
	return e.output, nil
}

func (e lcEmitter) Spec() EvaluatorSpec { return NewSpec(e.name) }

func TestLifecycleScalarValueAndReasonStored(t *testing.T) {
	tests := []struct {
		name       string
		emitter    lcEmitter
		bucket     func(rc ReportCase[string, string, string]) map[string]EvaluationResult
		wantValue  Scalar
		wantReason string
	}{
		{
			name:      "scalar bool assertion",
			emitter:   lcEmitter{output: ScalarValue(Bool(true)), name: "passed"},
			bucket:    func(rc ReportCase[string, string, string]) map[string]EvaluationResult { return rc.Assertions },
			wantValue: Bool(true),
		},
		{
			name:      "scalar int score",
			emitter:   lcEmitter{output: ScalarValue(Int(9)), name: "score"},
			bucket:    func(rc ReportCase[string, string, string]) map[string]EvaluationResult { return rc.Scores },
			wantValue: Int(9),
		},
		{
			name:      "scalar float score",
			emitter:   lcEmitter{output: ScalarValue(Float(0.5)), name: "ratio"},
			bucket:    func(rc ReportCase[string, string, string]) map[string]EvaluationResult { return rc.Scores },
			wantValue: Float(0.5),
		},
		{
			name:      "scalar label",
			emitter:   lcEmitter{output: ScalarValue(Label("category")), name: "kind"},
			bucket:    func(rc ReportCase[string, string, string]) map[string]EvaluationResult { return rc.Labels },
			wantValue: Label("category"),
		},
		{
			name:       "reason bool assertion",
			emitter:    lcEmitter{output: Reason(Bool(false), "because"), name: "checked"},
			bucket:     func(rc ReportCase[string, string, string]) map[string]EvaluationResult { return rc.Assertions },
			wantValue:  Bool(false),
			wantReason: "because",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ds := lcSingleCaseDataset(t, tt.emitter)
			report, err := ds.Evaluate(context.Background(), lcOKTask)
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			if len(report.Cases) != 1 {
				t.Fatalf("expected 1 case, got %d (failures %d)", len(report.Cases), len(report.Failures))
			}
			results := tt.bucket(report.Cases[0])
			got, ok := results[tt.emitter.name]
			if !ok {
				t.Fatalf("result %q not found in %v", tt.emitter.name, results)
			}
			if got.Value != tt.wantValue {
				t.Fatalf("Value = %v, want %v", got.Value, tt.wantValue)
			}
			if got.Reason != tt.wantReason {
				t.Fatalf("Reason = %q, want %q", got.Reason, tt.wantReason)
			}
			if got.Source.Name != tt.emitter.name {
				t.Fatalf("Source.Name = %q, want %q", got.Source.Name, tt.emitter.name)
			}
		})
	}
}

func TestLifecycleEvaluationReasonConstructor(t *testing.T) {
	r := EvaluationReason{Value: Int(3), Reason: "explanation"}
	if r.Value.String() != "3" {
		t.Fatalf("Value.String() = %q, want %q", r.Value.String(), "3")
	}
	if r.Reason != "explanation" {
		t.Fatalf("Reason = %q, want %q", r.Reason, "explanation")
	}
	if got := fmt.Sprintf("%s", r.Value); got != "3" {
		t.Fatalf("formatted scalar = %q, want %q", got, "3")
	}
}
