package evals

import (
	"context"
	"reflect"
	"testing"
	"time"
)

// runBuiltin evaluates a single evaluator against one case whose task returns the
// given output, and returns the resulting ReportCase. The output type O is `any`
// so a single evaluator can be exercised with arbitrary output shapes.
func runBuiltin[M any](t *testing.T, eval Evaluator[int, any, M], c Case[int, any, M], output any) ReportCase[int, any, M] {
	t.Helper()
	c.Evaluators = append(c.Evaluators, eval)
	ds, err := NewDataset("ds", []Case[int, any, M]{c})
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}
	task := func(_ context.Context, _ int) (any, error) { return output, nil }
	report, err := ds.Evaluate(context.Background(), task)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(report.Failures) != 0 {
		t.Fatalf("unexpected failures: %+v", report.Failures)
	}
	if len(report.Cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(report.Cases))
	}
	rc := report.Cases[0]
	if len(rc.EvaluatorFailures) != 0 {
		t.Fatalf("unexpected evaluator failures: %+v", rc.EvaluatorFailures)
	}
	return rc
}

func assertion(t *testing.T, rc ReportCase[int, any, any], name string) EvaluationResult {
	t.Helper()
	res, ok := rc.Assertions[name]
	if !ok {
		t.Fatalf("assertion %q not found; have %v", name, keysOf(rc.Assertions))
	}
	return res
}

