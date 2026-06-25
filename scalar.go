package evals

import (
	"fmt"
	"math"
)

// Scalar is the most primitive output allowed from an [Evaluator].
//
// The concrete kinds are [Bool], [Int], [Float] and [Label] (a string). A [Bool]
// is treated as an assertion, an [Int] or finite [Float] as a score, and a
// [Label] as a categorical label. This mirrors the Python `EvaluationScalar`
// union of `bool | int | float | str`.
type Scalar interface {
	isScalar()
	// String returns the human-readable form used when rendering reports.
	String() string
}

// Bool is a [Scalar] assertion result.
type Bool bool

// Int is a [Scalar] score result.
type Int int

// Float is a [Scalar] score result. Inf and NaN are not valid scores and are
// rejected when an evaluator's output is normalized.
type Float float64

// Label is a [Scalar] categorical result.
type Label string

func (Bool) isScalar()  {}
func (Int) isScalar()   {}
func (Float) isScalar() {}
func (Label) isScalar() {}

func (b Bool) String() string {
	if b {
		return "True"
	}
	return "False"
}

func (i Int) String() string   { return fmt.Sprintf("%d", int(i)) }
func (f Float) String() string { return fmt.Sprintf("%v", float64(f)) }
func (l Label) String() string { return string(l) }

// validateScalar rejects non-finite floats, matching pydantic's
// `Field(allow_inf_nan=False)` constraint on the float arm of the scalar union.
func validateScalar(s Scalar) error {
	if f, ok := s.(Float); ok {
		if math.IsInf(float64(f), 0) || math.IsNaN(float64(f)) {
			return fmt.Errorf("evaluator returned a non-finite float score: %v", float64(f))
		}
	}
	return nil
}
