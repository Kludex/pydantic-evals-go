package evals_test

import (
	"context"
	"testing"
	"time"

	evals "github.com/Kludex/pydantic-evals-go"
)

// evalOne runs a single evaluator against one synthesized case and returns the
// resulting ReportCase. It drives the public Dataset.Evaluate path so that the
// evaluator's Output is grouped into Assertions/Scores/Labels exactly as it
// would be in real use.
func evalOne[I, O, M any](t *testing.T, c evals.Case[I, O, M], task evals.TaskFunc[I, O], ev evals.Evaluator[I, O, M]) evals.ReportCase[I, O, M] {
	t.Helper()
	ds, err := evals.NewDataset("ds", []evals.Case[I, O, M]{c}, ev)
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}
	report, err := ds.Evaluate(context.Background(), task)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(report.Failures) != 0 {
		t.Fatalf("unexpected case failures: %+v", report.Failures)
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

// constTask returns a task that ignores its inputs and always produces out.
func constTask[I, O any](out O) evals.TaskFunc[I, O] {
	return func(_ context.Context, _ I) (O, error) { return out, nil }
}

func assertAssertion(t *testing.T, rc evals.ReportCase[string, string, any], name string, want bool, wantReason string) {
	t.Helper()
	r, ok := rc.Assertions[name]
	if !ok {
		t.Fatalf("assertion %q not found; have %v", name, keysOf(rc.Assertions))
	}
	if got := bool(r.Value.(evals.Bool)); got != want {
		t.Fatalf("assertion %q value = %v, want %v", name, got, want)
	}
	if r.Reason != wantReason {
		t.Fatalf("assertion %q reason = %q, want %q", name, r.Reason, wantReason)
	}
	if r.Name != name {
		t.Fatalf("assertion result Name = %q, want %q", r.Name, name)
	}
}

func keysOf[V any](m map[string]V) []string {
	ks := make([]string, 0, len(m))
	for k := range m {
		ks = append(ks, k)
	}
	return ks
}

func TestEquals(t *testing.T) {
	t.Run("match", func(t *testing.T) {
		c := evals.NewCase[string, string, any]("in")
		rc := evalOne(t, c, constTask[string]("hello"), evals.Equals[string, string, any]{Value: "hello"})
		assertAssertion(t, rc, "Equals", true, "")
	})

	t.Run("mismatch", func(t *testing.T) {
		c := evals.NewCase[string, string, any]("in")
		rc := evalOne(t, c, constTask[string]("hello"), evals.Equals[string, string, any]{Value: "world"})
		assertAssertion(t, rc, "Equals", false, "")
	})

	t.Run("custom name", func(t *testing.T) {
		c := evals.NewCase[string, string, any]("in")
		rc := evalOne(t, c, constTask[string]("hello"), evals.Equals[string, string, any]{Value: "hello", Name: "matches"})
		assertAssertion(t, rc, "matches", true, "")
	})
}

func TestEqualsEvaluationNameAndSpec(t *testing.T) {
	def := evals.Equals[string, string, any]{Value: "x"}
	if got := def.EvaluationName(); got != "Equals" {
		t.Fatalf("default EvaluationName = %q, want %q", got, "Equals")
	}
	named := evals.Equals[string, string, any]{Value: "x", Name: "eq"}
	if got := named.EvaluationName(); got != "eq" {
		t.Fatalf("custom EvaluationName = %q, want %q", got, "eq")
	}

	spec := def.Spec()
	if spec.Name != "Equals" {
		t.Fatalf("spec Name = %q, want %q", spec.Name, "Equals")
	}
	if got, ok := spec.Kwargs["value"]; !ok || got != "x" {
		t.Fatalf("spec Kwargs[value] = %v (ok=%v), want %q", got, ok, "x")
	}
	if _, ok := spec.Kwargs["evaluation_name"]; ok {
		t.Fatalf("default spec should not set evaluation_name, got %v", spec.Kwargs)
	}

	specNamed := named.Spec()
	if got := specNamed.Kwargs["evaluation_name"]; got != "eq" {
		t.Fatalf("named spec evaluation_name = %v, want %q", got, "eq")
	}
}

func TestEqualsExpected(t *testing.T) {
	t.Run("match", func(t *testing.T) {
		c := evals.NewCase[string, string, any]("in", evals.WithExpectedOutput[string, string, any]("hi"))
		rc := evalOne(t, c, constTask[string]("hi"), evals.EqualsExpected[string, string, any]{})
		assertAssertion(t, rc, "EqualsExpected", true, "")
	})

	t.Run("mismatch", func(t *testing.T) {
		c := evals.NewCase[string, string, any]("in", evals.WithExpectedOutput[string, string, any]("hi"))
		rc := evalOne(t, c, constTask[string]("bye"), evals.EqualsExpected[string, string, any]{})
		assertAssertion(t, rc, "EqualsExpected", false, "")
	})

	t.Run("no expected output yields no result", func(t *testing.T) {
		c := evals.NewCase[string, string, any]("in")
		rc := evalOne(t, c, constTask[string]("hi"), evals.EqualsExpected[string, string, any]{})
		if len(rc.Assertions) != 0 {
			t.Fatalf("expected no assertions, got %v", rc.Assertions)
		}
		if len(rc.Scores) != 0 || len(rc.Labels) != 0 {
			t.Fatalf("expected no scores/labels, got scores=%v labels=%v", rc.Scores, rc.Labels)
		}
	})

	t.Run("custom name", func(t *testing.T) {
		c := evals.NewCase[string, string, any]("in", evals.WithExpectedOutput[string, string, any]("hi"))
		rc := evalOne(t, c, constTask[string]("hi"), evals.EqualsExpected[string, string, any]{Name: "exp"})
		assertAssertion(t, rc, "exp", true, "")
	})
}

func TestEqualsExpectedEvaluationNameAndSpec(t *testing.T) {
	def := evals.EqualsExpected[string, string, any]{}
	if got := def.EvaluationName(); got != "EqualsExpected" {
		t.Fatalf("default EvaluationName = %q, want %q", got, "EqualsExpected")
	}
	if got := (evals.EqualsExpected[string, string, any]{Name: "x"}).EvaluationName(); got != "x" {
		t.Fatalf("custom EvaluationName = %q, want %q", got, "x")
	}

	spec := def.Spec()
	if spec.Name != "EqualsExpected" {
		t.Fatalf("spec Name = %q, want %q", spec.Name, "EqualsExpected")
	}
	if len(spec.Kwargs) != 0 || len(spec.Args) != 0 {
		t.Fatalf("default EqualsExpected spec should be bare, got %+v", spec)
	}

	specNamed := (evals.EqualsExpected[string, string, any]{Name: "x"}).Spec()
	if got := specNamed.Kwargs["evaluation_name"]; got != "x" {
		t.Fatalf("named spec evaluation_name = %v, want %q", got, "x")
	}
}

func TestContainsString(t *testing.T) {
	c := evals.NewCase[string, any, any]("in")

	t.Run("substring match", func(t *testing.T) {
		rc := evalOneAny(t, c, "hello world", evals.Contains[string, any, any]{Value: "world"})
		assertAnyAssertion(t, rc, "Contains", true, "")
	})

	t.Run("substring mismatch", func(t *testing.T) {
		rc := evalOneAny(t, c, "hello world", evals.Contains[string, any, any]{Value: "xyz"})
		assertAnyAssertion(t, rc, "Contains", false,
			`Output string "hello world" does not contain expected string "xyz"`)
	})

	t.Run("case insensitive by default", func(t *testing.T) {
		rc := evalOneAny(t, c, "Hello WORLD", evals.Contains[string, any, any]{Value: "world"})
		assertAnyAssertion(t, rc, "Contains", true, "")
	})

	t.Run("case sensitive mismatch", func(t *testing.T) {
		rc := evalOneAny(t, c, "Hello WORLD", evals.Contains[string, any, any]{Value: "world", CaseSensitive: true})
		assertAnyAssertion(t, rc, "Contains", false,
			`Output string "Hello WORLD" does not contain expected string "world"`)
	})

	t.Run("case sensitive match", func(t *testing.T) {
		rc := evalOneAny(t, c, "Hello WORLD", evals.Contains[string, any, any]{Value: "WORLD", CaseSensitive: true})
		assertAnyAssertion(t, rc, "Contains", true, "")
	})
}

func TestContainsAsStrings(t *testing.T) {
	c := evals.NewCase[string, any, any]("in")
	// Output is an int slice but AsStrings forces a string comparison of the
	// fmt-rendered forms: "[1 2 3]" contains "2".
	rc := evalOneAny(t, c, []int{1, 2, 3}, evals.Contains[string, any, any]{Value: "2", AsStrings: true})
	assertAnyAssertion(t, rc, "Contains", true, "")

	rc = evalOneAny(t, c, []int{1, 2, 3}, evals.Contains[string, any, any]{Value: "9", AsStrings: true})
	assertAnyAssertion(t, rc, "Contains", false,
		`Output string "[1 2 3]" does not contain expected string "9"`)
}

func TestContainsSlice(t *testing.T) {
	c := evals.NewCase[string, any, any]("in")

	t.Run("member present", func(t *testing.T) {
		rc := evalOneAny(t, c, []int{1, 2, 3}, evals.Contains[string, any, any]{Value: 2})
		assertAnyAssertion(t, rc, "Contains", true, "")
	})

	t.Run("member absent", func(t *testing.T) {
		rc := evalOneAny(t, c, []int{1, 2, 3}, evals.Contains[string, any, any]{Value: 9})
		assertAnyAssertion(t, rc, "Contains", false,
			`Output []int{1, 2, 3} does not contain provided value`)
	})

	t.Run("member absent with truncated output", func(t *testing.T) {
		big := make([]int, 0, 80)
		for i := 0; i < 80; i++ {
			big = append(big, i)
		}
		rc := evalOneAny(t, c, big, evals.Contains[string, any, any]{Value: 999})
		assertAnyAssertion(t, rc, "Contains", false,
			"Output []int{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25, "+
				"... 55, 56, 57, 58, 59, 60, 61, 62, 63, 64, 65, 66, 67, 68, 69, 70, 71, 72, 73, 74, 75, 76, 77, 78, 79} "+
				"does not contain provided value")
	})
}

func TestContainsMap(t *testing.T) {
	c := evals.NewCase[string, any, any]("in")
	output := map[string]int{"a": 1, "b": 2}

	t.Run("map contains map match", func(t *testing.T) {
		rc := evalOneAny(t, c, output, evals.Contains[string, any, any]{Value: map[string]int{"a": 1}})
		assertAnyAssertion(t, rc, "Contains", true, "")
	})

	t.Run("map contains map missing key", func(t *testing.T) {
		rc := evalOneAny(t, c, output, evals.Contains[string, any, any]{Value: map[string]int{"z": 1}})
		assertAnyAssertion(t, rc, "Contains", false,
			`Output does not contain expected key "z"`)
	})

	t.Run("map contains map different value", func(t *testing.T) {
		rc := evalOneAny(t, c, output, evals.Contains[string, any, any]{Value: map[string]int{"a": 9}})
		assertAnyAssertion(t, rc, "Contains", false,
			`Output has different value for key "a": 1 != 9`)
	})

	t.Run("map contains key", func(t *testing.T) {
		rc := evalOneAny(t, c, output, evals.Contains[string, any, any]{Value: "a"})
		assertAnyAssertion(t, rc, "Contains", true, "")
	})

	t.Run("map missing key", func(t *testing.T) {
		rc := evalOneAny(t, c, output, evals.Contains[string, any, any]{Value: "z"})
		assertAnyAssertion(t, rc, "Contains", false,
			`Output map[string]int{"a":1, "b":2} does not contain provided value as a key`)
	})
}

func TestContainsNonContainer(t *testing.T) {
	c := evals.NewCase[string, any, any]("in")
	rc := evalOneAny(t, c, 42, evals.Contains[string, any, any]{Value: 1})
	assertAnyAssertion(t, rc, "Contains", false,
		`Containment check failed: cannot check containment in int`)
}

func TestContainsName(t *testing.T) {
	c := evals.NewCase[string, any, any]("in")
	rc := evalOneAny(t, c, "hello", evals.Contains[string, any, any]{Value: "hello", Name: "has"})
	assertAnyAssertion(t, rc, "has", true, "")
}

func TestContainsEvaluationNameAndSpec(t *testing.T) {
	if got := (evals.Contains[string, any, any]{Value: "x"}).EvaluationName(); got != "Contains" {
		t.Fatalf("default EvaluationName = %q, want %q", got, "Contains")
	}
	if got := (evals.Contains[string, any, any]{Value: "x", Name: "c"}).EvaluationName(); got != "c" {
		t.Fatalf("custom EvaluationName = %q, want %q", got, "c")
	}

	t.Run("spec without optional fields", func(t *testing.T) {
		spec := (evals.Contains[string, any, any]{Value: "x"}).Spec()
		if spec.Name != "Contains" {
			t.Fatalf("spec Name = %q, want %q", spec.Name, "Contains")
		}
		if got := spec.Kwargs["value"]; got != "x" {
			t.Fatalf("spec Kwargs[value] = %v, want %q", got, "x")
		}
		if _, ok := spec.Kwargs["case_sensitive"]; ok {
			t.Fatalf("default spec should not set case_sensitive: %v", spec.Kwargs)
		}
		if _, ok := spec.Kwargs["as_strings"]; ok {
			t.Fatalf("default spec should not set as_strings: %v", spec.Kwargs)
		}
		if _, ok := spec.Kwargs["evaluation_name"]; ok {
			t.Fatalf("default spec should not set evaluation_name: %v", spec.Kwargs)
		}
	})

	t.Run("spec with all fields", func(t *testing.T) {
		spec := (evals.Contains[string, any, any]{
			Value: "x", CaseSensitive: true, AsStrings: true, Name: "c",
		}).Spec()
		if got := spec.Kwargs["case_sensitive"]; got != true {
			t.Fatalf("spec case_sensitive = %v, want true", got)
		}
		if got := spec.Kwargs["as_strings"]; got != true {
			t.Fatalf("spec as_strings = %v, want true", got)
		}
		if got := spec.Kwargs["evaluation_name"]; got != "c" {
			t.Fatalf("spec evaluation_name = %v, want %q", got, "c")
		}
	})
}

func TestIsInstance(t *testing.T) {
	c := evals.NewCase[string, any, any]("in")

	t.Run("match string", func(t *testing.T) {
		rc := evalOneAny(t, c, "hello", evals.IsInstance[string, any, any]{TypeName: "string"})
		assertAnyAssertion(t, rc, "IsInstance", true, "")
	})

	t.Run("match int", func(t *testing.T) {
		rc := evalOneAny(t, c, 7, evals.IsInstance[string, any, any]{TypeName: "int"})
		assertAnyAssertion(t, rc, "IsInstance", true, "")
	})

	t.Run("match named struct type", func(t *testing.T) {
		rc := evalOneAny(t, c, point{X: 1}, evals.IsInstance[string, any, any]{TypeName: "point"})
		assertAnyAssertion(t, rc, "IsInstance", true, "")
	})

	t.Run("mismatch reports actual type", func(t *testing.T) {
		rc := evalOneAny(t, c, 7, evals.IsInstance[string, any, any]{TypeName: "string"})
		assertAnyAssertion(t, rc, "IsInstance", false, "output is of type int")
	})

	t.Run("nil output", func(t *testing.T) {
		rc := evalOneAny(t, c, nil, evals.IsInstance[string, any, any]{TypeName: "string"})
		assertAnyAssertion(t, rc, "IsInstance", false, "output is of type nil")
	})

	t.Run("anonymous type reports its String form", func(t *testing.T) {
		rc := evalOneAny(t, c, []int{1, 2, 3}, evals.IsInstance[string, any, any]{TypeName: "string"})
		assertAnyAssertion(t, rc, "IsInstance", false, "output is of type []int")
	})

	t.Run("custom name", func(t *testing.T) {
		rc := evalOneAny(t, c, "hello", evals.IsInstance[string, any, any]{TypeName: "string", Name: "isstr"})
		assertAnyAssertion(t, rc, "isstr", true, "")
	})
}

func TestIsInstanceEvaluationNameAndSpec(t *testing.T) {
	if got := (evals.IsInstance[string, any, any]{TypeName: "string"}).EvaluationName(); got != "IsInstance" {
		t.Fatalf("default EvaluationName = %q, want %q", got, "IsInstance")
	}
	if got := (evals.IsInstance[string, any, any]{TypeName: "string", Name: "i"}).EvaluationName(); got != "i" {
		t.Fatalf("custom EvaluationName = %q, want %q", got, "i")
	}

	t.Run("single positional arg without name", func(t *testing.T) {
		spec := (evals.IsInstance[string, any, any]{TypeName: "string"}).Spec()
		if spec.Name != "IsInstance" {
			t.Fatalf("spec Name = %q, want %q", spec.Name, "IsInstance")
		}
		if len(spec.Args) != 1 || spec.Args[0] != "string" {
			t.Fatalf("spec Args = %v, want single arg %q", spec.Args, "string")
		}
		if len(spec.Kwargs) != 0 {
			t.Fatalf("spec Kwargs should be empty, got %v", spec.Kwargs)
		}
	})

	t.Run("kwargs when named", func(t *testing.T) {
		spec := (evals.IsInstance[string, any, any]{TypeName: "string", Name: "i"}).Spec()
		if len(spec.Args) != 0 {
			t.Fatalf("named spec should have no positional Args, got %v", spec.Args)
		}
		if got := spec.Kwargs["type_name"]; got != "string" {
			t.Fatalf("spec type_name = %v, want %q", got, "string")
		}
		if got := spec.Kwargs["evaluation_name"]; got != "i" {
			t.Fatalf("spec evaluation_name = %v, want %q", got, "i")
		}
	})
}

func TestMaxDuration(t *testing.T) {
	c := evals.NewCase[string, string, any]("in")

	t.Run("under large max passes", func(t *testing.T) {
		rc := evalOne(t, c, constTask[string]("out"), evals.MaxDuration[string, string, any]{Max: time.Hour})
		assertAssertion(t, rc, "MaxDuration", true, "")
	})

	t.Run("zero max fails", func(t *testing.T) {
		task := func(_ context.Context, _ string) (string, error) {
			return "out", nil
		}
		rc := evalOne(t, c, task, evals.MaxDuration[string, string, any]{Max: 0})
		assertAssertion(t, rc, "MaxDuration", false, "")
	})
}

func TestMaxDurationSpecAndDefaultName(t *testing.T) {
	spec := (evals.MaxDuration[string, string, any]{Max: 2500 * time.Millisecond}).Spec()
	if spec.Name != "MaxDuration" {
		t.Fatalf("spec Name = %q, want %q", spec.Name, "MaxDuration")
	}
	if len(spec.Args) != 1 {
		t.Fatalf("spec Args = %v, want single arg", spec.Args)
	}
	if got := spec.Args[0].(float64); got != 2.5 {
		t.Fatalf("spec seconds = %v, want 2.5", got)
	}

	// MaxDuration does not implement NamedEvaluator, so its report name defaults
	// to the spec name (its Go type name "MaxDuration").
	c := evals.NewCase[string, string, any]("in")
	rc := evalOne(t, c, constTask[string]("out"), evals.MaxDuration[string, string, any]{Max: time.Hour})
	if _, ok := rc.Assertions["MaxDuration"]; !ok {
		t.Fatalf("MaxDuration default report name not found; have %v", keysOf(rc.Assertions))
	}
}

// TestBuiltinsDefaultNamesInReport renders a report whose evaluators carry no
// custom names and asserts each built-in surfaces under its default name.
func TestBuiltinsDefaultNamesInReport(t *testing.T) {
	s := evals.For[string, string, any]()
	ds := s.Dataset("defaults",
		s.Case("in").Expect("out").Eval(
			s.Equals("out"),
			s.EqualsExpected(),
			s.MaxDuration(time.Hour),
		),
	)
	report, err := ds.Evaluate(context.Background(), constTask[string]("out"))
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	rc := report.Cases[0]
	for _, name := range []string{"Equals", "EqualsExpected", "MaxDuration"} {
		if _, ok := rc.Assertions[name]; !ok {
			t.Fatalf("default-named assertion %q not present; have %v", name, keysOf(rc.Assertions))
		}
	}

	out := report.Render(evals.RenderOptions{IncludeReasons: true})
	want := "" +
		"   Evaluation Summary: task\n" +
		"┏━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━┓\n" +
		"┃ Case ID ┃ Assertions        ┃\n" +
		"┡━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━┩\n" +
		"│ Case 1  │ Equals: ✔         │\n" +
		"│         │ EqualsExpected: ✔ │\n" +
		"│         │ MaxDuration: ✔    │\n" +
		"│         │                   │\n" +
		"└─────────┴───────────────────┘"
	if out != want {
		t.Fatalf("rendered report mismatch:\n got:\n%s\nwant:\n%s", out, want)
	}
}

// --- helpers and fixtures local to this file ---

type point struct {
	X int
	Y int
}

func evalOneAny(t *testing.T, c evals.Case[string, any, any], out any, ev evals.Evaluator[string, any, any]) evals.ReportCase[string, any, any] {
	t.Helper()
	task := func(_ context.Context, _ string) (any, error) { return out, nil }
	ds, err := evals.NewDataset("ds", []evals.Case[string, any, any]{c}, ev)
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}
	report, err := ds.Evaluate(context.Background(), task)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if len(report.Failures) != 0 {
		t.Fatalf("unexpected case failures: %+v", report.Failures)
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

func assertAnyAssertion(t *testing.T, rc evals.ReportCase[string, any, any], name string, want bool, wantReason string) {
	t.Helper()
	r, ok := rc.Assertions[name]
	if !ok {
		t.Fatalf("assertion %q not found; have %v", name, keysOf(rc.Assertions))
	}
	if got := bool(r.Value.(evals.Bool)); got != want {
		t.Fatalf("assertion %q value = %v, want %v", name, got, want)
	}
	if r.Reason != wantReason {
		t.Fatalf("assertion %q reason = %q, want %q", name, r.Reason, wantReason)
	}
	if r.Name != name {
		t.Fatalf("assertion result Name = %q, want %q", r.Name, name)
	}
}
