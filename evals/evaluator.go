package evals

import (
	"context"
	"fmt"
	"reflect"
)

// EvaluationReason is a [Scalar] result with an optional explanation.
type EvaluationReason struct {
	Value  Scalar
	Reason string
}

// EvaluationResult is the detail of a single named evaluation result.
type EvaluationResult struct {
	Name             string
	Value            Scalar
	Reason           string
	Source           EvaluatorSpec
	EvaluatorVersion string
}

// EvaluatorFailure represents an error raised while running an [Evaluator].
type EvaluatorFailure struct {
	Name             string
	ErrorMessage     string
	Source           EvaluatorSpec
	EvaluatorVersion string
	ErrorType        string
}

// Evaluator assesses the result of running a task against a single [Case].
//
// Implementations only need Evaluate. The name used for an evaluator's results
// in reports defaults to its Go type name; implement [NamedEvaluator] to
// override it, [SpecEvaluator] to make the evaluator serializable to YAML/JSON,
// and [VersionedEvaluator] to tag its results with a version.
type Evaluator[I, O, M any] interface {
	// Evaluate scores the task output described by ec, returning an [Output]
	// built with [Score], [Assertion], [Category] or [Named].
	Evaluate(ctx context.Context, ec *EvaluatorContext[I, O, M]) (Output, error)
}

// SpecEvaluator is an optional interface an [Evaluator] may implement so it can
// be serialized to and reconstructed from a [Dataset] file. Evaluators that are
// only used in-memory do not need it.
type SpecEvaluator interface {
	Spec() EvaluatorSpec
}

// NamedEvaluator is an optional interface an [Evaluator] may implement to
// override the name used for its results in reports.
type NamedEvaluator interface {
	EvaluationName() string
}

// VersionedEvaluator is an optional interface an [Evaluator] may implement to
// tag its results with a version string for downstream filtering.
type VersionedEvaluator interface {
	EvaluatorVersion() string
}

// evaluatorSpec returns the spec for an evaluator, honoring [SpecEvaluator] when
// implemented and otherwise synthesizing a name-only spec from the type name.
func evaluatorSpec[I, O, M any](e Evaluator[I, O, M]) EvaluatorSpec {
	if s, ok := any(e).(SpecEvaluator); ok {
		return s.Spec()
	}
	return EvaluatorSpec{Name: evaluatorTypeName(e)}
}

// defaultEvaluationName returns the report name for an evaluator: its
// [NamedEvaluator] name when implemented, otherwise its spec name.
func defaultEvaluationName[I, O, M any](e Evaluator[I, O, M]) string {
	if n, ok := any(e).(NamedEvaluator); ok {
		if name := n.EvaluationName(); name != "" {
			return name
		}
	}
	return evaluatorSpec(e).Name
}

// evaluatorVersion returns the version tag for an evaluator, honoring
// [VersionedEvaluator] when implemented.
func evaluatorVersion[I, O, M any](e Evaluator[I, O, M]) string {
	if v, ok := any(e).(VersionedEvaluator); ok {
		return v.EvaluatorVersion()
	}
	return ""
}

// evaluatorTypeName returns the unqualified Go type name of an evaluator value,
// used as its default report/spec name.
func evaluatorTypeName(e any) string {
	t := reflect.TypeOf(e)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}

// runEvaluator runs a single evaluator and normalizes its output into results.
// A returned EvaluatorFailure (non-nil) indicates the evaluator errored.
func runEvaluator[I, O, M any](
	ctx context.Context,
	e Evaluator[I, O, M],
	ec *EvaluatorContext[I, O, M],
) ([]EvaluationResult, *EvaluatorFailure) {
	name := defaultEvaluationName(e)
	version := evaluatorVersion(e)
	source := evaluatorSpec(e)

	fail := func(err error) *EvaluatorFailure {
		return &EvaluatorFailure{
			Name:             name,
			ErrorMessage:     err.Error(),
			Source:           source,
			EvaluatorVersion: version,
			ErrorType:        errorType(err),
		}
	}

	raw, err := e.Evaluate(ctx, ec)
	if err != nil {
		return nil, fail(err)
	}
	if raw == nil {
		return nil, fail(fmt.Errorf("evaluator %q returned a nil output", name))
	}
	results, err := raw.normalize(name)
	if err != nil {
		return nil, fail(err)
	}

	details := make([]EvaluationResult, 0, len(results))
	for resultName, r := range results {
		details = append(details, EvaluationResult{
			Name:             resultName,
			Value:            r.Value,
			Reason:           r.Reason,
			Source:           source,
			EvaluatorVersion: version,
		})
	}
	return details, nil
}
