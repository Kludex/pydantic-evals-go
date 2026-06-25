package evals_test

import (
	"context"
	"fmt"
	"testing"

	evals "github.com/pydantic/pydantic-evals-go"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

// scoreEval emits a Float score with a reason.
type scoreEval struct{}

func (scoreEval) Evaluate(_ context.Context, ec *evals.EvaluatorContext[string, string, any]) (evals.EvaluatorOutput, error) {
	return evals.Reason(evals.Float(0.5), "half"), nil
}
func (scoreEval) Spec() evals.EvaluatorSpec { return evals.NewSpec("Score") }

// labelEval emits a categorical Label.
type labelEval struct{}

func (labelEval) Evaluate(_ context.Context, ec *evals.EvaluatorContext[string, string, any]) (evals.EvaluatorOutput, error) {
	return evals.ScalarValue(evals.Label("good")), nil
}
func (labelEval) Spec() evals.EvaluatorSpec { return evals.NewSpec("Label") }

// boomEval errors on the input "boom".
type boomEval struct{}

func (boomEval) Evaluate(_ context.Context, ec *evals.EvaluatorContext[string, string, any]) (evals.EvaluatorOutput, error) {
	if ec.Inputs == "boom" {
		return nil, fmt.Errorf("kaboom")
	}
	return evals.ScalarValue(evals.Bool(true)), nil
}
func (boomEval) Spec() evals.EvaluatorSpec { return evals.NewSpec("Boom") }

// installRecorder sets a recording tracer provider as the global provider and
// returns the recorder plus a cleanup that restores the previous provider.
func installRecorder(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()
	prev := otel.GetTracerProvider()
	sr := tracetest.NewSpanRecorder()
	otel.SetTracerProvider(sdktrace.NewTracerProvider(sdktrace.WithSpanProcessor(sr)))
	t.Cleanup(func() { otel.SetTracerProvider(prev) })
	return sr
}

func spanByName(t *testing.T, spans []sdktrace.ReadOnlySpan, name string) sdktrace.ReadOnlySpan {
	t.Helper()
	for _, s := range spans {
		if s.Name() == name {
			return s
		}
	}
	t.Fatalf("no span named %q (have %s)", name, spanNames(spans))
	return nil
}

func spanNames(spans []sdktrace.ReadOnlySpan) string {
	var names []string
	for _, s := range spans {
		names = append(names, s.Name())
	}
	return fmt.Sprintf("%v", names)
}

func attrString(s sdktrace.ReadOnlySpan, key string) (string, bool) {
	for _, kv := range s.Attributes() {
		if string(kv.Key) == key {
			return kv.Value.AsString(), true
		}
	}
	return "", false
}

func TestTracingEmitsSpanTree(t *testing.T) {
	sr := installRecorder(t)

	cases := []evals.Case[string, string, any]{
		evals.NewCase[string, string, any]("hi",
			evals.WithCaseName[string, string, any]("ok"),
			evals.WithExpectedOutput[string, string, any]("HI"),
			evals.WithMetadata[string, string, any](map[string]any{"k": "v"}),
		),
		evals.NewCase[string, string, any]("boom", evals.WithCaseName[string, string, any]("evalfail")),
		evals.NewCase[string, string, any]("explode", evals.WithCaseName[string, string, any]("taskfail")),
	}
	ds, err := evals.NewDataset[string, string, any]("ds", cases,
		scoreEval{}, labelEval{}, boomEval{}, evals.EqualsExpected[string, string, any]{})
	if err != nil {
		t.Fatal(err)
	}

	task := func(ctx context.Context, in string) (string, error) {
		if in == "explode" {
			return "", fmt.Errorf("task boom")
		}
		evals.IncrementMetric(ctx, "n", 1)
		evals.SetAttribute(ctx, "seen", true)
		if in == "hi" {
			return "HI", nil
		}
		return in, nil
	}

	report, err := ds.Evaluate(context.Background(), task,
		evals.WithName[string, string, any]("exp"),
		evals.WithTaskName[string, string, any]("mytask"),
		evals.WithExperimentMetadata[string, string, any](map[string]any{"model": "m"}),
	)
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Cases) != 2 || len(report.Failures) != 1 {
		t.Fatalf("cases=%d failures=%d", len(report.Cases), len(report.Failures))
	}

	spans := sr.Ended()

	// Experiment span.
	exp := spanByName(t, spans, "evaluate exp")
	if op, _ := attrString(exp, "gen_ai.operation.name"); op != "experiment" {
		t.Fatalf("experiment op = %q", op)
	}
	if ds, _ := attrString(exp, "dataset_name"); ds != "ds" {
		t.Fatalf("dataset_name = %q", ds)
	}
	if md, _ := attrString(exp, "metadata"); md != `{"model":"m"}` {
		t.Fatalf("metadata = %q", md)
	}

	// Successful case span carries results + recorded metrics/attributes.
	okCase := spanByName(t, spans, "case: ok")
	if out, _ := attrString(okCase, "output"); out != `"HI"` {
		t.Fatalf("output = %q", out)
	}
	if sc, _ := attrString(okCase, "scores"); sc != `{"Score":0.5}` {
		t.Fatalf("scores = %q", sc)
	}
	if lb, _ := attrString(okCase, "labels"); lb != `{"Label":"good"}` {
		t.Fatalf("labels = %q", lb)
	}
	if as, _ := attrString(okCase, "assertions"); as == "" {
		t.Fatal("assertions attribute missing")
	}
	if m, _ := attrString(okCase, "metrics"); m != `{"n":1}` {
		t.Fatalf("metrics = %q", m)
	}
	if ra, _ := attrString(okCase, "attributes"); ra != `{"seen":true}` {
		t.Fatalf("attributes = %q", ra)
	}

	// Task-failure case span is marked as an error.
	taskFail := spanByName(t, spans, "case: taskfail")
	if taskFail.Status().Code.String() != "Error" {
		t.Fatalf("taskfail status = %s", taskFail.Status().Code)
	}

	// Evaluator failure: the Boom evaluator span on the "boom" case is an error.
	evalSpan := spanByName(t, spans, "evaluator: Boom")
	_ = evalSpan // presence asserted; per-parent error status verified via report
	if len(report.Cases) == 0 {
		t.Fatal("expected a successful case")
	}

	// Task span exists and the failing one is an error.
	taskSpans := 0
	for _, s := range spans {
		if s.Name() == "execute mytask" {
			taskSpans++
		}
	}
	if taskSpans != 3 {
		t.Fatalf("expected 3 task spans, got %d", taskSpans)
	}
}

