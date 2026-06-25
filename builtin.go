package evals

import (
	"context"
	"fmt"
	"reflect"
	"strings"
	"time"
)

// Equals checks whether the output exactly equals the provided value.
type Equals[I, O, M any] struct {
	// Value to compare the output against.
	Value any
	// EvaluationName overrides the result name in reports.
	EvaluationName string
}

func (e Equals[I, O, M]) Evaluate(_ context.Context, ec *EvaluatorContext[I, O, M]) (EvaluatorOutput, error) {
	return ScalarValue(Bool(deepEqual(ec.Output, e.Value))), nil
}

func (e Equals[I, O, M]) Spec() EvaluatorSpec {
	return specWithName("Equals", e.EvaluationName, map[string]any{"value": e.Value})
}

func (e Equals[I, O, M]) DefaultEvaluationName() string { return nameOr(e.EvaluationName, "Equals") }

// EqualsExpected checks whether the output exactly equals the case's expected
// output. If the case has no expected output, it yields no result.
type EqualsExpected[I, O, M any] struct {
	EvaluationName string
}

func (e EqualsExpected[I, O, M]) Evaluate(_ context.Context, ec *EvaluatorContext[I, O, M]) (EvaluatorOutput, error) {
	if !ec.HasExpectedOutput {
		return ScalarMapOutput{}, nil
	}
	return ScalarValue(Bool(deepEqual(ec.Output, ec.ExpectedOutput))), nil
}

func (e EqualsExpected[I, O, M]) Spec() EvaluatorSpec {
	return specWithName("EqualsExpected", e.EvaluationName, nil)
}

func (e EqualsExpected[I, O, M]) DefaultEvaluationName() string {
	return nameOr(e.EvaluationName, "EqualsExpected")
}

// Contains checks whether the output contains the provided value.
//
// For strings it checks substring containment; for slices/arrays, element
// membership; for maps, that every key/value in Value (when Value is a map) is
// present, or that Value is a key. CaseSensitive applies only to string checks.
type Contains[I, O, M any] struct {
	Value          any
	CaseSensitive  bool
	AsStrings      bool
	EvaluationName string
}

func (c Contains[I, O, M]) Evaluate(_ context.Context, ec *EvaluatorContext[I, O, M]) (EvaluatorOutput, error) {
	failureReason := containsCheck(ec.Output, c.Value, c.AsStrings, c.CaseSensitive)
	return Reason(Bool(failureReason == ""), failureReason), nil
}

func (c Contains[I, O, M]) Spec() EvaluatorSpec {
	kwargs := map[string]any{"value": c.Value}
	if c.CaseSensitive {
		kwargs["case_sensitive"] = c.CaseSensitive
	}
	if c.AsStrings {
		kwargs["as_strings"] = c.AsStrings
	}
	if c.EvaluationName != "" {
		kwargs["evaluation_name"] = c.EvaluationName
	}
	return EvaluatorSpec{Name: "Contains", Kwargs: kwargs}
}

func (c Contains[I, O, M]) DefaultEvaluationName() string {
	return nameOr(c.EvaluationName, "Contains")
}

// IsInstance checks whether the output's runtime type name matches TypeName.
//
// The match is against the unqualified type name (e.g. "string", "int", or a
// struct's name), making it useful when O is an interface type like `any`.
type IsInstance[I, O, M any] struct {
	TypeName       string
	EvaluationName string
}

func (e IsInstance[I, O, M]) Evaluate(_ context.Context, ec *EvaluatorContext[I, O, M]) (EvaluatorOutput, error) {
	actual := typeName(ec.Output)
	if actual == e.TypeName {
		return ScalarValue(Bool(true)), nil
	}
	return Reason(Bool(false), fmt.Sprintf("output is of type %s", actual)), nil
}

func (e IsInstance[I, O, M]) Spec() EvaluatorSpec {
	if e.EvaluationName != "" {
		return EvaluatorSpec{Name: "IsInstance", Kwargs: map[string]any{
			"type_name": e.TypeName, "evaluation_name": e.EvaluationName,
		}}
	}
	return NewSpecArg("IsInstance", e.TypeName)
}

