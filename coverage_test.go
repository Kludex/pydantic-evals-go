package evals

import (
	"context"
	"strings"
	"testing"
	"time"
)

// covScalarToFloatBranches drives the Float, Bool(true/false) and default arms of
// scalarToFloat via Averages, which calls it on every score value. Scores
// produced by real evaluators are only Int/Float, so the Bool and Label arms are
// reached by constructing ReportCases (whose fields are exported) directly.
func TestCovScalarToFloatBranchesViaAverages(t *testing.T) {
	report := &EvaluationReport[int, int, int]{
		Name: "cov",
		Cases: []ReportCase[int, int, int]{
			{
				Name: "c1",
				Scores: map[string]EvaluationResult{
					"f":     {Name: "f", Value: Float(2.5)},
					"i":     {Name: "i", Value: Int(4)},
					"btrue": {Name: "btrue", Value: Bool(true)},
					"bfals": {Name: "bfals", Value: Bool(false)},
					"lab":   {Name: "lab", Value: Label("x")},
				},
			},
		},
	}

	avg := report.Averages()
	if avg == nil {
		t.Fatal("expected averages")
	}
	want := map[string]float64{"f": 2.5, "i": 4, "btrue": 1, "bfals": 0, "lab": 0}
	for k, v := range want {
		if got := avg.Scores[k]; got != v {
			t.Fatalf("score %q = %v, want %v", k, got, v)
		}
	}
}

// covAggregateRowNilAssertions reaches the aggregateRow branch where an aggregate
// has no assertions: a single case with a score but no assertions renders an
// "Averages" row whose assertions cell is empty.
func TestCovAggregateRowNilAssertions(t *testing.T) {
	report := &EvaluationReport[int, int, int]{
		Name: "agg",
		Cases: []ReportCase[int, int, int]{
			{Name: "c1", Scores: map[string]EvaluationResult{"s": {Name: "s", Value: Int(1)}}},
			{Name: "c2", Scores: map[string]EvaluationResult{"s": {Name: "s", Value: Int(3)}}},
		},
	}
	out := report.Render(RenderOptions{IncludeAverages: true})
	if !strings.Contains(out, "Averages") {
		t.Fatalf("expected Averages row, got:\n%s", out)
	}
	if strings.Contains(out, "✔") {
		t.Fatalf("did not expect an assertion mark, got:\n%s", out)
	}
}

// covPadGapNegative reaches pad's gap<0 branch: a right-aligned duration cell
// whose content is wider than the header width forces a negative gap on the
// header pad of that column.
func TestCovPadGapNegative(t *testing.T) {
	report := &EvaluationReport[int, int, int]{
		Name: "pad",
		Cases: []ReportCase[int, int, int]{
			{Name: "only", TaskDuration: 1500 * time.Millisecond},
		},
	}
	out := report.Render(RenderOptions{IncludeDurations: true})
	if !strings.Contains(out, "1.5s") {
		t.Fatalf("expected duration cell, got:\n%s", out)
	}
}

// covRenderDurationMicrosecondsPrecisionZero reaches the µs branch of
// renderDuration where the value rounds to >=1 and precision drops to 0.
func TestCovRenderDurationMicrosecondsPrecisionZero(t *testing.T) {
	report := &EvaluationReport[int, int, int]{
		Name: "us",
		Cases: []ReportCase[int, int, int]{
			{Name: "fast", TaskDuration: 5 * time.Microsecond},
		},
	}
	out := report.Render(RenderOptions{IncludeDurations: true})
	if !strings.Contains(out, "5µs") {
		t.Fatalf("expected 5µs, got:\n%s", out)
	}
}

// covAveragePresentKeysSkipMissing reaches averagePresentKeys via a multi-run
// experiment where one group's summary lacks a key another has, plus an
// aggregate whose Assertions is nil (covering the nil arm of averageAggregates).
func TestCovAveragePresentKeysAndAggregateNilAssertions(t *testing.T) {
	d := mustDataset(t, "multi",
		NewCase[string, string, string]("a", WithCaseName[string, string, string]("a")),
		NewCase[string, string, string]("b", WithCaseName[string, string, string]("b")),
	)
	// Per-case evaluator that emits a score only for case "a", so the two
	// groups' summaries have different score keys.
	d.AddEvaluator(condScore{})

	report, err := d.Evaluate(context.Background(), echoTask, WithRepeat[string, string, string](2))
	if err != nil {
		t.Fatal(err)
	}
	avg := report.Averages()
	if avg == nil {
		t.Fatal("expected averages")
	}
	if _, ok := avg.Scores["only_a"]; !ok {
		t.Fatalf("expected only_a score in averages: %#v", avg.Scores)
	}
	if avg.Assertions != nil {
		t.Fatalf("expected nil assertions, got %v", *avg.Assertions)
	}
}

type condScore struct{}

func (condScore) Evaluate(_ context.Context, ec *EvaluatorContext[string, string, string]) (EvaluatorOutput, error) {
	if ec.Inputs == "a" {
		return ScalarValue(Float(1.0)), nil
	}
	return ScalarMapOutput{}, nil
}

