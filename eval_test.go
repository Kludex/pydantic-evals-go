package evals

import (
	"context"
	"errors"
	"math"
	"testing"
)

// scalarEvaluator returns a fixed Scalar wrapped via ScalarValue. It customizes
// its result name via NamedEvaluator and tags a version via VersionedEvaluator.
type scalarEvaluator struct {
	name    string
	version string
	value   Scalar
}

func (e scalarEvaluator) Evaluate(_ context.Context, _ *EvaluatorContext[int, int, int]) (EvaluatorOutput, error) {
	return ScalarValue(e.value), nil
}
func (e scalarEvaluator) Spec() EvaluatorSpec           { return NewSpec("scalar") }
func (e scalarEvaluator) DefaultEvaluationName() string { return e.name }
func (e scalarEvaluator) EvaluatorVersion() string      { return e.version }

// reasonEvaluator returns a Scalar with an explanation via Reason.
type reasonEvaluator struct {
	name   string
	value  Scalar
	reason string
}

func (e reasonEvaluator) Evaluate(_ context.Context, _ *EvaluatorContext[int, int, int]) (EvaluatorOutput, error) {
	return Reason(e.value, e.reason), nil
}
func (e reasonEvaluator) Spec() EvaluatorSpec           { return NewSpec("reason") }
func (e reasonEvaluator) DefaultEvaluationName() string { return e.name }

// scalarMapEvaluator returns a ScalarMapOutput, exercising the map normalization
// path with multiple named results in one evaluator.
type scalarMapEvaluator struct {
	out ScalarMapOutput
}

func (e scalarMapEvaluator) Evaluate(_ context.Context, _ *EvaluatorContext[int, int, int]) (EvaluatorOutput, error) {
	return e.out, nil
}
func (e scalarMapEvaluator) Spec() EvaluatorSpec { return NewSpec("scalar_map") }

// reasonMapEvaluator returns a ReasonMapOutput.
type reasonMapEvaluator struct {
	out ReasonMapOutput
}

func (e reasonMapEvaluator) Evaluate(_ context.Context, _ *EvaluatorContext[int, int, int]) (EvaluatorOutput, error) {
	return e.out, nil
}
func (e reasonMapEvaluator) Spec() EvaluatorSpec { return NewSpec("reason_map") }

// erroringEvaluator always returns an error from Evaluate.
type erroringEvaluator struct {
	err error
}

func (e erroringEvaluator) Evaluate(_ context.Context, _ *EvaluatorContext[int, int, int]) (EvaluatorOutput, error) {
	return nil, e.err
}
func (e erroringEvaluator) Spec() EvaluatorSpec { return NewSpec("erroring") }

// nilOutputEvaluator returns a nil EvaluatorOutput with no error.
type nilOutputEvaluator struct{}

func (nilOutputEvaluator) Evaluate(_ context.Context, _ *EvaluatorContext[int, int, int]) (EvaluatorOutput, error) {
	return nil, nil
}
func (nilOutputEvaluator) Spec() EvaluatorSpec { return NewSpec("nil_output") }

// nonFiniteEvaluator returns a non-finite Float that fails normalization.
type nonFiniteEvaluator struct {
	value Float
}

func (e nonFiniteEvaluator) Evaluate(_ context.Context, _ *EvaluatorContext[int, int, int]) (EvaluatorOutput, error) {
	return ScalarValue(e.value), nil
}
func (e nonFiniteEvaluator) Spec() EvaluatorSpec { return NewSpec("non_finite") }

// contextEvaluator records the EvaluatorContext it was given for assertions.
type contextEvaluator struct {
	name     string
	captured *EvaluatorContext[int, int, int]
}

func (e *contextEvaluator) Evaluate(_ context.Context, ec *EvaluatorContext[int, int, int]) (EvaluatorOutput, error) {
	e.captured = ec
	return ScalarValue(Bool(true)), nil
}
func (e *contextEvaluator) Spec() EvaluatorSpec           { return NewSpec("context") }
func (e *contextEvaluator) DefaultEvaluationName() string { return e.name }

func newIntDataset(t *testing.T, name string, cases ...Case[int, int, int]) *Dataset[int, int, int] {
	t.Helper()
	d, err := NewDataset(name, cases)
	if err != nil {
		t.Fatalf("NewDataset(%q): unexpected error: %v", name, err)
	}
	return d
}

func identityTask(_ context.Context, in int) (int, error) { return in, nil }

