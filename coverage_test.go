package evals_test

import (
	"context"
	"strings"
	"testing"

	evals "github.com/Kludex/pydantic-evals-go"
)

// TestCovSuiteContains exercises Suite.Contains, the type-bound builder facade
// for the Contains evaluator, and runs the built evaluator end-to-end.
func TestCovSuiteContains(t *testing.T) {
	s := evals.For[string, string, any]()
	ev := s.Contains("ell")
	if ev.Value != "ell" {
		t.Fatalf("Contains value = %v, want %q", ev.Value, "ell")
	}
	ds, err := evals.NewDataset[string, string, any](
		"d",
		[]evals.Case[string, string, any]{s.Case("hello").Name("c1").Build()},
		ev,
	)
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}
	report, err := ds.Evaluate(context.Background(), ltEchoTask)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	res, ok := report.Cases[0].Assertions["Contains"]
	if !ok {
		t.Fatalf("missing Contains assertion: %v", keysOf(report.Cases[0].Assertions))
	}
	if v, ok := res.Value.(evals.Bool); !ok || !bool(v) {
		t.Fatalf("Contains result = %#v, want Bool(true)", res.Value)
	}
}

// covPtrEvaluator has its Evaluate method on a pointer receiver so that, when
// used as &covPtrEvaluator{}, evaluatorTypeName must dereference the pointer to
// recover the underlying type name.
type covPtrEvaluator struct{}

func (e *covPtrEvaluator) Evaluate(_ context.Context, _ *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	return evals.Assertion(true), nil
}

func TestCovEvaluatorTypeNamePointer(t *testing.T) {
	s := evals.For[string, string, any]()
	ds, err := evals.NewDataset[string, string, any](
		"d",
		[]evals.Case[string, string, any]{s.Case("hi").Name("c1").Build()},
		&covPtrEvaluator{},
	)
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}
	report, err := ds.Evaluate(context.Background(), ltEchoTask)
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	got := report.Cases[0].Assertions
	if _, ok := got["covPtrEvaluator"]; !ok {
		t.Fatalf("expected assertion keyed by dereferenced type name, got %v", keysOf(got))
	}
}

// TestCovSprintValueMap renders an Output that is a map[string]any so the
// sorted-key map branch of sprintValue is exercised.
func TestCovSprintValueMap(t *testing.T) {
	rep := &evals.EvaluationReport[any, any, any]{
		Name: "task",
		Cases: []evals.ReportCase[any, any, any]{
			{
				Name:   "c1",
				Inputs: "in",
				Output: map[string]any{"b": 2, "a": 1},
			},
		},
	}
	got := rep.Render(evals.RenderOptions{IncludeOutput: true})
	if !strings.Contains(got, "{a: 1, b: 2}") {
		t.Fatalf("expected sorted map rendering, got:\n%s", got)
	}
}

// TestCovSprintValueDefault renders an Output whose Go type is neither string,
// nil, Scalar, nor map[string]any, hitting the default fmt arm of sprintValue.
func TestCovSprintValueDefault(t *testing.T) {
	rep := &evals.EvaluationReport[any, any, any]{
		Name: "task",
		Cases: []evals.ReportCase[any, any, any]{
			{Name: "c1", Inputs: "in", Output: 42},
		},
	}
	got := rep.Render(evals.RenderOptions{IncludeOutput: true})
	if !strings.Contains(got, "42") {
		t.Fatalf("expected fmt rendering of int output, got:\n%s", got)
	}
}

// TestCovScalarToFloatBool builds a report whose Scores map carries a Bool value
// directly (legal via the exported EvaluationResult.Value field) so that the Bool
// arm of scalarToFloat is reached through Averages.
func TestCovScalarToFloatBool(t *testing.T) {
	cases := []struct {
		name string
		val  evals.Scalar
		want float64
	}{
		{"true", evals.Bool(true), 1},
		{"false", evals.Bool(false), 0},
		{"label", evals.Label("x"), 0},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rep := &evals.EvaluationReport[string, string, any]{
				Name: "task",
				Cases: []evals.ReportCase[string, string, any]{
					{
						Name: "c1",
						Scores: map[string]evals.EvaluationResult{
							"sc": {Name: "sc", Value: tc.val},
						},
					},
				},
			}
			avg := rep.Averages()
			if avg == nil {
				t.Fatal("Averages returned nil")
			}
			if got := avg.Scores["sc"]; got != tc.want {
				t.Fatalf("avg score = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestCovCaseGroupsEmptySourceName covers the key == "" fallbacks in CaseGroups
// for both runs and failures: a multi-run report where some entries lack a
// SourceCaseName and must fall back to their own Name.
func TestCovCaseGroupsEmptySourceName(t *testing.T) {
	rep := &evals.EvaluationReport[string, string, any]{
		Name: "task",
		Cases: []evals.ReportCase[string, string, any]{
			{Name: "sourced", SourceCaseName: "grp"},
			{Name: "lonely"},
		},
		Failures: []evals.ReportCaseFailure[string, string, any]{
			{Name: "failsourced", SourceCaseName: "grp"},
			{Name: "faillonely"},
		},
	}
	groups := rep.CaseGroups()
	names := make(map[string]bool, len(groups))
	for _, g := range groups {
		names[g.Name] = true
	}
	for _, want := range []string{"grp", "lonely", "faillonely"} {
		if !names[want] {
			t.Fatalf("missing group %q in %v", want, names)
		}
	}
}

// TestCovNormalizeYAMLNonStringKeys loads a YAML dataset whose inputs are a
// nested mapping with a non-string key and a sequence, forcing normalizeYAML to
// walk its map[any]any and []any branches.
func TestCovNormalizeYAMLNonStringKeys(t *testing.T) {
	const data = "name: d\ncases:\n  - inputs:\n      1: x\n      list: [a, b]\n"
	reg := evals.NewRegistry[string, string, string]()
	reg.RegisterDefaults()
	var captured any
	ds, err := evals.LoadDataset([]byte(data), reg, evals.LoadOptions[string, string, string]{
		DecodeInputs: func(v any) (string, error) {
			captured = v
			return "ok", nil
		},
	})
	if err != nil {
		t.Fatalf("LoadDataset: %v", err)
	}
	if len(ds.Cases) != 1 {
		t.Fatalf("expected 1 case, got %d", len(ds.Cases))
	}
	m, ok := captured.(map[string]any)
	if !ok {
		t.Fatalf("normalized inputs not map[string]any: %#v", captured)
	}
	if m["1"] != "x" {
		t.Fatalf("non-string key not normalized: %#v", m)
	}
	list, ok := m["list"].([]any)
	if !ok || len(list) != 2 || list[0] != "a" || list[1] != "b" {
		t.Fatalf("sequence not normalized: %#v", m["list"])
	}
}