func (condScore) Spec() EvaluatorSpec { return NewSpec("condScore") }

func (condScore) DefaultEvaluationName() string { return "only_a" }

// covCaseGroupsFailureOnlyGroup reaches the CaseGroups branch where a group has
// only failures (len(runs)==0), so identity is pulled from the first failure.
func TestCovCaseGroupsFailureOnlyGroup(t *testing.T) {
	d := mustDataset(t, "fails",
		NewCase[string, string, string]("boom", WithCaseName[string, string, string]("boom")),
	)
	report, err := d.Evaluate(context.Background(), failTask, WithRepeat[string, string, string](2))
	if err != nil {
		t.Fatal(err)
	}
	groups := report.CaseGroups()
	if len(groups) != 1 {
		t.Fatalf("expected 1 group, got %d", len(groups))
	}
	g := groups[0]
	if len(g.Runs) != 0 {
		t.Fatalf("expected only failures, got %d runs", len(g.Runs))
	}
	if g.Name != "boom" {
		t.Fatalf("group name = %q", g.Name)
	}
	if g.Inputs != "boom" {
		t.Fatalf("expected inputs from first failure, got %q", g.Inputs)
	}
}

func failTask(_ context.Context, _ string) (string, error) {
	return "", errCov("kaboom")
}

type errCov string

func (e errCov) Error() string { return string(e) }

// covErrorTypeNilAndAnonymous covers errorType's nil and unnamed-type arms via a
// failing task whose error has a named type, plus the report cell rendering that
// uses it.
func TestCovErrorTypePopulatesFailure(t *testing.T) {
	d := mustDataset(t, "errtype",
		NewCase[string, string, string]("x"),
	)
	report, err := d.Evaluate(context.Background(), failTask, WithName[string, string, string]("errtype"))
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Failures) != 1 {
		t.Fatalf("expected 1 failure, got %d", len(report.Failures))
	}
	if report.Failures[0].ErrorType != "errCov" {
		t.Fatalf("ErrorType = %q, want errCov", report.Failures[0].ErrorType)
	}
}

// covSprintValueMapBranch reaches sprintValue's map[string]any arm by rendering a
// case whose inputs are a map, with deterministic sorted-key output.
func TestCovSprintValueMapBranch(t *testing.T) {
	report := &EvaluationReport[map[string]any, int, int]{
		Name: "map",
		Cases: []ReportCase[map[string]any, int, int]{
			{Name: "c1", Inputs: map[string]any{"b": 2, "a": 1}},
		},
	}
	out := report.Render(RenderOptions{IncludeInput: true})
	if !strings.Contains(out, "{a: 1, b: 2}") {
		t.Fatalf("expected sorted map rendering, got:\n%s", out)
	}
}

// covToFloatReachableKinds drives toFloat's float64 (JSON), int (YAML) and error
// arms through MaxDuration's registry factory, which calls toFloat on the seconds
// argument. JSON numbers deserialize as float64 and yaml.v3 integers as int;
// float32/int64/json.Number cannot be produced by LoadDataset and are therefore
// not reachable from the public API.
func TestCovToFloatReachableKinds(t *testing.T) {
	t.Run("float64ViaJSON", func(t *testing.T) {
		reg := NewRegistry[string, string, string]()
		reg.RegisterDefaults()
		data := []byte(`{"name":"td","cases":[{"inputs":"x"}],"evaluators":[{"MaxDuration":{"seconds":1.5}}]}`)
		ds, err := LoadDataset(data, reg, LoadOptions[string, string, string]{Format: "json"})
		if err != nil {
			t.Fatal(err)
		}
		if got := ds.Evaluators[0].(MaxDuration[string, string, string]).Max; got != 1500*time.Millisecond {
			t.Fatalf("Max = %v", got)
		}
	})

	t.Run("intViaYAML", func(t *testing.T) {
		reg := NewRegistry[string, string, string]()
		reg.RegisterDefaults()
		data := []byte("name: td\ncases:\n  - inputs: x\nevaluators:\n  - MaxDuration: 2\n")
		ds, err := LoadDataset(data, reg, LoadOptions[string, string, string]{})
		if err != nil {
			t.Fatal(err)
		}
		if got := ds.Evaluators[0].(MaxDuration[string, string, string]).Max; got != 2*time.Second {
			t.Fatalf("Max = %v", got)
		}
	})

	t.Run("errorArm", func(t *testing.T) {
		reg := NewRegistry[string, string, string]()
		reg.RegisterDefaults()
		data := []byte("name: td\ncases:\n  - inputs: x\nevaluators:\n  - MaxDuration: not-a-number\n")
		if _, err := LoadDataset(data, reg, LoadOptions[string, string, string]{}); err == nil {
			t.Fatal("expected error from non-numeric seconds")
		}
	})
}

