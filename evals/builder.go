package evals

import "time"

// Suite is a typed entry point that captures the inputs, output and metadata
// types once so you don't repeat them at every call site. Obtain one with [For]:
//
//	s := evals.For[string, string, any]()
//	c := s.Case("hi").Name("greeting").Expect("hello")
//	ds, _ := s.Dataset("quiz", c).With(s.IsInstance("string"))
//
// Suite is a zero-size value; create as many as you like.
type Suite[I, O, M any] struct{}

// For returns a [Suite] bound to the given inputs (I), output (O) and metadata
// (M) types.
func For[I, O, M any]() Suite[I, O, M] { return Suite[I, O, M]{} }

// Case starts building a [Case] with the given inputs. Chain Name/Expect/Meta/
// Eval to configure it; the [CaseBuilder] is usable directly as a [Case]
// argument or via [CaseBuilder.Build].
func (Suite[I, O, M]) Case(inputs I) *CaseBuilder[I, O, M] {
	return &CaseBuilder[I, O, M]{c: Case[I, O, M]{Inputs: inputs}}
}

// Dataset creates a [Dataset] from the given case builders. Add dataset-level
// evaluators with [Dataset.With]. It panics if two cases share a name, which is
// a test-definition error; use [NewDataset] if you need to handle that as a
// value.
func (Suite[I, O, M]) Dataset(name string, cases ...*CaseBuilder[I, O, M]) *Dataset[I, O, M] {
	built := make([]Case[I, O, M], len(cases))
	for i, cb := range cases {
		built[i] = cb.Build()
	}
	ds, err := NewDataset(name, built)
	if err != nil {
		panic(err)
	}
	return ds
}

// Equals builds an [Equals] evaluator bound to this suite's types.
func (Suite[I, O, M]) Equals(value O) Equals[I, O, M] { return Equals[I, O, M]{Value: value} }

// EqualsExpected builds an [EqualsExpected] evaluator bound to this suite's types.
func (Suite[I, O, M]) EqualsExpected() EqualsExpected[I, O, M] { return EqualsExpected[I, O, M]{} }

// Contains builds a [Contains] evaluator bound to this suite's types.
func (Suite[I, O, M]) Contains(value any) Contains[I, O, M] { return Contains[I, O, M]{Value: value} }

// IsInstance builds an [IsInstance] evaluator bound to this suite's types.
func (Suite[I, O, M]) IsInstance(typeName string) IsInstance[I, O, M] {
	return IsInstance[I, O, M]{TypeName: typeName}
}

// MaxDuration builds a [MaxDuration] evaluator bound to this suite's types.
func (Suite[I, O, M]) MaxDuration(max time.Duration) MaxDuration[I, O, M] {
	return MaxDuration[I, O, M]{Max: max}
}

// CaseBuilder is a fluent builder for a [Case], returned by [Suite.Case].
type CaseBuilder[I, O, M any] struct {
	c Case[I, O, M]
}

// Name sets the case name.
func (b *CaseBuilder[I, O, M]) Name(name string) *CaseBuilder[I, O, M] {
	b.c.Name = name
	return b
}

// Expect sets the expected output.
func (b *CaseBuilder[I, O, M]) Expect(expected O) *CaseBuilder[I, O, M] {
	b.c.ExpectedOutput = expected
	b.c.HasExpectedOutput = true
	return b
}

// Meta sets the case metadata.
func (b *CaseBuilder[I, O, M]) Meta(metadata M) *CaseBuilder[I, O, M] {
	b.c.Metadata = metadata
	b.c.HasMetadata = true
	return b
}

// Eval appends case-specific evaluators.
func (b *CaseBuilder[I, O, M]) Eval(evaluators ...Evaluator[I, O, M]) *CaseBuilder[I, O, M] {
	b.c.Evaluators = append(b.c.Evaluators, evaluators...)
	return b
}

// Build returns the configured [Case].
func (b *CaseBuilder[I, O, M]) Build() Case[I, O, M] { return b.c }

// With appends dataset-level evaluators and returns the dataset for chaining.
func (d *Dataset[I, O, M]) With(evaluators ...Evaluator[I, O, M]) *Dataset[I, O, M] {
	d.Evaluators = append(d.Evaluators, evaluators...)
	return d
}