func keysOf[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

func boolValue(t *testing.T, res EvaluationResult) bool {
	t.Helper()
	b, ok := res.Value.(Bool)
	if !ok {
		t.Fatalf("expected Bool value, got %T", res.Value)
	}
	return bool(b)
}

type namedType struct{ X int }

func TestEquals(t *testing.T) {
	t.Run("match", func(t *testing.T) {
		rc := runBuiltin[any](t, Equals[int, any, any]{Value: "hello"}, NewCase[int, any, any](1), "hello")
		res := assertion(t, rc, "Equals")
		if !boolValue(t, res) {
			t.Fatalf("expected match")
		}
		if res.Reason != "" {
			t.Fatalf("expected empty reason, got %q", res.Reason)
		}
	})

	t.Run("mismatch", func(t *testing.T) {
		rc := runBuiltin[any](t, Equals[int, any, any]{Value: "hello"}, NewCase[int, any, any](1), "world")
		if boolValue(t, assertion(t, rc, "Equals")) {
			t.Fatalf("expected mismatch")
		}
	})

	t.Run("with evaluation name", func(t *testing.T) {
		rc := runBuiltin[any](t, Equals[int, any, any]{Value: 5, EvaluationName: "eq_five"}, NewCase[int, any, any](1), 5)
		if !boolValue(t, assertion(t, rc, "eq_five")) {
			t.Fatalf("expected match under custom name")
		}
		if _, ok := rc.Assertions["Equals"]; ok {
			t.Fatalf("did not expect default name when EvaluationName is set")
		}
	})

	t.Run("spec and default name", func(t *testing.T) {
		e := Equals[int, any, any]{Value: 5}
		if got, want := e.Spec(), (EvaluatorSpec{Name: "Equals", Kwargs: map[string]any{"value": 5}}); !reflect.DeepEqual(got, want) {
			t.Fatalf("Spec() = %+v, want %+v", got, want)
		}
		if got := e.DefaultEvaluationName(); got != "Equals" {
			t.Fatalf("DefaultEvaluationName() = %q, want Equals", got)
		}

		named := Equals[int, any, any]{Value: 5, EvaluationName: "eq"}
		want := EvaluatorSpec{Name: "Equals", Kwargs: map[string]any{"value": 5, "evaluation_name": "eq"}}
		if got := named.Spec(); !reflect.DeepEqual(got, want) {
			t.Fatalf("Spec() = %+v, want %+v", got, want)
		}
		if got := named.DefaultEvaluationName(); got != "eq" {
			t.Fatalf("DefaultEvaluationName() = %q, want eq", got)
		}
	})
}

func TestEqualsExpected(t *testing.T) {
	t.Run("match", func(t *testing.T) {
		c := NewCase[int, any, any](1, WithExpectedOutput[int, any, any](any("hi")))
		rc := runBuiltin[any](t, EqualsExpected[int, any, any]{}, c, "hi")
		if !boolValue(t, assertion(t, rc, "EqualsExpected")) {
			t.Fatalf("expected match")
		}
	})

	t.Run("mismatch", func(t *testing.T) {
		c := NewCase[int, any, any](1, WithExpectedOutput[int, any, any](any("hi")))
		rc := runBuiltin[any](t, EqualsExpected[int, any, any]{}, c, "bye")
		if boolValue(t, assertion(t, rc, "EqualsExpected")) {
			t.Fatalf("expected mismatch")
		}
	})

	t.Run("no expected output yields no result", func(t *testing.T) {
		rc := runBuiltin[any](t, EqualsExpected[int, any, any]{}, NewCase[int, any, any](1), "anything")
		if len(rc.Assertions) != 0 || len(rc.Scores) != 0 || len(rc.Labels) != 0 {
			t.Fatalf("expected no results, got assertions=%v scores=%v labels=%v", rc.Assertions, rc.Scores, rc.Labels)
		}
	})

	t.Run("spec and default name", func(t *testing.T) {
		e := EqualsExpected[int, any, any]{}
		if got := e.Spec(); !reflect.DeepEqual(got, EvaluatorSpec{Name: "EqualsExpected"}) {
			t.Fatalf("Spec() = %+v", got)
		}
		if got := e.DefaultEvaluationName(); got != "EqualsExpected" {
			t.Fatalf("DefaultEvaluationName() = %q", got)
		}

		named := EqualsExpected[int, any, any]{EvaluationName: "exp"}
		want := EvaluatorSpec{Name: "EqualsExpected", Kwargs: map[string]any{"evaluation_name": "exp"}}
		if got := named.Spec(); !reflect.DeepEqual(got, want) {
			t.Fatalf("Spec() = %+v, want %+v", got, want)
		}
		if got := named.DefaultEvaluationName(); got != "exp" {
			t.Fatalf("DefaultEvaluationName() = %q", got)
		}
	})
}

func TestContainsString(t *testing.T) {
	t.Run("substring match", func(t *testing.T) {
		rc := runBuiltin[any](t, Contains[int, any, any]{Value: "ell"}, NewCase[int, any, any](1), "hello")
		res := assertion(t, rc, "Contains")
		if !boolValue(t, res) {
			t.Fatalf("expected match")
		}
		if res.Reason != "" {
			t.Fatalf("expected empty reason, got %q", res.Reason)
		}
	})

	t.Run("substring mismatch reason", func(t *testing.T) {
		rc := runBuiltin[any](t, Contains[int, any, any]{Value: "xyz"}, NewCase[int, any, any](1), "hello world")
		res := assertion(t, rc, "Contains")
		if boolValue(t, res) {
			t.Fatalf("expected mismatch")
		}
		want := `Output string "hello world" does not contain expected string "xyz"`
		if res.Reason != want {
			t.Fatalf("reason = %q, want %q", res.Reason, want)
		}
	})

	t.Run("case insensitive default match", func(t *testing.T) {
		rc := runBuiltin[any](t, Contains[int, any, any]{Value: "WORLD"}, NewCase[int, any, any](1), "hello world")
		if !boolValue(t, assertion(t, rc, "Contains")) {
			t.Fatalf("expected case-insensitive match")
		}
	})

	t.Run("case sensitive mismatch", func(t *testing.T) {
		rc := runBuiltin[any](t, Contains[int, any, any]{Value: "WORLD", CaseSensitive: true}, NewCase[int, any, any](1), "hello world")
		res := assertion(t, rc, "Contains")
		if boolValue(t, res) {
			t.Fatalf("expected case-sensitive mismatch")
		}
		want := `Output string "hello world" does not contain expected string "WORLD"`
		if res.Reason != want {
			t.Fatalf("reason = %q, want %q", res.Reason, want)
		}
	})

	t.Run("as strings forces string compare of non-strings", func(t *testing.T) {
		rc := runBuiltin[any](t, Contains[int, any, any]{Value: 23, AsStrings: true}, NewCase[int, any, any](1), 12345)
		if !boolValue(t, assertion(t, rc, "Contains")) {
			t.Fatalf("expected '23' to be a substring of '12345'")
		}
	})

	t.Run("as strings mismatch reason", func(t *testing.T) {
		rc := runBuiltin[any](t, Contains[int, any, any]{Value: 99, AsStrings: true}, NewCase[int, any, any](1), 12345)
		res := assertion(t, rc, "Contains")
		if boolValue(t, res) {
			t.Fatalf("expected mismatch")
		}
		want := `Output string "12345" does not contain expected string "99"`
		if res.Reason != want {
			t.Fatalf("reason = %q, want %q", res.Reason, want)
		}
	})

	t.Run("long output truncates in reason", func(t *testing.T) {
		long := ""
		for i := 0; i < 120; i++ {
			long += "a"
		}
		rc := runBuiltin[any](t, Contains[int, any, any]{Value: "z"}, NewCase[int, any, any](1), long)
		res := assertion(t, rc, "Contains")
		if boolValue(t, res) {
			t.Fatalf("expected mismatch")
		}
		want := `Output string "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa...aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa" does not contain expected string "z"`
		if res.Reason != want {
			t.Fatalf("reason = %q, want %q", res.Reason, want)
		}
	})
}

func TestContainsSlice(t *testing.T) {
	t.Run("membership match", func(t *testing.T) {
		rc := runBuiltin[any](t, Contains[int, any, any]{Value: 2}, NewCase[int, any, any](1), []int{1, 2, 3})
		res := assertion(t, rc, "Contains")
		if !boolValue(t, res) {
			t.Fatalf("expected membership")
		}
		if res.Reason != "" {
			t.Fatalf("expected empty reason, got %q", res.Reason)
		}
	})

	t.Run("membership mismatch reason", func(t *testing.T) {
		rc := runBuiltin[any](t, Contains[int, any, any]{Value: 9}, NewCase[int, any, any](1), []int{1, 2, 3})
		res := assertion(t, rc, "Contains")
		if boolValue(t, res) {
			t.Fatalf("expected mismatch")
		}
		want := `Output []int{1, 2, 3} does not contain provided value`
		if res.Reason != want {
			t.Fatalf("reason = %q, want %q", res.Reason, want)
		}
	})
}

func TestContainsMap(t *testing.T) {
	t.Run("map contains map match", func(t *testing.T) {
		out := map[string]any{"a": 1, "b": 2}
		rc := runBuiltin[any](t, Contains[int, any, any]{Value: map[string]any{"a": 1}}, NewCase[int, any, any](1), out)
		res := assertion(t, rc, "Contains")
		if !boolValue(t, res) {
			t.Fatalf("expected submap match, reason=%q", res.Reason)
		}
	})

	t.Run("map missing key reason", func(t *testing.T) {
		out := map[string]any{"a": 1}
		rc := runBuiltin[any](t, Contains[int, any, any]{Value: map[string]any{"missing": 1}}, NewCase[int, any, any](1), out)
		res := assertion(t, rc, "Contains")
		if boolValue(t, res) {
			t.Fatalf("expected mismatch")
		}
		want := `Output does not contain expected key "missing"`
		if res.Reason != want {
			t.Fatalf("reason = %q, want %q", res.Reason, want)
		}
	})

	t.Run("map different value reason", func(t *testing.T) {
		out := map[string]any{"a": 1}
		rc := runBuiltin[any](t, Contains[int, any, any]{Value: map[string]any{"a": 2}}, NewCase[int, any, any](1), out)
		res := assertion(t, rc, "Contains")
		if boolValue(t, res) {
			t.Fatalf("expected mismatch")
		}
		want := `Output has different value for key "a": 1 != 2`
		if res.Reason != want {
			t.Fatalf("reason = %q, want %q", res.Reason, want)
		}
	})

	t.Run("map contains key true", func(t *testing.T) {
		out := map[string]any{"a": 1, "b": 2}
		rc := runBuiltin[any](t, Contains[int, any, any]{Value: "a"}, NewCase[int, any, any](1), out)
		if !boolValue(t, assertion(t, rc, "Contains")) {
			t.Fatalf("expected key to be present")
		}
	})

	t.Run("map contains key false reason", func(t *testing.T) {
		out := map[string]any{"a": 1}
		rc := runBuiltin[any](t, Contains[int, any, any]{Value: "z"}, NewCase[int, any, any](1), out)
		res := assertion(t, rc, "Contains")
		if boolValue(t, res) {
			t.Fatalf("expected key to be missing")
		}
		want := `Output map[string]interface {}{"a":1} does not contain provided value as a key`
		if res.Reason != want {
			t.Fatalf("reason = %q, want %q", res.Reason, want)
		}
	})
}

func TestContainsNonContainerFailure(t *testing.T) {
	t.Run("named type", func(t *testing.T) {
		rc := runBuiltin[any](t, Contains[int, any, any]{Value: 3}, NewCase[int, any, any](1), 42)
		res := assertion(t, rc, "Contains")
		if boolValue(t, res) {
			t.Fatalf("expected failure on non-container output")
		}
		want := `Containment check failed: cannot check containment in int`
		if res.Reason != want {
			t.Fatalf("reason = %q, want %q", res.Reason, want)
		}
	})

	t.Run("unnamed pointer type", func(t *testing.T) {
		n := 5
		rc := runBuiltin[any](t, Contains[int, any, any]{Value: 3}, NewCase[int, any, any](1), &n)
		res := assertion(t, rc, "Contains")
		if boolValue(t, res) {
			t.Fatalf("expected failure on pointer output")
		}
		want := `Containment check failed: cannot check containment in *int`
		if res.Reason != want {
			t.Fatalf("reason = %q, want %q", res.Reason, want)
		}
	})
}

func TestContainsSpec(t *testing.T) {
	t.Run("value only", func(t *testing.T) {
		c := Contains[int, any, any]{Value: "x"}
		want := EvaluatorSpec{Name: "Contains", Kwargs: map[string]any{"value": "x"}}
		if got := c.Spec(); !reflect.DeepEqual(got, want) {
			t.Fatalf("Spec() = %+v, want %+v", got, want)
		}
		if got := c.DefaultEvaluationName(); got != "Contains" {
			t.Fatalf("DefaultEvaluationName() = %q", got)
		}
	})

	t.Run("all fields set", func(t *testing.T) {
		c := Contains[int, any, any]{Value: "x", CaseSensitive: true, AsStrings: true, EvaluationName: "has_x"}
		want := EvaluatorSpec{Name: "Contains", Kwargs: map[string]any{
			"value":           "x",
			"case_sensitive":  true,
			"as_strings":      true,
			"evaluation_name": "has_x",
		}}
		if got := c.Spec(); !reflect.DeepEqual(got, want) {
			t.Fatalf("Spec() = %+v, want %+v", got, want)
		}
		if got := c.DefaultEvaluationName(); got != "has_x" {
			t.Fatalf("DefaultEvaluationName() = %q", got)
		}
	})
}

func TestIsInstance(t *testing.T) {
	t.Run("match string", func(t *testing.T) {
		rc := runBuiltin[any](t, IsInstance[int, any, any]{TypeName: "string"}, NewCase[int, any, any](1), "hello")
		res := assertion(t, rc, "IsInstance")
		if !boolValue(t, res) {
			t.Fatalf("expected string match")
		}
		if res.Reason != "" {
			t.Fatalf("expected empty reason, got %q", res.Reason)
		}
	})

	t.Run("match int", func(t *testing.T) {
		rc := runBuiltin[any](t, IsInstance[int, any, any]{TypeName: "int"}, NewCase[int, any, any](1), 7)
		if !boolValue(t, assertion(t, rc, "IsInstance")) {
			t.Fatalf("expected int match")
		}
	})

	t.Run("match named struct type", func(t *testing.T) {
		rc := runBuiltin[any](t, IsInstance[int, any, any]{TypeName: "namedType"}, NewCase[int, any, any](1), namedType{X: 1})
		if !boolValue(t, assertion(t, rc, "IsInstance")) {
			t.Fatalf("expected named struct match")
		}
	})

	t.Run("mismatch reason", func(t *testing.T) {
		rc := runBuiltin[any](t, IsInstance[int, any, any]{TypeName: "string"}, NewCase[int, any, any](1), 7)
		res := assertion(t, rc, "IsInstance")
		if boolValue(t, res) {
			t.Fatalf("expected mismatch")
		}
		want := "output is of type int"
		if res.Reason != want {
			t.Fatalf("reason = %q, want %q", res.Reason, want)
		}
	})

	t.Run("with evaluation name", func(t *testing.T) {
		rc := runBuiltin[any](t, IsInstance[int, any, any]{TypeName: "string", EvaluationName: "is_str"}, NewCase[int, any, any](1), "x")
		if !boolValue(t, assertion(t, rc, "is_str")) {
			t.Fatalf("expected match under custom name")
		}
	})

	t.Run("nil output reports nil type", func(t *testing.T) {
		rc := runBuiltin[any](t, IsInstance[int, any, any]{TypeName: "string"}, NewCase[int, any, any](1), nil)
		res := assertion(t, rc, "IsInstance")
		if boolValue(t, res) {
			t.Fatalf("expected mismatch for nil output")
		}
		if res.Reason != "output is of type nil" {
			t.Fatalf("reason = %q, want %q", res.Reason, "output is of type nil")
		}
	})

	t.Run("spec single arg", func(t *testing.T) {
		e := IsInstance[int, any, any]{TypeName: "string"}
		want := EvaluatorSpec{Name: "IsInstance", Args: []any{"string"}}
		if got := e.Spec(); !reflect.DeepEqual(got, want) {
			t.Fatalf("Spec() = %+v, want %+v", got, want)
		}
		if got := e.DefaultEvaluationName(); got != "IsInstance" {
			t.Fatalf("DefaultEvaluationName() = %q", got)
		}
	})

	t.Run("spec kwargs", func(t *testing.T) {
		e := IsInstance[int, any, any]{TypeName: "string", EvaluationName: "is_str"}
		want := EvaluatorSpec{Name: "IsInstance", Kwargs: map[string]any{"type_name": "string", "evaluation_name": "is_str"}}
		if got := e.Spec(); !reflect.DeepEqual(got, want) {
			t.Fatalf("Spec() = %+v, want %+v", got, want)
		}
		if got := e.DefaultEvaluationName(); got != "is_str" {
			t.Fatalf("DefaultEvaluationName() = %q", got)
		}
	})
}

func TestMaxDuration(t *testing.T) {
	t.Run("under limit passes", func(t *testing.T) {
		rc := runBuiltin[any](t, MaxDuration[int, any, any]{Max: time.Hour}, NewCase[int, any, any](1), "x")
		if !boolValue(t, assertion(t, rc, "MaxDuration")) {
			t.Fatalf("expected pass under a large limit")
		}
	})

	t.Run("over limit fails", func(t *testing.T) {
		ds, err := NewDataset("ds", []Case[int, any, any]{
			NewCase[int, any, any](1, WithCaseEvaluators[int, any, any](MaxDuration[int, any, any]{Max: 0})),
		})
		if err != nil {
			t.Fatalf("NewDataset: %v", err)
		}
		task := func(_ context.Context, _ int) (any, error) {
			time.Sleep(time.Millisecond)
			return "x", nil
		}
		report, err := ds.Evaluate(context.Background(), task)
		if err != nil {
			t.Fatalf("Evaluate: %v", err)
		}
		res := assertion(t, report.Cases[0], "MaxDuration")
		if boolValue(t, res) {
			t.Fatalf("expected fail when Max is 0 and a real duration elapsed")
		}
	})

	t.Run("spec produces seconds", func(t *testing.T) {
		e := MaxDuration[int, any, any]{Max: 2500 * time.Millisecond}
		want := EvaluatorSpec{Name: "MaxDuration", Args: []any{2.5}}
		if got := e.Spec(); !reflect.DeepEqual(got, want) {
			t.Fatalf("Spec() = %+v, want %+v", got, want)
		}
	})
}
