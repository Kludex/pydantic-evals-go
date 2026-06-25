package evals

// Output is the value an [Evaluator] returns from Evaluate.
//
// Build one with [Score], [Assertion] or [Category] for a single result, or
// [Named] for several named results from one evaluator. A bare [Score] becomes a
// numeric score in the report, [Assertion] a boolean assertion, and [Category] a
// string label.
type Output interface {
	// normalize converts the output to a name->result mapping, using scalarName
	// as the key for a single unnamed result.
	normalize(scalarName string) (map[string]EvaluationReason, error)
}

// single is one unnamed result with an optional reason.
type single struct {
	value  Scalar
	reason string
}

// WithReason attaches an explanation to a single-result [Output]. It has no
// effect on [Named] outputs (give each [Named] entry its own reason instead).
func (s single) WithReason(reason string) single {
	s.reason = reason
	return s
}

func (s single) normalize(scalarName string) (map[string]EvaluationReason, error) {
	if err := validateScalar(s.value); err != nil {
		return nil, err
	}
	return map[string]EvaluationReason{scalarName: {Value: s.value, Reason: s.reason}}, nil
}

// Score returns a numeric score [Output]. Use [single.WithReason] to explain it.
func Score(value float64) single { return single{value: Float(value)} }

// ScoreInt returns an integer score [Output], rendered without decimals.
func ScoreInt(value int) single { return single{value: Int(value)} }

// Assertion returns a boolean assertion [Output] (pass/fail).
func Assertion(passed bool) single { return single{value: Bool(passed)} }

// Category returns a categorical label [Output].
func Category(label string) single { return single{value: Label(label)} }

// named is a set of named results from a single evaluator.
type named map[string]EvaluationReason

func (n named) normalize(string) (map[string]EvaluationReason, error) {
	for _, r := range n {
		if err := validateScalar(r.Value); err != nil {
			return nil, err
		}
	}
	return map[string]EvaluationReason(n), nil
}

// Named groups several results from one evaluator under explicit names. Pass
// alternating name and [Output] arguments; each [Output] must be a single result
// (from [Score]/[Assertion]/[Category]), not another [Named].
//
//	return evals.Named(
//	    "length", evals.Score(0.8).WithReason("close"),
//	    "sentiment", evals.Category("neutral"),
//	), nil
//
// Named panics if given an odd number of arguments or a non-string name, which
// are programming errors rather than runtime conditions.
func Named(pairs ...any) Output {
	if len(pairs)%2 != 0 {
		panic("evals.Named: expected an even number of name/output arguments")
	}
	out := named{}
	for i := 0; i < len(pairs); i += 2 {
		name, ok := pairs[i].(string)
		if !ok {
			panic("evals.Named: argument names must be strings")
		}
		s, ok := pairs[i+1].(single)
		if !ok {
			panic("evals.Named: argument values must come from Score/ScoreInt/Assertion/Category")
		}
		out[name] = EvaluationReason{Value: s.value, Reason: s.reason}
	}
	return out
}

// NoResult returns an [Output] that records nothing, for evaluators that
// conditionally produce no result (e.g. when there is no expected output).
func NoResult() Output { return named{} }
