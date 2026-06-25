package evals

import (
	"context"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// otelScope is the instrumentation scope name, matching the Python package so
// Logfire's evaluation views recognize the spans.
const otelScope = "pydantic-evals"

// tracer returns the package tracer from the global provider. When no provider
// is configured this is a no-op tracer, so instrumentation costs nothing unless
// the user opts in via otel.SetTracerProvider.
func tracer() trace.Tracer {
	return otel.GetTracerProvider().Tracer(otelScope)
}

// startExperimentSpan opens the top-level span for an experiment run.
func startExperimentSpan(ctx context.Context, name, taskName, datasetName string, nCases, repeat int, metadata map[string]any) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String("name", name),
		attribute.String("task_name", taskName),
		attribute.String("dataset_name", datasetName),
		attribute.Int("n_cases", nCases),
		attribute.String("gen_ai.operation.name", "experiment"),
	}
	if repeat > 1 {
		attrs = append(attrs, attribute.Int("logfire.experiment.repeat", repeat))
	}
	if metadata != nil {
		attrs = append(attrs, attribute.String("metadata", jsonString(metadata)))
	}
	return tracer().Start(ctx, "evaluate "+name, trace.WithAttributes(attrs...))
}

// setExperimentResultAttributes records the assertion pass rate on the
// experiment span, matching Logfire's experiment view.
func setExperimentResultAttributes(span trace.Span, assertionPassRate float64) {
	span.SetAttributes(attribute.Float64("assertion_pass_rate", assertionPassRate))
}

// startCaseSpan opens a per-case span as a child of the experiment span.
func startCaseSpan(ctx context.Context, taskName, caseName string, inputs, metadata, expectedOutput any, hasMeta, hasExp bool, sourceCaseName string) (context.Context, trace.Span) {
	attrs := []attribute.KeyValue{
		attribute.String("task_name", taskName),
		attribute.String("case_name", caseName),
		attribute.String("inputs", jsonString(inputs)),
	}
	if hasMeta {
		attrs = append(attrs, attribute.String("metadata", jsonString(metadata)))
	}
	if hasExp {
		attrs = append(attrs, attribute.String("expected_output", jsonString(expectedOutput)))
	}
	if sourceCaseName != "" {
		attrs = append(attrs, attribute.String("logfire.experiment.source_case_name", sourceCaseName))
	}
	return tracer().Start(ctx, "case: "+caseName, trace.WithAttributes(attrs...))
}

// setCaseResultAttributes records the task output, durations, metrics and
// evaluation results on the case span, matching the Python case-span attributes.
func setCaseResultAttributes[I, O, M any](span trace.Span, rc *ReportCase[I, O, M]) {
	span.SetAttributes(
		attribute.String("output", jsonString(rc.Output)),
		attribute.Float64("task_duration", rc.TaskDuration.Seconds()),
		attribute.String("metrics", jsonString(rc.Metrics)),
		attribute.String("attributes", jsonString(rc.Attributes)),
		attribute.String("scores", jsonString(resultValues(rc.Scores))),
		attribute.String("labels", jsonString(resultValues(rc.Labels))),
		attribute.String("assertions", jsonString(resultValues(rc.Assertions))),
	)
}

// startTaskSpan opens the span wrapping the task execution itself.
func startTaskSpan(ctx context.Context, taskName string) (context.Context, trace.Span) {
	return tracer().Start(ctx, "execute "+taskName, trace.WithAttributes(
		attribute.String("task", taskName),
	))
}

// startEvaluatorSpan opens the span for a single evaluator run.
func startEvaluatorSpan(ctx context.Context, evaluatorName string) (context.Context, trace.Span) {
	return tracer().Start(ctx, "evaluator: "+evaluatorName, trace.WithAttributes(
		attribute.String("evaluator_name", evaluatorName),
	))
}

// recordSpanError marks a span as failed with the given error.
func recordSpanError(span trace.Span, err error) {
	span.RecordError(err)
	span.SetStatus(codes.Error, err.Error())
}

// resultValues projects an EvaluationResult map to a name->value map for span
// attributes (the scalar value is what the eval views read).
func resultValues(m map[string]EvaluationResult) map[string]any {
	out := make(map[string]any, len(m))
	for k, v := range m {
		out[k] = scalarToAny(v.Value)
	}
	return out
}

// scalarToAny unwraps a Scalar to its native Go value so span-attribute JSON
// keeps numbers as numbers and labels as strings (better for Logfire queries).
// Scalar is a sealed interface with exactly these four implementations; the
// trailing return is structurally required by Go but unreachable.
func scalarToAny(s Scalar) any {
	switch v := s.(type) {
	case Bool:
		return bool(v)
	case Int:
		return int(v)
	case Float:
		return float64(v)
	case Label:
		return string(v)
	}
	return nil // unreachable: Scalar is sealed
}