func (e IsInstance[I, O, M]) DefaultEvaluationName() string {
	return nameOr(e.EvaluationName, "IsInstance")
}

// MaxDuration checks whether the task ran in at most Max.
type MaxDuration[I, O, M any] struct {
	Max time.Duration
}

func (e MaxDuration[I, O, M]) Evaluate(_ context.Context, ec *EvaluatorContext[I, O, M]) (EvaluatorOutput, error) {
	return ScalarValue(Bool(ec.Duration <= e.Max)), nil
}

func (e MaxDuration[I, O, M]) Spec() EvaluatorSpec {
	return NewSpecArg("MaxDuration", e.Max.Seconds())
}

// specWithName builds a spec for an evaluator whose only optional field is
// evaluation_name, plus a fixed set of other kwargs.
func specWithName(name, evaluationName string, kwargs map[string]any) EvaluatorSpec {
	if kwargs == nil {
		kwargs = map[string]any{}
	}
	if evaluationName != "" {
		kwargs["evaluation_name"] = evaluationName
	}
	if len(kwargs) == 0 {
		return EvaluatorSpec{Name: name}
	}
	return EvaluatorSpec{Name: name, Kwargs: kwargs}
}

func nameOr(override, fallback string) string {
	if override != "" {
		return override
	}
	return fallback
}

// typeName returns the unqualified runtime type name of v, mapping nil to "nil".
func typeName(v any) string {
	if v == nil {
		return "nil"
	}
	t := reflect.TypeOf(v)
	if t.Name() != "" {
		return t.Name()
	}
	return t.String()
}

// deepEqual compares two values, treating numeric Scalars and plain numbers
// consistently. It falls back to reflect.DeepEqual.
func deepEqual(a, b any) bool {
	return reflect.DeepEqual(a, b)
}

// containsCheck returns an empty string if output contains value, otherwise a
// human-readable failure reason.
func containsCheck(output, value any, asStrings, caseSensitive bool) string {
	_, outIsStr := output.(string)
	_, valIsStr := value.(string)
	if asStrings || (outIsStr && valIsStr) {
		outStr := fmt.Sprintf("%v", output)
		expStr := fmt.Sprintf("%v", value)
		if !caseSensitive {
			outStr = strings.ToLower(outStr)
			expStr = strings.ToLower(expStr)
		}
		if !strings.Contains(outStr, expStr) {
			return fmt.Sprintf("Output string %s does not contain expected string %s",
				truncatedRepr(outStr, 100), truncatedRepr(expStr, 100))
		}
		return ""
	}

	rv := reflect.ValueOf(output)
	switch rv.Kind() {
	case reflect.Map:
		if vm := reflect.ValueOf(value); vm.Kind() == reflect.Map {
			for _, k := range vm.MapKeys() {
				ov := rv.MapIndex(k)
				if !ov.IsValid() {
					return fmt.Sprintf("Output does not contain expected key %s", truncatedRepr(k.Interface(), 30))
				}
				if !reflect.DeepEqual(ov.Interface(), vm.MapIndex(k).Interface()) {
					return fmt.Sprintf("Output has different value for key %s: %s != %s",
						truncatedRepr(k.Interface(), 30),
						truncatedRepr(ov.Interface(), 100),
						truncatedRepr(vm.MapIndex(k).Interface(), 100))
				}
			}
			return ""
		}
		if rv.MapIndex(reflect.ValueOf(value)).IsValid() {
			return ""
		}
		return fmt.Sprintf("Output %s does not contain provided value as a key", truncatedRepr(output, 200))
	case reflect.Slice, reflect.Array:
		for i := 0; i < rv.Len(); i++ {
			if reflect.DeepEqual(rv.Index(i).Interface(), value) {
				return ""
			}
		}
		return fmt.Sprintf("Output %s does not contain provided value", truncatedRepr(output, 200))
	default:
		return fmt.Sprintf("Containment check failed: cannot check containment in %s", typeName(output))
	}
}

func truncatedRepr(value any, maxLen int) string {
	s := fmt.Sprintf("%#v", value)
	if len(s) > maxLen {
		s = s[:maxLen/2] + "..." + s[len(s)-maxLen/2:]
	}
	return s
}