func evaluate(t *testing.T, d *Dataset[int, int, int], task TaskFunc[int, int], opts ...EvaluateOption[int, int, int]) *EvaluationReport[int, int, int] {
	t.Helper()
	report, err := d.Evaluate(context.Background(), task, opts...)
	if err != nil {
		t.Fatalf("Evaluate: unexpected error: %v", err)
	}
	return report
}

func findCase[I, O, M any](t *testing.T, report *EvaluationReport[I, O, M], name string) ReportCase[I, O, M] {
	t.Helper()
	for _, c := range report.Cases {
		if c.Name == name {
			return c
		}
	}
	t.Fatalf("case %q not found in report (cases: %d, failures: %d)", name, len(report.Cases), len(report.Failures))
	return ReportCase[I, O, M]{}
}

func TestNewCaseOptions(t *testing.T) {
	ev := scalarEvaluator{name: "x", value: Bool(true)}
	c := NewCase(
		7,
		WithCaseName[int, int, int]("mycase"),
		WithMetadata[int, int, int](42),
		WithExpectedOutput[int, int, int](7),
		WithCaseEvaluators[int, int, int](ev),
	)
	if c.Name != "mycase" {
		t.Errorf("Name = %q, want %q", c.Name, "mycase")
	}
	if c.Inputs != 7 {
		t.Errorf("Inputs = %d, want 7", c.Inputs)
	}
	if !c.HasMetadata || c.Metadata != 42 {
		t.Errorf("Metadata = %d (has=%v), want 42 (has=true)", c.Metadata, c.HasMetadata)
	}
	if !c.HasExpectedOutput || c.ExpectedOutput != 7 {
		t.Errorf("ExpectedOutput = %d (has=%v), want 7 (has=true)", c.ExpectedOutput, c.HasExpectedOutput)
	}
	if len(c.Evaluators) != 1 {
		t.Fatalf("len(Evaluators) = %d, want 1", len(c.Evaluators))
	}
}

func TestNewCaseNoOptions(t *testing.T) {
	c := NewCase[int, int, int](3)
	if c.Inputs != 3 {
		t.Errorf("Inputs = %d, want 3", c.Inputs)
	}
	if c.HasMetadata || c.HasExpectedOutput {
		t.Errorf("flags should be false: HasMetadata=%v HasExpectedOutput=%v", c.HasMetadata, c.HasExpectedOutput)
	}
	if c.Name != "" || len(c.Evaluators) != 0 {
		t.Errorf("unexpected Name=%q or Evaluators=%d", c.Name, len(c.Evaluators))
	}
}

func TestNewDatasetDuplicateName(t *testing.T) {
	cases := []Case[int, int, int]{
		NewCase(1, WithCaseName[int, int, int]("dup")),
		NewCase(2, WithCaseName[int, int, int]("dup")),
	}
	_, err := NewDataset("ds", cases)
	if err == nil {
		t.Fatal("NewDataset: expected duplicate name error, got nil")
	}
	if got, want := err.Error(), `duplicate case name: "dup"`; got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
}

func TestNewDatasetEmptyNamesAllowed(t *testing.T) {
	cases := []Case[int, int, int]{NewCase[int, int, int](1), NewCase[int, int, int](2)}
	d, err := NewDataset("ds", cases)
	if err != nil {
		t.Fatalf("NewDataset: unexpected error: %v", err)
	}
	if len(d.Cases) != 2 || d.Name != "ds" {
		t.Errorf("unexpected dataset: name=%q cases=%d", d.Name, len(d.Cases))
	}
}

func TestAddCase(t *testing.T) {
	d := newIntDataset(t, "ds")
	if err := d.AddCase(NewCase(1, WithCaseName[int, int, int]("a"))); err != nil {
		t.Fatalf("AddCase: unexpected error: %v", err)
	}
	if err := d.AddCase(NewCase[int, int, int](2)); err != nil {
		t.Fatalf("AddCase(unnamed): unexpected error: %v", err)
	}
	err := d.AddCase(NewCase(3, WithCaseName[int, int, int]("a")))
	if err == nil {
		t.Fatal("AddCase: expected duplicate name error, got nil")
	}
	if got, want := err.Error(), `duplicate case name: "a"`; got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
	if len(d.Cases) != 2 {
		t.Errorf("len(Cases) = %d, want 2", len(d.Cases))
	}
}

