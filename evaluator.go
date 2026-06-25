package evals

import (
	"context"
	"fmt"
)

// EvaluationReason is a [Scalar] result with an optional explanation.
type EvaluationReason struct {
	Value  Scalar
	Reason string
}

// EvaluatorOutput is the value an [Evaluator] may return from Evaluate.
//
// Implementations return one of:
//   - a [Scalar] (Bool, Int, Float, Label),
//   - an [EvaluationReason],
//   - a [ScalarOutput] / [ReasonOutput] mapping of names to either.
//
// Use the helper constructors [ScalarValue], [Reason], [ScalarOutput] and
// [ReasonOutput] to build outputs.
type EvaluatorOutput interface {
	// normalize converts the output to a mapping from result name to reason,
	// using scalarName as the key for a single unnamed scalar/reason result.
	normalize(scalarName string) (map[string]EvaluationReason, error)
}

// scalarOutput is a single unnamed scalar result.
type scalarOutput struct{ value Scalar }

// reasonOutput is a single unnamed scalar result with a reason.
type reasonOutput struct{ reason EvaluationReason }

// ScalarMapOutput is a mapping of result names to scalar values.
type ScalarMapOutput map[string]Scalar

// ReasonMapOutput is a mapping of result names to [EvaluationReason] values.
type ReasonMapOutput map[string]EvaluationReason

// ScalarValue wraps a single [Scalar] as an [EvaluatorOutput].
func ScalarValue(s Scalar) EvaluatorOutput { return scalarOutput{value: s} }

// Reason wraps a single [Scalar] and explanation as an [EvaluatorOutput].
func Reason(s Scalar, reason string) EvaluatorOutput {
	return reasonOutput{reason: EvaluationReason{Value: s, Reason: reason}}
}

func (o scalarOutput) normalize(scalarName string) (map[string]EvaluationReason, error) {
	if err := validateScalar(o.value); err != nil {
		return nil, err
	}
	return map[string]EvaluationReason{scalarName: {Value: o.value}}, nil
}

func (o reasonOutput) normalize(scalarName string) (map[string]EvaluationReason, error) {
	if err := validateScalar(o.reason.Value); err != nil {
		return nil, err
	}
	return map[string]EvaluationReason{scalarName: o.reason}, nil
}

func (m ScalarMapOutput) normalize(string) (map[string]EvaluationReason, error) {
	out := make(map[string]EvaluationReason, len(m))
	for name, s := range m {
		if err := validateScalar(s); err != nil {
			return nil, err
		}
		out[name] = EvaluationReason{Value: s}
	}
	return out, nil
}

func (m ReasonMapOutput) normalize(string) (map[string]EvaluationReason, error) {
	out := make(map[string]EvaluationReason, len(m))
	for name, r := range m {
		if err := validateScalar(r.Value); err != nil {
			return nil, err
		}
		out[name] = r
	}
	return out, nil
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
// Implementations must provide Evaluate. The default name used in reports is the
// evaluator's [EvaluatorSpec] name; embed [BaseEvaluator] to satisfy the
// auxiliary methods, or implement [NamedEvaluator] / [VersionedEvaluator] to
// customize the report name and version.
type Evaluator[I, O, M any] interface {
	// Evaluate scores the task output described by ctx.
	Evaluate(ctx context.Context, ec *EvaluatorContext[I, O, M]) (EvaluatorOutput, error)
	// Spec returns the serializable specification of this evaluator.
	Spec() EvaluatorSpec
}

// NamedEvaluator is an optional interface an [Evaluator] may implement to
// override the name used for its results in reports. When not implemented, the
// [EvaluatorSpec] name is used.
type NamedEvaluator interface {
	DefaultEvaluationName() string
}

// VersionedEvaluator is an optional interface an [Evaluator] may implement to
// tag its results with a version string for downstream filtering.
type VersionedEvaluator interface {
	EvaluatorVersion() string
}

// defaultEvaluationName returns the report name for an evaluator, honoring
// [NamedEvaluator] when implemented.
func defaultEvaluationName[I, O, M any](e Evaluator[I, O, M]) string {
	if n, ok := any(e).(NamedEvaluator); ok {
		if name := n.DefaultEvaluationName(); name != "" {
			return name
		}
	}
	return e.Spec().Name
}

// evaluatorVersion returns the version tag for an evaluator, honoring
// [VersionedEvaluator] when implemented.
func evaluatorVersion[I, O, M any](e Evaluator[I, O, M]) string {
	if v, ok := any(e).(VersionedEvaluator); ok {
		return v.EvaluatorVersion()
	}
	return ""
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
	source := e.Spec()

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
