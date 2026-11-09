package evals

// Case is a single row of a [Dataset].
//
// Each case represents one test scenario. Inputs is required; Name, Metadata,
// ExpectedOutput, and case-specific Evaluators are optional. Use [NewCase] with
// option functions to construct a case, which sets the Has* flags consistently.
type Case[I, O, M any] struct {
	// Name identifies the case in reports and for filtering. When empty, a
	// generic name ("Case N") is assigned during evaluation.
	Name string
	// Inputs to the task being evaluated.
	Inputs I
	// Metadata associated with the case, available to evaluators.
	Metadata M
	// HasMetadata reports whether Metadata was provided.
	HasMetadata bool
	// ExpectedOutput of the task, for comparison by evaluators.
	ExpectedOutput O
	// HasExpectedOutput reports whether ExpectedOutput was provided.
	HasExpectedOutput bool
	// Evaluators run only on this case, in addition to dataset-level evaluators.
	Evaluators []Evaluator[I, O, M]
}

// CaseOption configures a [Case] built with [NewCase].
type CaseOption[I, O, M any] func(*Case[I, O, M])

// WithCaseName sets the case name.
func WithCaseName[I, O, M any](name string) CaseOption[I, O, M] {
	return func(c *Case[I, O, M]) { c.Name = name }
}

// WithMetadata sets the case metadata and marks it as present.
func WithMetadata[I, O, M any](metadata M) CaseOption[I, O, M] {
	return func(c *Case[I, O, M]) {
		c.Metadata = metadata
		c.HasMetadata = true
	}
}

// WithExpectedOutput sets the case expected output and marks it as present.
func WithExpectedOutput[I, O, M any](expected O) CaseOption[I, O, M] {
	return func(c *Case[I, O, M]) {
		c.ExpectedOutput = expected
		c.HasExpectedOutput = true
	}
}

// WithCaseEvaluators appends case-specific evaluators.
func WithCaseEvaluators[I, O, M any](evaluators ...Evaluator[I, O, M]) CaseOption[I, O, M] {
	return func(c *Case[I, O, M]) { c.Evaluators = append(c.Evaluators, evaluators...) }
}

// NewCase creates a [Case] with the given inputs and options.
func NewCase[I, O, M any](inputs I, opts ...CaseOption[I, O, M]) Case[I, O, M] {
	c := Case[I, O, M]{Inputs: inputs}
	for _, opt := range opts {
		opt(&c)
	}
	return c
}