func TestAddEvaluator(t *testing.T) {
	d := newIntDataset(t, "ds", NewCase(1, WithCaseName[int, int, int]("a")))
	d.AddEvaluator(scalarEvaluator{name: "global", value: Bool(true)})
	if len(d.Evaluators) != 1 {
		t.Fatalf("len(Evaluators) = %d, want 1", len(d.Evaluators))
	}
	report := evaluate(t, d, identityTask)
	c := findCase(t, report, "a")
	if _, ok := c.Assertions["global"]; !ok {
		t.Errorf("expected assertion %q, got %v", "global", c.Assertions)
	}
}

func TestAddEvaluatorForCase(t *testing.T) {
	d := newIntDataset(t,
		"ds",
		NewCase(1, WithCaseName[int, int, int]("a")),
		NewCase(2, WithCaseName[int, int, int]("b")),
	)
	if err := d.AddEvaluatorForCase("a", scalarEvaluator{name: "onlyA", value: Bool(true)}); err != nil {
		t.Fatalf("AddEvaluatorForCase(a): unexpected error: %v", err)
	}
	err := d.AddEvaluatorForCase("missing", scalarEvaluator{name: "x", value: Bool(true)})
	if err == nil {
		t.Fatal("AddEvaluatorForCase(missing): expected error, got nil")
	}
	if got, want := err.Error(), `case "missing" not found in the dataset`; got != want {
		t.Errorf("error = %q, want %q", got, want)
	}

	report := evaluate(t, d, identityTask)
	ca := findCase(t, report, "a")
	if _, ok := ca.Assertions["onlyA"]; !ok {
		t.Errorf("case a missing assertion onlyA: %v", ca.Assertions)
	}
	cb := findCase(t, report, "b")
	if _, ok := cb.Assertions["onlyA"]; ok {
		t.Errorf("case b should not have assertion onlyA: %v", cb.Assertions)
	}
}

func TestEvaluatePassingTask(t *testing.T) {
	d := newIntDataset(t, "ds", NewCase(5, WithCaseName[int, int, int]("c1")))
	d.AddEvaluator(scalarEvaluator{name: "ok", value: Bool(true)})
	report := evaluate(t, d, identityTask)
	if len(report.Failures) != 0 {
		t.Fatalf("expected no failures, got %d", len(report.Failures))
	}
	c := findCase(t, report, "c1")
	if c.Output != 5 {
		t.Errorf("Output = %d, want 5", c.Output)
	}
	if report.Name != "task" {
		t.Errorf("report.Name = %q, want %q", report.Name, "task")
	}
}

func TestEvaluateTaskError(t *testing.T) {
	d := newIntDataset(t, "ds", NewCase(1, WithCaseName[int, int, int]("boom")))
	boom := errors.New("task exploded")
	task := func(_ context.Context, _ int) (int, error) { return 0, boom }
	report := evaluate(t, d, task)
	if len(report.Cases) != 0 {
		t.Fatalf("expected no successful cases, got %d", len(report.Cases))
	}
	if len(report.Failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(report.Failures))
	}
	f := report.Failures[0]
	if f.Name != "boom" {
		t.Errorf("failure Name = %q, want %q", f.Name, "boom")
	}
	if f.ErrorMessage != "task exploded" {
		t.Errorf("ErrorMessage = %q, want %q", f.ErrorMessage, "task exploded")
	}
	if f.ErrorType != "errorString" {
		t.Errorf("ErrorType = %q, want %q", f.ErrorType, "errorString")
	}
}

func TestEvaluateScalarReasonAndMaps(t *testing.T) {
	d := newIntDataset(t, "ds", NewCase(1, WithCaseName[int, int, int]("c")))
	d.AddEvaluator(scalarEvaluator{name: "score", value: Int(3)})
	d.AddEvaluator(reasonEvaluator{name: "labelled", value: Label("good"), reason: "looks good"})
	d.AddEvaluator(scalarMapEvaluator{out: ScalarMapOutput{
		"smap_assert": Bool(true),
		"smap_score":  Float(0.5),
	}})
	d.AddEvaluator(reasonMapEvaluator{out: ReasonMapOutput{
		"rmap_label": {Value: Label("cat"), Reason: "because"},
	}})

	report := evaluate(t, d, identityTask)
	c := findCase(t, report, "c")

	if got := c.Scores["score"]; got.Value != Int(3) {
		t.Errorf("Scores[score].Value = %v, want Int(3)", got.Value)
	}
	if got := c.Labels["labelled"]; got.Value != Label("good") || got.Reason != "looks good" {
		t.Errorf("Labels[labelled] = %+v, want value=good reason='looks good'", got)
	}
	if got := c.Assertions["smap_assert"]; got.Value != Bool(true) {
		t.Errorf("Assertions[smap_assert].Value = %v, want Bool(true)", got.Value)
	}
	if got := c.Scores["smap_score"]; got.Value != Float(0.5) {
		t.Errorf("Scores[smap_score].Value = %v, want Float(0.5)", got.Value)
	}
	if got := c.Labels["rmap_label"]; got.Value != Label("cat") || got.Reason != "because" {
		t.Errorf("Labels[rmap_label] = %+v, want value=cat reason='because'", got)
	}
}

