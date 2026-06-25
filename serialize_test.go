package evals

import (
	"context"
	"strings"
	"testing"
	"time"
)

func newDefaultRegistry[I, O, M any]() *Registry[I, O, M] {
	reg := NewRegistry[I, O, M]()
	reg.RegisterDefaults()
	return reg
}

func TestSaveYAMLDefaultFormat(t *testing.T) {
	d, err := NewDataset[string, string, map[string]any](
		"ds",
		[]Case[string, string, map[string]any]{
			NewCase[string, string, map[string]any]("in1",
				WithCaseName[string, string, map[string]any]("c1"),
				WithExpectedOutput[string, string, map[string]any]("out1"),
				WithMetadata[string, string, map[string]any](map[string]any{"k": "v"}),
				WithCaseEvaluators[string, string, map[string]any](
					Equals[string, string, map[string]any]{Value: "out1"},
					Contains[string, string, map[string]any]{Value: "o", CaseSensitive: true},
					IsInstance[string, string, map[string]any]{TypeName: "string"},
					MaxDuration[string, string, map[string]any]{Max: 1500 * time.Millisecond},
				),
			),
		},
		EqualsExpected[string, string, map[string]any]{},
	)
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}

	got, err := d.Save(SaveOptions{})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}

	want := `name: ds
cases:
  - name: c1
    inputs: in1
    metadata:
      k: v
    expected_output: out1
    evaluators:
      - Equals:
          value: out1
      - Contains:
          case_sensitive: true
          value: o
      - IsInstance: string
      - MaxDuration: 1.5
evaluators:
  - EqualsExpected
`
	if string(got) != want {
		t.Fatalf("YAML mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestSaveYAMLWithSchema(t *testing.T) {
	d, err := NewDataset[string, string, map[string]any]("ds", []Case[string, string, map[string]any]{
		NewCase[string, string, map[string]any]("in1", WithCaseName[string, string, map[string]any]("c1")),
	})
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}
	got, err := d.Save(SaveOptions{Format: "yaml", Schema: "schema.json"})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	want := `# yaml-language-server: $schema=schema.json
$schema: schema.json
name: ds
cases:
  - name: c1
    inputs: in1
`
	if string(got) != want {
		t.Fatalf("YAML+schema mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestSaveJSON(t *testing.T) {
	d, err := NewDataset[string, string, map[string]any]("ds", []Case[string, string, map[string]any]{
		NewCase[string, string, map[string]any]("in1",
			WithCaseName[string, string, map[string]any]("c1"),
			WithExpectedOutput[string, string, map[string]any]("out1"),
			WithMetadata[string, string, map[string]any](map[string]any{"k": "v"}),
			WithCaseEvaluators[string, string, map[string]any](
				IsInstance[string, string, map[string]any]{TypeName: "string"},
			),
		),
	}, EqualsExpected[string, string, map[string]any]{})
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}

	got, err := d.Save(SaveOptions{Format: "json"})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	want := `{
  "name": "ds",
  "cases": [
    {
      "name": "c1",
      "inputs": "in1",
      "metadata": {
        "k": "v"
      },
      "expected_output": "out1",
      "evaluators": [
        {
          "IsInstance": "string"
        }
      ]
    }
  ],
  "evaluators": [
    "EqualsExpected"
  ]
}
`
	if string(got) != want {
		t.Fatalf("JSON mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestSaveJSONWithSchema(t *testing.T) {
	d, err := NewDataset[string, string, map[string]any]("ds", []Case[string, string, map[string]any]{
		NewCase[string, string, map[string]any]("in1", WithCaseName[string, string, map[string]any]("c1")),
	})
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}
	got, err := d.Save(SaveOptions{Format: "json", Schema: "schema.json"})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	want := `{
  "$schema": "schema.json",
  "name": "ds",
  "cases": [
    {
      "name": "c1",
      "inputs": "in1"
    }
  ]
}
`
	if string(got) != want {
		t.Fatalf("JSON+schema mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestSaveUnknownFormat(t *testing.T) {
	d, err := NewDataset[string, string, map[string]any]("ds", []Case[string, string, map[string]any]{
		NewCase[string, string, map[string]any]("in1"),
	})
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}
	_, err = d.Save(SaveOptions{Format: "toml"})
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
	want := `unknown format "toml" (want "yaml" or "json")`
	if err.Error() != want {
		t.Fatalf("error mismatch: got %q want %q", err.Error(), want)
	}
}

func TestSaveJSONMarshalError(t *testing.T) {
	d := &Dataset[any, any, any]{
		Name:  "ds",
		Cases: []Case[any, any, any]{{Name: "c1", Inputs: make(chan int)}},
	}
	_, err := d.Save(SaveOptions{Format: "json"})
	if err == nil {
		t.Fatal("expected marshal error for unsupported input type")
	}
	if !strings.Contains(err.Error(), "unsupported type") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestSpecConstructorsThroughSave(t *testing.T) {
	reg := NewRegistry[string, string, map[string]any]()
	reg.Register("Bare", func(EvaluatorSpec) (Evaluator[string, string, map[string]any], error) {
		return specEvaluator{spec: NewSpec("Bare")}, nil
	})
	reg.Register("Arg", func(EvaluatorSpec) (Evaluator[string, string, map[string]any], error) {
		return specEvaluator{spec: NewSpecArg("Arg", "hello")}, nil
	})
	reg.Register("Kw", func(EvaluatorSpec) (Evaluator[string, string, map[string]any], error) {
		return specEvaluator{spec: NewSpecKwargs("Kw", map[string]any{"x": "y"})}, nil
	})

	d, err := NewDataset[string, string, map[string]any]("ds", []Case[string, string, map[string]any]{
		NewCase[string, string, map[string]any]("in1", WithCaseName[string, string, map[string]any]("c1")),
	},
		specEvaluator{spec: NewSpec("Bare")},
		specEvaluator{spec: NewSpecArg("Arg", "hello")},
		specEvaluator{spec: NewSpecKwargs("Kw", map[string]any{"x": "y"})},
	)
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}

	got, err := d.Save(SaveOptions{})
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	want := `name: ds
cases:
  - name: c1
    inputs: in1
evaluators:
  - Bare
  - Arg: hello
  - Kw:
      x: "y"
`
	if string(got) != want {
		t.Fatalf("spec short-form mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestLoadDatasetYAML(t *testing.T) {
	reg := newDefaultRegistry[string, string, map[string]any]()
	data := []byte(`name: yds
cases:
  - name: c1
    inputs: in1
    metadata:
      k: v
    expected_output: out1
    evaluators:
      - EqualsExpected
`)
	ds, err := LoadDataset[string, string, map[string]any](data, reg, LoadOptions[string, string, map[string]any]{})
	if err != nil {
		t.Fatalf("LoadDataset: %v", err)
	}
	if ds.Name != "yds" {
		t.Fatalf("name = %q", ds.Name)
	}
	if len(ds.Cases) != 1 {
		t.Fatalf("cases = %d", len(ds.Cases))
	}
	c := ds.Cases[0]
	if c.Name != "c1" || c.Inputs != "in1" || c.ExpectedOutput != "out1" {
		t.Fatalf("case = %#v", c)
	}
	if !c.HasMetadata || c.Metadata["k"] != "v" {
		t.Fatalf("metadata = %#v has=%v", c.Metadata, c.HasMetadata)
	}
	if !c.HasExpectedOutput {
		t.Fatal("expected HasExpectedOutput")
	}
	if len(c.Evaluators) != 1 {
		t.Fatalf("case evaluators = %d", len(c.Evaluators))
	}
}

func TestLoadDatasetJSONInferredFormat(t *testing.T) {
	reg := newDefaultRegistry[string, string, map[string]any]()
	data := []byte(`{"name":"jds","cases":[{"name":"c1","inputs":"in1"}]}`)
	ds, err := LoadDataset[string, string, map[string]any](data, reg, LoadOptions[string, string, map[string]any]{})
	if err != nil {
		t.Fatalf("LoadDataset: %v", err)
	}
	if ds.Name != "jds" || len(ds.Cases) != 1 || ds.Cases[0].Inputs != "in1" {
		t.Fatalf("ds = %#v", ds)
	}
}

func TestLoadDatasetJSONExplicitFormat(t *testing.T) {
	reg := newDefaultRegistry[string, string, map[string]any]()
	data := []byte(`{"name":"jds","cases":[{"name":"c1","inputs":"in1"}]}`)
	ds, err := LoadDataset[string, string, map[string]any](data, reg, LoadOptions[string, string, map[string]any]{Format: "json"})
	if err != nil {
		t.Fatalf("LoadDataset: %v", err)
	}
	if ds.Name != "jds" {
		t.Fatalf("name = %q", ds.Name)
	}
}

func TestLoadDatasetDefaultName(t *testing.T) {
	reg := newDefaultRegistry[string, string, map[string]any]()
	data := []byte(`cases:
  - inputs: in1
`)
	ds, err := LoadDataset[string, string, map[string]any](data, reg, LoadOptions[string, string, map[string]any]{DefaultName: "fallback"})
	if err != nil {
		t.Fatalf("LoadDataset: %v", err)
	}
	if ds.Name != "fallback" {
		t.Fatalf("name = %q want fallback", ds.Name)
	}
}

func TestLoadDatasetMissingNameError(t *testing.T) {
	reg := newDefaultRegistry[string, string, map[string]any]()
	data := []byte(`cases:
  - inputs: in1
`)
	_, err := LoadDataset[string, string, map[string]any](data, reg, LoadOptions[string, string, map[string]any]{})
	if err == nil {
		t.Fatal("expected missing name error")
	}
	want := "dataset name is required: provide one in the data or via DefaultName"
	if err.Error() != want {
		t.Fatalf("error = %q want %q", err.Error(), want)
	}
}

func TestLoadDatasetUnknownFormat(t *testing.T) {
	reg := newDefaultRegistry[string, string, map[string]any]()
	_, err := LoadDataset[string, string, map[string]any]([]byte("name: x"), reg, LoadOptions[string, string, map[string]any]{Format: "toml"})
	if err == nil {
		t.Fatal("expected unknown format error")
	}
	want := `unknown format "toml" (want "yaml" or "json")`
	if err.Error() != want {
		t.Fatalf("error = %q want %q", err.Error(), want)
	}
}

func TestLoadDatasetMalformedYAML(t *testing.T) {
	reg := newDefaultRegistry[string, string, map[string]any]()
	_, err := LoadDataset[string, string, map[string]any]([]byte("name: x\n\tbad: : :"), reg, LoadOptions[string, string, map[string]any]{Format: "yaml"})
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.HasPrefix(err.Error(), "parsing dataset YAML: ") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestLoadDatasetMalformedJSON(t *testing.T) {
	reg := newDefaultRegistry[string, string, map[string]any]()
	_, err := LoadDataset[string, string, map[string]any]([]byte(`{"name": `), reg, LoadOptions[string, string, map[string]any]{Format: "json"})
	if err == nil {
		t.Fatal("expected parse error")
	}
	if !strings.HasPrefix(err.Error(), "parsing dataset JSON: ") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestLoadDatasetUnregisteredEvaluator(t *testing.T) {
	reg := NewRegistry[string, string, map[string]any]()
	data := []byte(`name: x
cases:
  - inputs: in1
    evaluators:
      - Mystery
`)
	_, err := LoadDataset[string, string, map[string]any](data, reg, LoadOptions[string, string, map[string]any]{})
	if err == nil {
		t.Fatal("expected unregistered evaluator error")
	}
	if !strings.Contains(err.Error(), `evaluator "Mystery" is not registered`) {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestLoadDatasetUnregisteredDatasetEvaluator(t *testing.T) {
	reg := NewRegistry[string, string, map[string]any]()
	data := []byte(`name: x
cases:
  - inputs: in1
evaluators:
  - Mystery
`)
	_, err := LoadDataset[string, string, map[string]any](data, reg, LoadOptions[string, string, map[string]any]{})
	if err == nil {
		t.Fatal("expected unregistered dataset evaluator error")
	}
	if !strings.Contains(err.Error(), `evaluator "Mystery" is not registered`) {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestLoadDatasetUnregisteredListsValidChoices(t *testing.T) {
	reg := newDefaultRegistry[string, string, map[string]any]()
	data := []byte(`name: x
cases:
  - inputs: in1
    evaluators:
      - Mystery
`)
	_, err := LoadDataset[string, string, map[string]any](data, reg, LoadOptions[string, string, map[string]any]{})
	if err == nil {
		t.Fatal("expected unregistered evaluator error")
	}
	for _, choice := range []string{"Equals", "EqualsExpected", "Contains", "IsInstance", "MaxDuration"} {
		if !strings.Contains(err.Error(), choice) {
			t.Fatalf("error %q missing valid choice %q", err.Error(), choice)
		}
	}
}

func TestLoadDatasetDatasetLevelEvaluator(t *testing.T) {
	reg := newDefaultRegistry[string, string, map[string]any]()
	data := []byte(`name: x
cases:
  - name: c1
    inputs: in1
evaluators:
  - IsInstance: string
`)
	ds, err := LoadDataset[string, string, map[string]any](data, reg, LoadOptions[string, string, map[string]any]{})
	if err != nil {
		t.Fatalf("LoadDataset: %v", err)
	}
	if len(ds.Evaluators) != 1 {
		t.Fatalf("dataset evaluators = %d want 1", len(ds.Evaluators))
	}
	rep, err := ds.Evaluate(context.Background(), func(_ context.Context, _ string) (string, error) {
		return "x", nil
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	if v := assertionValue(t, rep.Cases[0], "IsInstance"); v != Bool(true) {
		t.Fatalf("IsInstance = %v want True", v)
	}
}

func TestLoadDatasetDatasetEvaluatorParseError(t *testing.T) {
	reg := newDefaultRegistry[string, string, map[string]any]()
	data := []byte(`name: x
cases:
  - inputs: in1
evaluators:
  - 42
`)
	_, err := LoadDataset[string, string, map[string]any](data, reg, LoadOptions[string, string, map[string]any]{})
	if err == nil {
		t.Fatal("expected parse error for numeric dataset evaluator")
	}
	if !strings.Contains(err.Error(), "invalid evaluator spec") {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestParseSpecInvalidScalarEvaluator(t *testing.T) {
	reg := newDefaultRegistry[string, string, map[string]any]()
	data := []byte(`name: x
cases:
  - inputs: in1
    evaluators:
      - 42
`)
	_, err := LoadDataset[string, string, map[string]any](data, reg, LoadOptions[string, string, map[string]any]{})
	if err == nil {
		t.Fatal("expected parse error for numeric evaluator")
	}
	want := "invalid evaluator spec: expected string or single-key mapping, got int"
	if err.Error() != want {
		t.Fatalf("error = %q want %q", err.Error(), want)
	}
}

func TestLoadDatasetDefaultDecoderUnmarshalError(t *testing.T) {
	reg := newDefaultRegistry[structIn, structOut, structMeta]()
	data := []byte(`name: ds
cases:
  - name: c1
    inputs: not-a-struct
`)
	_, err := LoadDataset[structIn, structOut, structMeta](data, reg, LoadOptions[structIn, structOut, structMeta]{})
	if err == nil {
		t.Fatal("expected default-decoder unmarshal error")
	}
	if !strings.Contains(err.Error(), `case "c1" inputs:`) {
		t.Fatalf("error = %q", err.Error())
	}
}

func TestLoadDatasetNonStringMapKeysNormalized(t *testing.T) {
	reg := newDefaultRegistry[string, string, map[string]any]()
	data := []byte(`name: ds
cases:
  - name: c1
    inputs: in1
    metadata:
      1: a
      2: b
`)
	ds, err := LoadDataset[string, string, map[string]any](data, reg, LoadOptions[string, string, map[string]any]{})
	if err != nil {
		t.Fatalf("LoadDataset: %v", err)
	}
	c := ds.Cases[0]
	if !c.HasMetadata {
		t.Fatal("expected metadata")
	}
	if c.Metadata["1"] != "a" || c.Metadata["2"] != "b" {
		t.Fatalf("metadata = %#v want normalized string keys", c.Metadata)
	}
}

func TestParseSpecForms(t *testing.T) {
	tests := []struct {
		name       string
		yaml       string
		wantName   string
		wantArgs   []any
		wantKwargs map[string]any
	}{
		{
			name:     "bare string",
			yaml:     "Custom",
			wantName: "Custom",
		},
		{
			name:     "single key scalar positional",
			yaml:     "Custom: 42",
			wantName: "Custom",
			wantArgs: []any{42},
		},
		{
			name:     "single key list positional",
			yaml:     "Custom: [a, b]",
			wantName: "Custom",
			wantArgs: []any{[]any{"a", "b"}},
		},
		{
			name:       "single key string-map kwargs",
			yaml:       "Custom: {x: 1, y: hi}",
			wantName:   "Custom",
			wantKwargs: map[string]any{"x": 1, "y": "hi"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var captured EvaluatorSpec
			reg := NewRegistry[string, string, map[string]any]()
			reg.Register("Custom", func(spec EvaluatorSpec) (Evaluator[string, string, map[string]any], error) {
				captured = spec
				return specEvaluator{spec: spec}, nil
			})
			data := []byte("name: ds\ncases:\n  - inputs: in1\n    evaluators:\n      - " + tt.yaml + "\n")
			_, err := LoadDataset[string, string, map[string]any](data, reg, LoadOptions[string, string, map[string]any]{})
			if err != nil {
				t.Fatalf("LoadDataset: %v", err)
			}
			if captured.Name != tt.wantName {
				t.Fatalf("name = %q want %q", captured.Name, tt.wantName)
			}
			if len(captured.Args) != len(tt.wantArgs) {
				t.Fatalf("args = %#v want %#v", captured.Args, tt.wantArgs)
			}
			for i := range tt.wantArgs {
				if !equalAny(captured.Args[i], tt.wantArgs[i]) {
					t.Fatalf("arg[%d] = %#v want %#v", i, captured.Args[i], tt.wantArgs[i])
				}
			}
			if len(captured.Kwargs) != len(tt.wantKwargs) {
				t.Fatalf("kwargs = %#v want %#v", captured.Kwargs, tt.wantKwargs)
			}
			for k, v := range tt.wantKwargs {
				if !equalAny(captured.Kwargs[k], v) {
					t.Fatalf("kwargs[%q] = %#v want %#v", k, captured.Kwargs[k], v)
				}
			}
		})
	}
}

func TestParseSpecMultiKeyMapError(t *testing.T) {
	reg := NewRegistry[string, string, map[string]any]()
	reg.Register("Custom", func(spec EvaluatorSpec) (Evaluator[string, string, map[string]any], error) {
		return specEvaluator{spec: spec}, nil
	})
	data := []byte(`name: ds
cases:
  - inputs: in1
    evaluators:
      - {A: 1, B: 2}
`)
	_, err := LoadDataset[string, string, map[string]any](data, reg, LoadOptions[string, string, map[string]any]{})
	if err == nil {
		t.Fatal("expected multi-key spec error")
	}
	want := "expected a single key containing the evaluator name, found keys [A B]"
	if err.Error() != want {
		t.Fatalf("error = %q want %q", err.Error(), want)
	}
}

func TestRegisterCustomEvaluatorRoundTrip(t *testing.T) {
	reg := NewRegistry[string, string, map[string]any]()
	reg.Register("StartsWith", func(spec EvaluatorSpec) (Evaluator[string, string, map[string]any], error) {
		prefix, _ := spec.Args[0].(string)
		return startsWith{prefix: prefix}, nil
	})
	data := []byte(`name: ds
cases:
  - name: c1
    inputs: in1
    evaluators:
      - StartsWith: hel
`)
	ds, err := LoadDataset[string, string, map[string]any](data, reg, LoadOptions[string, string, map[string]any]{})
	if err != nil {
		t.Fatalf("LoadDataset: %v", err)
	}
	rep, err := ds.Evaluate(context.Background(), func(_ context.Context, in string) (string, error) {
		return "hello", nil
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	got := assertionValue(t, rep.Cases[0], "StartsWith")
	if got != Bool(true) {
		t.Fatalf("StartsWith = %v want True", got)
	}
}

func TestBuiltinRoundTrips(t *testing.T) {
	tests := []struct {
		name      string
		evaluator string
		input     string
		output    string
		expected  string
		resName   string
		wantValue Scalar
	}{
		{
			name:      "Equals positional match",
			evaluator: "Equals: hello",
			output:    "hello",
			resName:   "Equals",
			wantValue: Bool(true),
		},
		{
			name:      "Equals positional mismatch",
			evaluator: "Equals: hello",
			output:    "world",
			resName:   "Equals",
			wantValue: Bool(false),
		},
		{
			name:      "Equals kwargs",
			evaluator: "Equals: {value: hello, evaluation_name: eq}",
			output:    "hello",
			resName:   "eq",
			wantValue: Bool(true),
		},
		{
			name:      "EqualsExpected positional name",
			evaluator: "EqualsExpected: ee",
			output:    "x",
			expected:  "x",
			resName:   "ee",
			wantValue: Bool(true),
		},
		{
			name:      "EqualsExpected kwargs",
			evaluator: "EqualsExpected: {evaluation_name: ee2}",
			output:    "x",
			expected:  "x",
			resName:   "ee2",
			wantValue: Bool(true),
		},
		{
			name:      "Contains positional",
			evaluator: "Contains: ell",
			output:    "hello",
			resName:   "Contains",
			wantValue: Bool(true),
		},
		{
			name:      "Contains kwargs",
			evaluator: "Contains: {value: ELL, case_sensitive: false, evaluation_name: ct}",
			output:    "hello",
			resName:   "ct",
			wantValue: Bool(true),
		},
		{
			name:      "IsInstance positional",
			evaluator: "IsInstance: string",
			output:    "x",
			resName:   "IsInstance",
			wantValue: Bool(true),
		},
		{
			name:      "IsInstance kwargs",
			evaluator: "IsInstance: {type_name: string, evaluation_name: ii}",
			output:    "x",
			resName:   "ii",
			wantValue: Bool(true),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := newDefaultRegistry[string, string, map[string]any]()
			var b strings.Builder
			b.WriteString("name: ds\ncases:\n  - name: c1\n    inputs: in\n")
			if tt.expected != "" {
				b.WriteString("    expected_output: " + tt.expected + "\n")
			}
			b.WriteString("    evaluators:\n      - " + tt.evaluator + "\n")
			ds, err := LoadDataset[string, string, map[string]any]([]byte(b.String()), reg, LoadOptions[string, string, map[string]any]{})
			if err != nil {
				t.Fatalf("LoadDataset: %v", err)
			}
			out := tt.output
			rep, err := ds.Evaluate(context.Background(), func(_ context.Context, _ string) (string, error) {
				return out, nil
			})
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			got := assertionValue(t, rep.Cases[0], tt.resName)
			if got != tt.wantValue {
				t.Fatalf("%s = %v want %v", tt.resName, got, tt.wantValue)
			}
		})
	}
}

func TestMaxDurationRoundTrips(t *testing.T) {
	tests := []struct {
		name      string
		evaluator string
		wantValue Scalar
	}{
		{
			name:      "positional generous",
			evaluator: "MaxDuration: 3600",
			wantValue: Bool(true),
		},
		{
			name:      "positional impossible",
			evaluator: "MaxDuration: 0",
			wantValue: Bool(false),
		},
		{
			name:      "kwargs seconds",
			evaluator: "MaxDuration:\n          seconds: 3600",
			wantValue: Bool(true),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := newDefaultRegistry[string, string, map[string]any]()
			data := []byte("name: ds\ncases:\n  - name: c1\n    inputs: in\n    evaluators:\n      - " + tt.evaluator + "\n")
			ds, err := LoadDataset[string, string, map[string]any](data, reg, LoadOptions[string, string, map[string]any]{})
			if err != nil {
				t.Fatalf("LoadDataset: %v", err)
			}
			rep, err := ds.Evaluate(context.Background(), func(_ context.Context, _ string) (string, error) {
				time.Sleep(time.Millisecond)
				return "x", nil
			})
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			got := assertionValue(t, rep.Cases[0], "MaxDuration")
			if got != tt.wantValue {
				t.Fatalf("MaxDuration = %v want %v", got, tt.wantValue)
			}
		})
	}
}

func TestFactoryInstantiateErrors(t *testing.T) {
	tests := []struct {
		name      string
		evaluator string
		wantSub   string
	}{
		{
			name:      "IsInstance no type_name",
			evaluator: "IsInstance: {evaluation_name: x}",
			wantSub:   "IsInstance requires a type_name",
		},
		{
			name:      "IsInstance type_name not a string",
			evaluator: "IsInstance: 42",
			wantSub:   "IsInstance type_name must be a string",
		},
		{
			name:      "MaxDuration no seconds",
			evaluator: "MaxDuration: {other: 1}",
			wantSub:   "MaxDuration requires seconds",
		},
		{
			name:      "MaxDuration non-numeric seconds",
			evaluator: "MaxDuration: abc",
			wantSub:   "MaxDuration seconds: expected a number",
		},
		{
			name:      "MaxDuration kwargs non-numeric seconds",
			evaluator: "MaxDuration: {seconds: abc}",
			wantSub:   "MaxDuration seconds: expected a number",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := newDefaultRegistry[string, string, map[string]any]()
			data := []byte("name: ds\ncases:\n  - name: c1\n    inputs: in\n    evaluators:\n      - " + tt.evaluator + "\n")
			_, err := LoadDataset[string, string, map[string]any](data, reg, LoadOptions[string, string, map[string]any]{})
			if err == nil {
				t.Fatalf("expected error for %q", tt.evaluator)
			}
			if !strings.Contains(err.Error(), tt.wantSub) {
				t.Fatalf("error = %q want substring %q", err.Error(), tt.wantSub)
			}
			if !strings.Contains(err.Error(), "failed to instantiate evaluator") {
				t.Fatalf("error = %q missing instantiate prefix", err.Error())
			}
			if !strings.Contains(err.Error(), `for case "c1"`) {
				t.Fatalf("error = %q missing case context", err.Error())
			}
		})
	}
}

type structIn struct {
	City string `json:"city"`
}

type structOut struct {
	Answer string `json:"answer"`
	Score  int    `json:"score"`
}

type structMeta struct {
	Difficulty string `json:"difficulty"`
}

func TestLoadDatasetStructDefaultDecoder(t *testing.T) {
	cases := map[string][]byte{
		"yaml": []byte(`name: structds
cases:
  - name: c1
    inputs:
      city: Paris
    metadata:
      difficulty: easy
    expected_output:
      answer: Paris
      score: 10
    evaluators:
      - EqualsExpected
`),
		"json": []byte(`{"name":"structds","cases":[{"name":"c1","inputs":{"city":"Paris"},"metadata":{"difficulty":"easy"},"expected_output":{"answer":"Paris","score":10},"evaluators":["EqualsExpected"]}]}`),
	}
	for format, data := range cases {
		t.Run(format, func(t *testing.T) {
			reg := newDefaultRegistry[structIn, structOut, structMeta]()
			ds, err := LoadDataset[structIn, structOut, structMeta](data, reg, LoadOptions[structIn, structOut, structMeta]{})
			if err != nil {
				t.Fatalf("LoadDataset: %v", err)
			}
			c := ds.Cases[0]
			if c.Inputs != (structIn{City: "Paris"}) {
				t.Fatalf("inputs = %#v", c.Inputs)
			}
			if !c.HasMetadata || c.Metadata != (structMeta{Difficulty: "easy"}) {
				t.Fatalf("metadata = %#v has=%v", c.Metadata, c.HasMetadata)
			}
			if !c.HasExpectedOutput || c.ExpectedOutput != (structOut{Answer: "Paris", Score: 10}) {
				t.Fatalf("expected = %#v has=%v", c.ExpectedOutput, c.HasExpectedOutput)
			}

			rep, err := ds.Evaluate(context.Background(), func(_ context.Context, in structIn) (structOut, error) {
				return structOut{Answer: in.City, Score: 10}, nil
			})
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			got := assertionValue(t, rep.Cases[0], "EqualsExpected")
			if got != Bool(true) {
				t.Fatalf("EqualsExpected = %v want True", got)
			}
		})
	}
}

func TestLoadDatasetCustomDecodeHooks(t *testing.T) {
	reg := newDefaultRegistry[structIn, structOut, structMeta]()
	data := []byte(`name: ds
cases:
  - name: c1
    inputs:
      raw: paris
    metadata:
      raw: easy
    expected_output:
      raw: PARIS
`)
	opts := LoadOptions[structIn, structOut, structMeta]{
		DecodeInputs: func(v any) (structIn, error) {
			m := v.(map[string]any)
			return structIn{City: m["raw"].(string)}, nil
		},
		DecodeMetadata: func(v any) (structMeta, error) {
			m := v.(map[string]any)
			return structMeta{Difficulty: m["raw"].(string)}, nil
		},
		DecodeOutput: func(v any) (structOut, error) {
			m := v.(map[string]any)
			return structOut{Answer: m["raw"].(string)}, nil
		},
	}
	ds, err := LoadDataset[structIn, structOut, structMeta](data, reg, opts)
	if err != nil {
		t.Fatalf("LoadDataset: %v", err)
	}
	c := ds.Cases[0]
	if c.Inputs != (structIn{City: "paris"}) {
		t.Fatalf("inputs = %#v", c.Inputs)
	}
	if c.Metadata != (structMeta{Difficulty: "easy"}) {
		t.Fatalf("metadata = %#v", c.Metadata)
	}
	if c.ExpectedOutput != (structOut{Answer: "PARIS"}) {
		t.Fatalf("expected = %#v", c.ExpectedOutput)
	}
}

func TestLoadDatasetDecodeHookError(t *testing.T) {
	reg := newDefaultRegistry[structIn, structOut, structMeta]()
	tests := []struct {
		name string
		opts LoadOptions[structIn, structOut, structMeta]
		data string
		sub  string
	}{
		{
			name: "inputs",
			data: "name: ds\ncases:\n  - name: c1\n    inputs: x\n",
			opts: LoadOptions[structIn, structOut, structMeta]{
				DecodeInputs: func(any) (structIn, error) { return structIn{}, errBoom },
			},
			sub: `case "c1" inputs:`,
		},
		{
			name: "metadata",
			data: "name: ds\ncases:\n  - name: c1\n    inputs: x\n    metadata: y\n",
			opts: LoadOptions[structIn, structOut, structMeta]{
				DecodeInputs:   func(any) (structIn, error) { return structIn{}, nil },
				DecodeMetadata: func(any) (structMeta, error) { return structMeta{}, errBoom },
			},
			sub: `case "c1" metadata:`,
		},
		{
			name: "expected_output",
			data: "name: ds\ncases:\n  - name: c1\n    inputs: x\n    expected_output: y\n",
			opts: LoadOptions[structIn, structOut, structMeta]{
				DecodeInputs: func(any) (structIn, error) { return structIn{}, nil },
				DecodeOutput: func(any) (structOut, error) { return structOut{}, errBoom },
			},
			sub: `case "c1" expected_output:`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadDataset[structIn, structOut, structMeta]([]byte(tt.data), reg, tt.opts)
			if err == nil {
				t.Fatal("expected decode error")
			}
			if !strings.Contains(err.Error(), tt.sub) {
				t.Fatalf("error = %q want substring %q", err.Error(), tt.sub)
			}
		})
	}
}

func TestLoadDatasetMetadataAndExpectedAbsent(t *testing.T) {
	reg := newDefaultRegistry[string, string, map[string]any]()
	data := []byte(`name: ds
cases:
  - name: c1
    inputs: in1
`)
	ds, err := LoadDataset[string, string, map[string]any](data, reg, LoadOptions[string, string, map[string]any]{})
	if err != nil {
		t.Fatalf("LoadDataset: %v", err)
	}
	c := ds.Cases[0]
	if c.HasMetadata {
		t.Fatal("expected no metadata")
	}
	if c.HasExpectedOutput {
		t.Fatal("expected no expected_output")
	}
}

func TestSaveLoadRoundTripAllBuiltins(t *testing.T) {
	reg := newDefaultRegistry[string, string, map[string]any]()
	d, err := NewDataset[string, string, map[string]any]("rt", []Case[string, string, map[string]any]{
		NewCase[string, string, map[string]any]("hello",
			WithCaseName[string, string, map[string]any]("c1"),
			WithExpectedOutput[string, string, map[string]any]("hello"),
			WithCaseEvaluators[string, string, map[string]any](
				Equals[string, string, map[string]any]{Value: "hello"},
				EqualsExpected[string, string, map[string]any]{},
				Contains[string, string, map[string]any]{Value: "ell"},
				IsInstance[string, string, map[string]any]{TypeName: "string"},
				MaxDuration[string, string, map[string]any]{Max: time.Hour},
			),
		),
	})
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}

	for _, format := range []string{"yaml", "json"} {
		t.Run(format, func(t *testing.T) {
			data, err := d.Save(SaveOptions{Format: format})
			if err != nil {
				t.Fatalf("Save: %v", err)
			}
			ds, err := LoadDataset[string, string, map[string]any](data, reg, LoadOptions[string, string, map[string]any]{Format: format})
			if err != nil {
				t.Fatalf("LoadDataset: %v", err)
			}
			rep, err := ds.Evaluate(context.Background(), func(_ context.Context, in string) (string, error) {
				return in, nil
			})
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			rc := rep.Cases[0]
			for _, n := range []string{"Equals", "EqualsExpected", "Contains", "IsInstance", "MaxDuration"} {
				if v := assertionValue(t, rc, n); v != Bool(true) {
					t.Fatalf("%s = %v want True", n, v)
				}
			}
		})
	}
}

// --- helpers ---

var errBoom = errBoomError("boom")

type errBoomError string

func (e errBoomError) Error() string { return string(e) }

func assertionValue[I, O, M any](t *testing.T, rc ReportCase[I, O, M], name string) Scalar {
	t.Helper()
	r, ok := rc.Assertions[name]
	if !ok {
		t.Fatalf("assertion %q not found in %#v", name, rc.Assertions)
	}
	return r.Value
}

func equalAny(a, b any) bool {
	switch av := a.(type) {
	case []any:
		bv, ok := b.([]any)
		if !ok || len(av) != len(bv) {
			return false
		}
		for i := range av {
			if !equalAny(av[i], bv[i]) {
				return false
			}
		}
		return true
	default:
		return a == b
	}
}

type specEvaluator struct {
	spec EvaluatorSpec
}

func (s specEvaluator) Evaluate(_ context.Context, _ *EvaluatorContext[string, string, map[string]any]) (EvaluatorOutput, error) {
	return ScalarValue(Bool(true)), nil
}

func (s specEvaluator) Spec() EvaluatorSpec { return s.spec }

type startsWith struct {
	prefix string
}

func (s startsWith) Evaluate(_ context.Context, ec *EvaluatorContext[string, string, map[string]any]) (EvaluatorOutput, error) {
	return ScalarValue(Bool(strings.HasPrefix(ec.Output, s.prefix))), nil
}

func (s startsWith) Spec() EvaluatorSpec { return NewSpecArg("StartsWith", s.prefix) }