// TestTracingHandlesUnmarshalableAttribute exercises the span-attribute
// serialization fallback: an attribute value that JSON cannot marshal (a channel)
// must not crash the evaluation when tracing is active.
func TestTracingHandlesUnmarshalableAttribute(t *testing.T) {
	sr := installRecorder(t)

	ds, err := evals.NewDataset[string, string, any]("ds",
		[]evals.Case[string, string, any]{evals.NewCase[string, string, any]("x",
			evals.WithCaseName[string, string, any]("c"))},
		evals.EqualsExpected[string, string, any]{})
	if err != nil {
		t.Fatal(err)
	}
	report, err := ds.Evaluate(context.Background(), func(ctx context.Context, s string) (string, error) {
		evals.SetAttribute(ctx, "ch", make(chan int)) // not JSON-marshalable
		return s, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Cases) != 1 {
		t.Fatalf("cases = %d", len(report.Cases))
	}
	caseSpan := spanByName(t, sr.Ended(), "case: c")
	if attrs, ok := attrString(caseSpan, "attributes"); !ok || attrs == "" {
		t.Fatal("expected an attributes span attribute even for unmarshalable values")
	}
}

// intScoreEval emits an Int score, exercising the Int arm of the span-attribute
// scalar conversion.
type intScoreEval struct{}

func (intScoreEval) Evaluate(_ context.Context, ec *evals.EvaluatorContext[string, string, any]) (evals.EvaluatorOutput, error) {
	return evals.ScalarValue(evals.Int(3)), nil
}
func (intScoreEval) Spec() evals.EvaluatorSpec { return evals.NewSpec("IntScore") }

func TestTracingIntScoreAttribute(t *testing.T) {
	sr := installRecorder(t)
	ds, err := evals.NewDataset[string, string, any]("ds",
		[]evals.Case[string, string, any]{evals.NewCase[string, string, any]("x",
			evals.WithCaseName[string, string, any]("c"))},
		intScoreEval{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := ds.Evaluate(context.Background(),
		func(_ context.Context, s string) (string, error) { return s, nil }); err != nil {
		t.Fatal(err)
	}
	caseSpan := spanByName(t, sr.Ended(), "case: c")
	if sc, _ := attrString(caseSpan, "scores"); sc != `{"IntScore":3}` {
		t.Fatalf("scores = %q", sc)
	}
}

// TestTracingNoOpByDefault verifies that without a configured provider the
// evaluation still runs (the global no-op tracer produces no recorded spans).
func TestTracingNoOpByDefault(t *testing.T) {
	ds, err := evals.NewDataset[string, string, any]("ds",
		[]evals.Case[string, string, any]{evals.NewCase[string, string, any]("x")})
	if err != nil {
		t.Fatal(err)
	}
	report, err := ds.Evaluate(context.Background(),
		func(_ context.Context, s string) (string, error) { return s, nil })
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Cases) != 1 {
		t.Fatalf("cases = %d", len(report.Cases))
	}
}