func TestEvaluateEvaluatorErrorBecomesFailure(t *testing.T) {
	d := newIntDataset(t, "ds", NewCase(1, WithCaseName[int, int, int]("c")))
	d.AddEvaluator(scalarEvaluator{name: "ok", value: Bool(true)})
	d.AddEvaluator(erroringEvaluator{err: errors.New("eval kaput")})

	report := evaluate(t, d, identityTask)
	c := findCase(t, report, "c")
	if len(c.EvaluatorFailures) != 1 {
		t.Fatalf("len(EvaluatorFailures) = %d, want 1", len(c.EvaluatorFailures))
	}
	f := c.EvaluatorFailures[0]
	if f.Name != "erroring" {
		t.Errorf("failure Name = %q, want %q", f.Name, "erroring")
	}
	if f.ErrorMessage != "eval kaput" {
		t.Errorf("ErrorMessage = %q, want %q", f.ErrorMessage, "eval kaput")
	}
	if _, ok := c.Assertions["ok"]; !ok {
		t.Errorf("other evaluator should still succeed: %v", c.Assertions)
	}
}

func TestEvaluateNonFiniteFloatError(t *testing.T) {
	for _, tc := range []struct {
		name  string
		value Float
		want  string
	}{
		{"inf", Float(math.Inf(1)), "evaluator returned a non-finite float score: +Inf"},
		{"nan", Float(math.NaN()), "evaluator returned a non-finite float score: NaN"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := newIntDataset(t, "ds", NewCase(1, WithCaseName[int, int, int]("c")))
			d.AddEvaluator(nonFiniteEvaluator{value: tc.value})
			report := evaluate(t, d, identityTask)
			c := findCase(t, report, "c")
			if len(c.EvaluatorFailures) != 1 {
				t.Fatalf("len(EvaluatorFailures) = %d, want 1", len(c.EvaluatorFailures))
			}
			if got := c.EvaluatorFailures[0].ErrorMessage; got != tc.want {
				t.Errorf("ErrorMessage = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEvaluateNilOutputError(t *testing.T) {
	d := newIntDataset(t, "ds", NewCase(1, WithCaseName[int, int, int]("c")))
	d.AddEvaluator(nilOutputEvaluator{})
	report := evaluate(t, d, identityTask)
	c := findCase(t, report, "c")
	if len(c.EvaluatorFailures) != 1 {
		t.Fatalf("len(EvaluatorFailures) = %d, want 1", len(c.EvaluatorFailures))
	}
	if got, want := c.EvaluatorFailures[0].ErrorMessage, `evaluator "nil_output" returned a nil output`; got != want {
		t.Errorf("ErrorMessage = %q, want %q", got, want)
	}
}

// reasonNonFiniteEvaluator returns a non-finite Float via Reason.
type reasonNonFiniteEvaluator struct{}

func (reasonNonFiniteEvaluator) Evaluate(_ context.Context, _ *EvaluatorContext[int, int, int]) (EvaluatorOutput, error) {
	return Reason(Float(math.Inf(1)), "why"), nil
}
func (reasonNonFiniteEvaluator) Spec() EvaluatorSpec { return NewSpec("reason_nonfinite") }

// scalarMapNonFiniteEvaluator returns a non-finite Float inside a ScalarMapOutput.
type scalarMapNonFiniteEvaluator struct{}

func (scalarMapNonFiniteEvaluator) Evaluate(_ context.Context, _ *EvaluatorContext[int, int, int]) (EvaluatorOutput, error) {
	return ScalarMapOutput{"bad": Float(math.NaN())}, nil
}
func (scalarMapNonFiniteEvaluator) Spec() EvaluatorSpec { return NewSpec("smap_nonfinite") }

// reasonMapNonFiniteEvaluator returns a non-finite Float inside a ReasonMapOutput.
type reasonMapNonFiniteEvaluator struct{}

func (reasonMapNonFiniteEvaluator) Evaluate(_ context.Context, _ *EvaluatorContext[int, int, int]) (EvaluatorOutput, error) {
	return ReasonMapOutput{"bad": {Value: Float(math.Inf(-1)), Reason: "r"}}, nil
}
func (reasonMapNonFiniteEvaluator) Spec() EvaluatorSpec { return NewSpec("rmap_nonfinite") }

func TestNonFiniteFloatAcrossOutputKinds(t *testing.T) {
	for _, tc := range []struct {
		name string
		ev   Evaluator[int, int, int]
		want string
	}{
		{"reason", reasonNonFiniteEvaluator{}, "evaluator returned a non-finite float score: +Inf"},
		{"scalarMap", scalarMapNonFiniteEvaluator{}, "evaluator returned a non-finite float score: NaN"},
		{"reasonMap", reasonMapNonFiniteEvaluator{}, "evaluator returned a non-finite float score: -Inf"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := newIntDataset(t, "ds", NewCase(1, WithCaseName[int, int, int]("c")))
			d.AddEvaluator(tc.ev)
			report := evaluate(t, d, identityTask)
			c := findCase(t, report, "c")
			if len(c.EvaluatorFailures) != 1 {
				t.Fatalf("len(EvaluatorFailures) = %d, want 1", len(c.EvaluatorFailures))
			}
			if got := c.EvaluatorFailures[0].ErrorMessage; got != tc.want {
				t.Errorf("ErrorMessage = %q, want %q", got, tc.want)
			}
		})
	}
}

// metricLifecycle enriches the evaluator context in PrepareContext and records
// the teardown result for assertions.
type metricLifecycle struct {
	BaseLifecycle[int, int, int]
	setupCalled    bool
	teardownResult *ReportCase[int, int, int]
}

func (l *metricLifecycle) Setup(_ context.Context) error {
	l.setupCalled = true
	return nil
}
func (l *metricLifecycle) PrepareContext(_ context.Context, ec *EvaluatorContext[int, int, int]) (*EvaluatorContext[int, int, int], error) {
	ec.Attributes["from_prepare"] = true
	return ec, nil
}
func (l *metricLifecycle) Teardown(_ context.Context, result *ReportCase[int, int, int], _ *ReportCaseFailure[int, int, int]) error {
	l.teardownResult = result
	return nil
}

func TestEvaluateWithLifecycle(t *testing.T) {
	d := newIntDataset(t, "ds", NewCase(1, WithCaseName[int, int, int]("c")))
	captured := &contextEvaluator{name: "capture"}
	d.AddEvaluator(captured)

	var created *metricLifecycle
	report := evaluate(t, d, identityTask, WithLifecycle(func(_ Case[int, int, int]) Lifecycle[int, int, int] {
		created = &metricLifecycle{}
		return created
	}))
	c := findCase(t, report, "c")

	if !created.setupCalled {
		t.Error("Setup was not called")
	}
	if c.Attributes["from_prepare"] != true {
		t.Errorf("PrepareContext enrichment missing: %v", c.Attributes)
	}
	if captured.captured.Attributes["from_prepare"] != true {
		t.Errorf("evaluator did not see prepared attribute: %v", captured.captured.Attributes)
	}
	if created.teardownResult == nil || created.teardownResult.Name != "c" {
		t.Errorf("Teardown result = %+v, want case c", created.teardownResult)
	}
}

// failingSetupLifecycle errors in Setup, producing a setup failure.
type failingSetupLifecycle struct {
	BaseLifecycle[int, int, int]
}

func (failingSetupLifecycle) Setup(_ context.Context) error { return errors.New("setup boom") }

// failingPrepareLifecycle errors in PrepareContext.
type failingPrepareLifecycle struct {
	BaseLifecycle[int, int, int]
}

func (failingPrepareLifecycle) PrepareContext(_ context.Context, _ *EvaluatorContext[int, int, int]) (*EvaluatorContext[int, int, int], error) {
	return nil, errors.New("prepare boom")
}

// failingTeardownLifecycle errors in Teardown, turning a success into a failure.
type failingTeardownLifecycle struct {
	BaseLifecycle[int, int, int]
}

func (failingTeardownLifecycle) Teardown(_ context.Context, _ *ReportCase[int, int, int], _ *ReportCaseFailure[int, int, int]) error {
	return errors.New("teardown boom")
}

func TestEvaluateLifecycleErrors(t *testing.T) {
	for _, tc := range []struct {
		name    string
		factory func(Case[int, int, int]) Lifecycle[int, int, int]
		want    string
	}{
		{"setup", func(Case[int, int, int]) Lifecycle[int, int, int] { return failingSetupLifecycle{} }, "setup: setup boom"},
		{"prepare", func(Case[int, int, int]) Lifecycle[int, int, int] { return failingPrepareLifecycle{} }, "prepare context: prepare boom"},
		{"teardown", func(Case[int, int, int]) Lifecycle[int, int, int] { return failingTeardownLifecycle{} }, "teardown: teardown boom"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			d := newIntDataset(t, "ds", NewCase(1, WithCaseName[int, int, int]("c")))
			report := evaluate(t, d, identityTask, WithLifecycle(tc.factory))
			if len(report.Cases) != 0 {
				t.Fatalf("expected no successful cases, got %d", len(report.Cases))
			}
			if len(report.Failures) != 1 {
				t.Fatalf("expected 1 failure, got %d", len(report.Failures))
			}
			if got := report.Failures[0].ErrorMessage; got != tc.want {
				t.Errorf("ErrorMessage = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestEvaluateCancelledContext(t *testing.T) {
	d := newIntDataset(t, "ds", NewCase(1, WithCaseName[int, int, int]("c")))
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	report, err := d.Evaluate(ctx, identityTask)
	if err == nil {
		t.Fatal("Evaluate: expected context cancellation error, got nil")
	}
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v, want context.Canceled", err)
	}
	if report != nil {
		t.Errorf("report = %v, want nil on cancellation", report)
	}
}

func TestBaseLifecycleNoOps(t *testing.T) {
	var lc Lifecycle[int, int, int] = BaseLifecycle[int, int, int]{}
	if err := lc.Setup(context.Background()); err != nil {
		t.Errorf("Setup error: %v", err)
	}
	ec := &EvaluatorContext[int, int, int]{}
	got, err := lc.PrepareContext(context.Background(), ec)
	if err != nil || got != ec {
		t.Errorf("PrepareContext returned (%v, %v), want (ec, nil)", got, err)
	}
	if err := lc.Teardown(context.Background(), nil, nil); err != nil {
		t.Errorf("Teardown error: %v", err)
	}
}

func TestEvaluateOptionsNameAndTaskName(t *testing.T) {
	t.Run("defaults to task", func(t *testing.T) {
		d := newIntDataset(t, "ds", NewCase[int, int, int](1))
		report := evaluate(t, d, identityTask)
		if report.Name != "task" {
			t.Errorf("report.Name = %q, want %q", report.Name, "task")
		}
	})
	t.Run("name defaults to task name", func(t *testing.T) {
		d := newIntDataset(t, "ds", NewCase[int, int, int](1))
		report := evaluate(t, d, identityTask, WithTaskName[int, int, int]("my-task"))
		if report.Name != "my-task" {
			t.Errorf("report.Name = %q, want %q", report.Name, "my-task")
		}
	})
	t.Run("explicit name overrides", func(t *testing.T) {
		d := newIntDataset(t, "ds", NewCase[int, int, int](1))
		report := evaluate(t, d, identityTask,
			WithTaskName[int, int, int]("my-task"),
			WithName[int, int, int]("experiment-1"),
		)
		if report.Name != "experiment-1" {
			t.Errorf("report.Name = %q, want %q", report.Name, "experiment-1")
		}
	})
}

func TestEvaluateMaxConcurrency(t *testing.T) {
	cases := make([]Case[int, int, int], 0, 6)
	for i := 0; i < 6; i++ {
		cases = append(cases, NewCase(i, WithCaseName[int, int, int](Int(i).String())))
	}
	d := newIntDataset(t, "ds", cases...)
	d.AddEvaluator(scalarEvaluator{name: "ok", value: Bool(true)})
	report := evaluate(t, d, identityTask, WithMaxConcurrency[int, int, int](2))
	if len(report.Cases) != 6 {
		t.Fatalf("len(Cases) = %d, want 6", len(report.Cases))
	}
}

func TestEvaluateExperimentMetadata(t *testing.T) {
	d := newIntDataset(t, "ds", NewCase[int, int, int](1))
	meta := map[string]any{"model": "gpt", "temperature": 0.7}
	report := evaluate(t, d, identityTask, WithExperimentMetadata[int, int, int](meta))
	if report.ExperimentMetadata["model"] != "gpt" {
		t.Errorf("ExperimentMetadata = %v, want model=gpt", report.ExperimentMetadata)
	}
}

func TestEvaluateRepeatGrouping(t *testing.T) {
	d := newIntDataset(t, "ds", NewCase(1, WithCaseName[int, int, int]("c")))
	d.AddEvaluator(scalarEvaluator{name: "ok", value: Bool(true)})
	report := evaluate(t, d, identityTask, WithRepeat[int, int, int](3))
	if len(report.Cases) != 3 {
		t.Fatalf("len(Cases) = %d, want 3", len(report.Cases))
	}
	wantNames := map[string]bool{"c [1/3]": true, "c [2/3]": true, "c [3/3]": true}
	for _, c := range report.Cases {
		if !wantNames[c.Name] {
			t.Errorf("unexpected case name %q", c.Name)
		}
		if c.SourceCaseName != "c" {
			t.Errorf("SourceCaseName = %q, want %q", c.SourceCaseName, "c")
		}
	}
	groups := report.CaseGroups()
	if len(groups) != 1 {
		t.Fatalf("len(CaseGroups) = %d, want 1", len(groups))
	}
	if groups[0].Name != "c" || len(groups[0].Runs) != 3 {
		t.Errorf("group = name %q runs %d, want c/3", groups[0].Name, len(groups[0].Runs))
	}
	if avg := report.Averages(); avg == nil {
		t.Error("Averages() = nil, want non-nil")
	}
}

func TestEvaluateRepeatInvalid(t *testing.T) {
	d := newIntDataset(t, "ds", NewCase[int, int, int](1))
	_, err := d.Evaluate(context.Background(), identityTask, WithRepeat[int, int, int](0))
	if err == nil {
		t.Fatal("Evaluate: expected error for repeat < 1, got nil")
	}
	if got, want := err.Error(), "repeat must be >= 1, got 0"; got != want {
		t.Errorf("error = %q, want %q", got, want)
	}
}

func TestEvaluateUnnamedCasesGetGenericNames(t *testing.T) {
	d := newIntDataset(t,
		"ds",
		NewCase[int, int, int](1),
		NewCase(2, WithCaseName[int, int, int]("named")),
		NewCase[int, int, int](3),
	)
	report := evaluate(t, d, identityTask)
	names := map[string]bool{}
	for _, c := range report.Cases {
		names[c.Name] = true
	}
	for _, want := range []string{"Case 1", "named", "Case 3"} {
		if !names[want] {
			t.Errorf("missing case name %q in %v", want, names)
		}
	}
}

func TestTaskSetAttributeAndIncrementMetric(t *testing.T) {
	d := newIntDataset(t, "ds", NewCase(1, WithCaseName[int, int, int]("c")))
	captured := &contextEvaluator{name: "capture"}
	d.AddEvaluator(captured)

	task := func(ctx context.Context, in int) (int, error) {
		SetAttribute(ctx, "kind", "demo")
		IncrementMetric(ctx, "tokens", 10)
		IncrementMetric(ctx, "tokens", 5)
		IncrementMetric(ctx, "fresh_zero", 0)
		IncrementMetric(ctx, "back_to_zero", 3)
		IncrementMetric(ctx, "back_to_zero", -3)
		return in, nil
	}
	report := evaluate(t, d, task)
	c := findCase(t, report, "c")

	if c.Attributes["kind"] != "demo" {
		t.Errorf("ReportCase.Attributes[kind] = %v, want demo", c.Attributes["kind"])
	}
	if c.Metrics["tokens"] != 15 {
		t.Errorf("ReportCase.Metrics[tokens] = %v, want 15", c.Metrics["tokens"])
	}
	// Incrementing a never-seen metric by zero is dropped entirely.
	if _, ok := c.Metrics["fresh_zero"]; ok {
		t.Errorf("zero increment on a fresh metric should be dropped, got %v", c.Metrics["fresh_zero"])
	}
	// A metric that returns to zero after being non-zero is kept (stored as 0).
	if v, ok := c.Metrics["back_to_zero"]; !ok || v != 0 {
		t.Errorf("back_to_zero = %v (present=%v), want 0 present", v, ok)
	}

	if captured.captured == nil {
		t.Fatal("evaluator context was not captured")
	}
	if captured.captured.Attributes["kind"] != "demo" {
		t.Errorf("EvaluatorContext.Attributes[kind] = %v, want demo", captured.captured.Attributes["kind"])
	}
	if captured.captured.Metrics["tokens"] != 15 {
		t.Errorf("EvaluatorContext.Metrics[tokens] = %v, want 15", captured.captured.Metrics["tokens"])
	}
}

func TestSetAttributeAndIncrementMetricNonTaskContextNoOp(t *testing.T) {
	ctx := context.Background()
	SetAttribute(ctx, "x", 1)
	IncrementMetric(ctx, "y", 2)
}

func TestDuplicateResultNamesGetSuffixes(t *testing.T) {
	d := newIntDataset(t, "ds", NewCase(1, WithCaseName[int, int, int]("c")))
	d.AddEvaluator(scalarMapEvaluator{out: ScalarMapOutput{
		"dup":   Int(1),
		"dup_2": Int(2),
	}})
	d.AddEvaluator(scalarEvaluator{name: "dup", value: Int(3)})

	report := evaluate(t, d, identityTask)
	c := findCase(t, report, "c")

	for _, want := range []string{"dup", "dup_2", "dup_3"} {
		if _, ok := c.Scores[want]; !ok {
			t.Errorf("missing score %q in %v", want, c.Scores)
		}
	}
}

func TestEvaluatorVersionPropagation(t *testing.T) {
	d := newIntDataset(t, "ds", NewCase(1, WithCaseName[int, int, int]("c")))
	d.AddEvaluator(scalarEvaluator{name: "v_ok", version: "1.2.3", value: Bool(true)})
	d.AddEvaluator(scalarEvaluator{name: "v_err", version: "9.9.9", value: Bool(true)})
	d.AddEvaluator(erroringEvaluator{err: errors.New("boom")})

	report := evaluate(t, d, identityTask)
	c := findCase(t, report, "c")

	if got := c.Assertions["v_ok"].EvaluatorVersion; got != "1.2.3" {
		t.Errorf("EvaluationResult.EvaluatorVersion = %q, want %q", got, "1.2.3")
	}

	// erroringEvaluator does not implement VersionedEvaluator: version is empty.
	d2 := newIntDataset(t, "ds2", NewCase(1, WithCaseName[int, int, int]("c")))
	d2.AddEvaluator(versionedErroringEvaluator{version: "7.0", err: errors.New("nope")})
	report2 := evaluate(t, d2, identityTask)
	c2 := findCase(t, report2, "c")
	if len(c2.EvaluatorFailures) != 1 {
		t.Fatalf("len(EvaluatorFailures) = %d, want 1", len(c2.EvaluatorFailures))
	}
	if got := c2.EvaluatorFailures[0].EvaluatorVersion; got != "7.0" {
		t.Errorf("EvaluatorFailure.EvaluatorVersion = %q, want %q", got, "7.0")
	}
}

// versionedErroringEvaluator errors but reports a version, so the resulting
// EvaluatorFailure carries that version.
type versionedErroringEvaluator struct {
	version string
	err     error
}

func (e versionedErroringEvaluator) Evaluate(_ context.Context, _ *EvaluatorContext[int, int, int]) (EvaluatorOutput, error) {
	return nil, e.err
}
func (e versionedErroringEvaluator) Spec() EvaluatorSpec      { return NewSpec("verr") }
func (e versionedErroringEvaluator) EvaluatorVersion() string { return e.version }

func TestEvaluationResultSourceSpec(t *testing.T) {
	d := newIntDataset(t, "ds", NewCase(1, WithCaseName[int, int, int]("c")))
	d.AddEvaluator(scalarEvaluator{name: "s", value: Bool(true)})
	report := evaluate(t, d, identityTask)
	c := findCase(t, report, "c")
	if got := c.Assertions["s"].Source.Name; got != "scalar" {
		t.Errorf("EvaluationResult.Source.Name = %q, want %q", got, "scalar")
	}
}

func TestCaseLevelEvaluatorViaOption(t *testing.T) {
	d := newIntDataset(t,
		"ds",
		NewCase(1,
			WithCaseName[int, int, int]("c"),
			WithCaseEvaluators[int, int, int](scalarEvaluator{name: "case_only", value: Bool(true)}),
		),
	)
	report := evaluate(t, d, identityTask)
	c := findCase(t, report, "c")
	if _, ok := c.Assertions["case_only"]; !ok {
		t.Errorf("expected case-level assertion %q, got %v", "case_only", c.Assertions)
	}
}

func TestDefaultEvaluationNameFallsBackToSpec(t *testing.T) {
	d := newIntDataset(t, "ds", NewCase(1, WithCaseName[int, int, int]("c")))
	// Empty NamedEvaluator name falls back to the spec name "scalar".
	d.AddEvaluator(scalarEvaluator{name: "", value: Bool(true)})
	report := evaluate(t, d, identityTask)
	c := findCase(t, report, "c")
	if _, ok := c.Assertions["scalar"]; !ok {
		t.Errorf("expected assertion under spec name %q, got %v", "scalar", c.Assertions)
	}
}