// TestCovAsStringKeyedMapViaSave round-trips a kwargs evaluator spec through YAML
// to exercise the string-keyed-map kwargs path of spec parsing. normalizeYAML
// converts yaml.v3's map[any]any to map[string]any before parsing, so the parser
// only ever sees string-keyed maps.
func TestCovAsStringKeyedMapViaSave(t *testing.T) {
	// kwargs spec -> shortForm produces map[string]any{name: kwargs}; round-trip
	// through YAML exercises the string-keyed kwargs branch.
	d := mustDataset(t, "skm")
	d.AddEvaluator(specEval{spec: NewSpecKwargs("Custom", map[string]any{"k": "v"})})

	data, err := d.Save(SaveOptions{Format: "yaml"})
	if err != nil {
		t.Fatal(err)
	}

	reg := NewRegistry[string, string, string]()
	reg.Register("Custom", func(spec EvaluatorSpec) (Evaluator[string, string, string], error) {
		if spec.Kwargs["k"] != "v" {
			t.Fatalf("expected kwargs k=v, got %#v", spec.Kwargs)
		}
		return specEval{spec: spec}, nil
	})
	if _, err := LoadDataset(data, reg, LoadOptions[string, string, string]{DefaultName: "skm"}); err != nil {
		t.Fatal(err)
	}
}

type specEval struct{ spec EvaluatorSpec }

func (s specEval) Evaluate(_ context.Context, _ *EvaluatorContext[string, string, string]) (EvaluatorOutput, error) {
	return ScalarValue(Bool(true)), nil
}

func (s specEval) Spec() EvaluatorSpec { return s.spec }

// covConvertViaError reaches convertVia's json.Unmarshal error arm: a non-nil
// DecodeInputs is absent so convertVia is used, and a value that cannot decode
// into the target type (a string into an int) triggers the unmarshal error.
func TestCovConvertViaUnmarshalError(t *testing.T) {
	reg := NewRegistry[int, int, int]()
	yamlData := "name: cv\ncases:\n  - inputs: not-an-int\n"
	_, err := LoadDataset([]byte(yamlData), reg, LoadOptions[int, int, int]{})
	if err == nil {
		t.Fatal("expected decode error")
	}
	if !strings.Contains(err.Error(), "inputs") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func mustDataset(t *testing.T, name string, cases ...Case[string, string, string]) *Dataset[string, string, string] {
	t.Helper()
	d, err := NewDataset(name, cases)
	if err != nil {
		t.Fatal(err)
	}
	return d
}

func echoTask(_ context.Context, in string) (string, error) { return in, nil }

// covSprintValueDefaultArm reaches sprintValue's default (non-map) arm by
// rendering a case whose inputs are a plain slice.
func TestCovSprintValueDefaultArm(t *testing.T) {
	report := &EvaluationReport[[]int, int, int]{
		Name: "slice",
		Cases: []ReportCase[[]int, int, int]{
			{Name: "c1", Inputs: []int{1, 2, 3}},
		},
	}
	out := report.Render(RenderOptions{IncludeInput: true})
	if !strings.Contains(out, "[1 2 3]") {
		t.Fatalf("expected slice rendering, got:\n%s", out)
	}
}

// covCaseGroupsEmptySourceFallsBackToName reaches the key=="" fall-back arms of
// CaseGroups for both a case and a failure: one case carries a SourceCaseName
// (so grouping is active) while another case and a failure leave it empty,
// forcing the fall-back to Name.
func TestCovCaseGroupsEmptySourceFallsBackToName(t *testing.T) {
	report := &EvaluationReport[string, string, string]{
		Name: "mixed",
		Cases: []ReportCase[string, string, string]{
			{Name: "grouped_1", SourceCaseName: "grouped"},
			{Name: "lonely"},
		},
		Failures: []ReportCaseFailure[string, string, string]{
			{Name: "failed"},
		},
	}
	groups := report.CaseGroups()
	names := map[string]bool{}
	for _, g := range groups {
		names[g.Name] = true
	}
	for _, want := range []string{"grouped", "lonely", "failed"} {
		if !names[want] {
			t.Fatalf("missing group %q in %v", want, names)
		}
	}
}

// covRenderDurationZero reaches renderDuration's seconds==0 "0s" arm via a case
// whose task duration is zero.
func TestCovRenderDurationZero(t *testing.T) {
	report := &EvaluationReport[int, int, int]{
		Name:  "zero",
		Cases: []ReportCase[int, int, int]{{Name: "c1", TaskDuration: 0}},
	}
	out := report.Render(RenderOptions{IncludeDurations: true})
	if !strings.Contains(out, "0s") {
		t.Fatalf("expected 0s, got:\n%s", out)
	}
}

// TestCovSaveYAMLEncodeError reaches Save's YAML encode-error arm with an input
// value the YAML encoder cannot marshal.
func TestCovSaveYAMLEncodeError(t *testing.T) {
	d := &Dataset[any, any, any]{
		Name:  "ds",
		Cases: []Case[any, any, any]{{Name: "c1", Inputs: make(chan int)}},
	}
	if _, err := d.Save(SaveOptions{Format: "yaml"}); err == nil {
		t.Fatal("expected a YAML encode error for an unsupported input type")
	}
}
