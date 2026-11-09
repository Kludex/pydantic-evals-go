package evals

import (
	"context"
	"strings"
	"testing"
	"time"
)

// --- helpers -----------------------------------------------------------------

func mustSave(t *testing.T, ds *Dataset[string, string, string], opts SaveOptions) string {
	t.Helper()
	data, err := ds.Save(opts)
	if err != nil {
		t.Fatalf("Save(%+v) returned error: %v", opts, err)
	}
	return string(data)
}

func defaultStringRegistry() *Registry[string, string, string] {
	reg := NewRegistry[string, string, string]()
	reg.RegisterDefaults()
	return reg
}

func loadString(t *testing.T, data string, opts LoadOptions[string, string, string]) *Dataset[string, string, string] {
	t.Helper()
	ds, err := LoadDataset([]byte(data), defaultStringRegistry(), opts)
	if err != nil {
		t.Fatalf("LoadDataset returned error: %v", err)
	}
	return ds
}

func loadStringErr(data string, opts LoadOptions[string, string, string]) error {
	_, err := LoadDataset([]byte(data), defaultStringRegistry(), opts)
	return err
}

func assertContains(t *testing.T, got, want string) {
	t.Helper()
	if !strings.Contains(got, want) {
		t.Fatalf("expected %q to contain %q", got, want)
	}
}

// --- a dataset exercising every short-form an evaluator can take -------------

func allBuiltinsDataset(t *testing.T) *Dataset[string, string, string] {
	t.Helper()
	ds, err := NewDataset(
		"greet",
		[]Case[string, string, string]{
			NewCase[string, string, string](
				"hi",
				WithCaseName[string, string, string]("c1"),
				WithExpectedOutput[string, string, string]("HI"),
				WithMetadata[string, string, string]("m1"),
				WithCaseEvaluators[string, string, string](
					Equals[string, string, string]{Value: "HI"},
					EqualsExpected[string, string, string]{},
					Contains[string, string, string]{Value: "H"},
					IsInstance[string, string, string]{TypeName: "string"},
					MaxDuration[string, string, string]{Max: 1500 * time.Millisecond},
				),
			),
		},
		EqualsExpected[string, string, string]{},
	)
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}
	return ds
}

func TestSaveYAMLDefault(t *testing.T) {
	ds := allBuiltinsDataset(t)
	const want = `name: greet
cases:
  - name: c1
    inputs: hi
    metadata: m1
    expected_output: HI
    evaluators:
      - Equals:
          value: HI
      - EqualsExpected
      - Contains:
          value: H
      - IsInstance: string
      - MaxDuration: 1.5
evaluators:
  - EqualsExpected
`
	if got := mustSave(t, ds, SaveOptions{}); got != want {
		t.Fatalf("default YAML mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
	// Format: "yaml" is identical to the default.
	if got := mustSave(t, ds, SaveOptions{Format: "yaml"}); got != want {
		t.Fatalf("explicit yaml mismatch:\n--- got ---\n%s", got)
	}
}

func TestSaveYAMLWithSchema(t *testing.T) {
	ds := allBuiltinsDataset(t)
	const want = `# yaml-language-server: $schema=./schema.json
$schema: ./schema.json
name: greet
cases:
  - name: c1
    inputs: hi
    metadata: m1
    expected_output: HI
    evaluators:
      - Equals:
          value: HI
      - EqualsExpected
      - Contains:
          value: H
      - IsInstance: string
      - MaxDuration: 1.5
evaluators:
  - EqualsExpected
`
	if got := mustSave(t, ds, SaveOptions{Format: "yaml", Schema: "./schema.json"}); got != want {
		t.Fatalf("schema YAML mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestSaveJSON(t *testing.T) {
	ds := allBuiltinsDataset(t)
	const want = `{
  "name": "greet",
  "cases": [
    {
      "name": "c1",
      "inputs": "hi",
      "metadata": "m1",
      "expected_output": "HI",
      "evaluators": [
        {
          "Equals": {
            "value": "HI"
          }
        },
        "EqualsExpected",
        {
          "Contains": {
            "value": "H"
          }
        },
        {
          "IsInstance": "string"
        },
        {
          "MaxDuration": 1.5
        }
      ]
    }
  ],
  "evaluators": [
    "EqualsExpected"
  ]
}
`
	if got := mustSave(t, ds, SaveOptions{Format: "json"}); got != want {
		t.Fatalf("JSON mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestSaveJSONWithSchema(t *testing.T) {
	ds := allBuiltinsDataset(t)
	const want = `{
  "$schema": "./schema.json",
  "name": "greet",
  "cases": [
    {
      "name": "c1",
      "inputs": "hi",
      "metadata": "m1",
      "expected_output": "HI",
      "evaluators": [
        {
          "Equals": {
            "value": "HI"
          }
        },
        "EqualsExpected",
        {
          "Contains": {
            "value": "H"
          }
        },
        {
          "IsInstance": "string"
        },
        {
          "MaxDuration": 1.5
        }
      ]
    }
  ],
  "evaluators": [
    "EqualsExpected"
  ]
}
`
	if got := mustSave(t, ds, SaveOptions{Format: "json", Schema: "./schema.json"}); got != want {
		t.Fatalf("JSON schema mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

func TestSaveUnknownFormat(t *testing.T) {
	ds := allBuiltinsDataset(t)
	_, err := ds.Save(SaveOptions{Format: "toml"})
	if err == nil {
		t.Fatal("expected error for unknown format")
	}
	if got := err.Error(); got != `unknown format "toml" (want "yaml" or "json")` {
		t.Fatalf("unexpected error: %q", got)
	}
}

// --- evaluator spec short forms, including the named (kwargs) variants -------

func TestSaveEvaluatorKwargsAndNamedForms(t *testing.T) {
	ds, err := NewDataset[string, string, string](
		"d",
		[]Case[string, string, string]{NewCase[string, string, string]("hi")},
		Equals[string, string, string]{Value: "v", Name: "eqname"},
		EqualsExpected[string, string, string]{Name: "eename"},
		Contains[string, string, string]{Value: "x", CaseSensitive: true, AsStrings: true, Name: "myc"},
		IsInstance[string, string, string]{TypeName: "string", Name: "isname"},
	)
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}
	const want = `name: d
cases:
  - inputs: hi
evaluators:
  - Equals:
      evaluation_name: eqname
      value: v
  - EqualsExpected:
      evaluation_name: eename
  - Contains:
      as_strings: true
      case_sensitive: true
      evaluation_name: myc
      value: x
  - IsInstance:
      evaluation_name: isname
      type_name: string
`
	if got := mustSave(t, ds, SaveOptions{}); got != want {
		t.Fatalf("kwargs YAML mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// A custom evaluator without Spec() serializes as just its Go type name.
type customNoSpec struct{}

func (customNoSpec) Evaluate(_ context.Context, _ *EvaluatorContext[string, string, string]) (Output, error) {
	return Assertion(true), nil
}

func TestSaveEvaluatorWithoutSpec(t *testing.T) {
	ds, err := NewDataset[string, string, string](
		"d",
		[]Case[string, string, string]{NewCase[string, string, string]("hi")},
		customNoSpec{},
	)
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}
	const want = `name: d
cases:
  - inputs: hi
evaluators:
  - customNoSpec
`
	if got := mustSave(t, ds, SaveOptions{}); got != want {
		t.Fatalf("no-spec YAML mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// --- spec constructors observed via Save -------------------------------------

type specConstructorEval struct {
	spec EvaluatorSpec
}

func (e specConstructorEval) Evaluate(_ context.Context, _ *EvaluatorContext[string, string, string]) (Output, error) {
	return Assertion(true), nil
}
func (e specConstructorEval) Spec() EvaluatorSpec { return e.spec }

func TestSaveSpecConstructors(t *testing.T) {
	ds, err := NewDataset[string, string, string](
		"d",
		[]Case[string, string, string]{NewCase[string, string, string]("hi")},
		specConstructorEval{spec: NewSpec("Bare")},
		specConstructorEval{spec: NewSpecArg("Arg", 7)},
		specConstructorEval{spec: NewSpecKwargs("Kw", map[string]any{"k": "v"})},
	)
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}
	const want = `name: d
cases:
  - inputs: hi
evaluators:
  - Bare
  - Arg: 7
  - Kw:
      k: v
`
	if got := mustSave(t, ds, SaveOptions{}); got != want {
		t.Fatalf("spec constructor YAML mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// --- Save marshal-error paths (a value YAML/JSON cannot encode) --------------

type chanInput struct {
	Ch chan int
}

func channelDataset(t *testing.T) *Dataset[chanInput, string, string] {
	t.Helper()
	ds, err := NewDataset[chanInput, string, string](
		"d",
		[]Case[chanInput, string, string]{
			NewCase[chanInput, string, string](chanInput{Ch: make(chan int)}),
		},
	)
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}
	return ds
}

func TestSaveYAMLMarshalError(t *testing.T) {
	ds := channelDataset(t)
	_, err := ds.Save(SaveOptions{Format: "yaml"})
	if err == nil {
		t.Fatal("expected YAML marshal error for channel value")
	}
	assertContains(t, err.Error(), "encoding dataset YAML")
}

func TestSaveJSONMarshalError(t *testing.T) {
	ds := channelDataset(t)
	_, err := ds.Save(SaveOptions{Format: "json"})
	if err == nil {
		t.Fatal("expected JSON marshal error for channel value")
	}
	assertContains(t, err.Error(), "unsupported type: chan int")
}

// --- LoadDataset: format inference and explicit format -----------------------

func TestLoadFormatInference(t *testing.T) {
	yaml := "name: d\ncases:\n  - inputs: x\nevaluators:\n  - EqualsExpected\n"
	if ds := loadString(t, yaml, LoadOptions[string, string, string]{}); ds.Name != "d" || len(ds.Evaluators) != 1 {
		t.Fatalf("inferred yaml load wrong: %+v", ds)
	}

	json := `{"name":"jd","cases":[{"inputs":"x"}],"evaluators":["EqualsExpected"]}`
	if ds := loadString(t, json, LoadOptions[string, string, string]{}); ds.Name != "jd" || len(ds.Evaluators) != 1 {
		t.Fatalf("inferred json load wrong: %+v", ds)
	}

	// Leading whitespace before '{' must still infer JSON.
	if ds := loadString(t, "  "+json, LoadOptions[string, string, string]{}); ds.Name != "jd" {
		t.Fatalf("whitespace json load wrong: %+v", ds)
	}

	// Explicit json format on bytes that don't start with '{'.
	wrapped := loadString(t, json, LoadOptions[string, string, string]{Format: "json"})
	if wrapped.Name != "jd" {
		t.Fatalf("explicit json load wrong: %+v", wrapped)
	}
}

func TestLoadDefaultName(t *testing.T) {
	ds := loadString(t, "cases:\n  - inputs: x\n", LoadOptions[string, string, string]{DefaultName: "fallback"})
	if ds.Name != "fallback" {
		t.Fatalf("expected fallback name, got %q", ds.Name)
	}
	// A name in the data wins over DefaultName.
	ds = loadString(t, "name: real\ncases:\n  - inputs: x\n", LoadOptions[string, string, string]{DefaultName: "fallback"})
	if ds.Name != "real" {
		t.Fatalf("expected data name, got %q", ds.Name)
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		name string
		data string
		opts LoadOptions[string, string, string]
		want string
	}{
		{
			name: "missing name",
			data: "cases:\n  - inputs: x\n",
			want: "dataset name is required: provide one in the data or via DefaultName",
		},
		{
			name: "unknown explicit format",
			data: "name: d\ncases: []\n",
			opts: LoadOptions[string, string, string]{Format: "toml"},
			want: `unknown format "toml" (want "yaml" or "json")`,
		},
		{
			name: "malformed yaml",
			data: "name: d\ncases: [unterminated\n",
			want: "parsing dataset YAML:",
		},
		{
			name: "malformed json",
			data: "{not json",
			want: "parsing dataset JSON:",
		},
		{
			name: "unregistered evaluator",
			data: "name: d\ncases:\n  - inputs: x\nevaluators:\n  - Nope\n",
			want: `evaluator "Nope" is not registered`,
		},
		{
			name: "multi-key map spec",
			data: "name: d\ncases:\n  - inputs: x\nevaluators:\n  - {Equals: 1, Extra: 2}\n",
			want: "expected a single key containing the evaluator name, found keys [Equals Extra]",
		},
		{
			name: "invalid spec type",
			data: "name: d\ncases:\n  - inputs: x\nevaluators:\n  - [a, b]\n",
			want: "invalid evaluator spec: expected string or single-key mapping, got []interface {}",
		},
		{
			name: "duplicate case name",
			data: "name: d\ncases:\n  - name: dup\n    inputs: a\n  - name: dup\n    inputs: b\n",
			want: `duplicate case name: "dup"`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := loadStringErr(tt.data, tt.opts)
			if err == nil {
				t.Fatalf("expected error, got nil")
			}
			assertContains(t, err.Error(), tt.want)
		})
	}
}

// The unregistered-evaluator error lists the valid choices in non-deterministic
// (map iteration) order, so we assert membership rather than a fixed ordering.
func TestLoadUnregisteredListsChoices(t *testing.T) {
	err := loadStringErr("name: d\ncases:\n  - inputs: x\nevaluators:\n  - Nope\n", LoadOptions[string, string, string]{})
	if err == nil {
		t.Fatal("expected error")
	}
	msg := err.Error()
	for _, name := range []string{"Equals", "EqualsExpected", "Contains", "IsInstance", "MaxDuration"} {
		assertContains(t, msg, name)
	}
	assertContains(t, msg, "register it before loading")
}

// --- factory instantiate errors for the built-ins ----------------------------

func TestLoadFactoryInstantiateErrors(t *testing.T) {
	tests := []struct {
		name string
		data string
		want string
	}{
		{
			name: "isinstance no type_name",
			data: "name: d\ncases:\n  - inputs: x\nevaluators:\n  - IsInstance: {}\n",
			want: `failed to instantiate evaluator "IsInstance" for dataset: IsInstance requires a type_name`,
		},
		{
			name: "isinstance type_name not string",
			data: "name: d\ncases:\n  - inputs: x\nevaluators:\n  - IsInstance: 5\n",
			want: `failed to instantiate evaluator "IsInstance" for dataset: IsInstance type_name must be a string, got int`,
		},
		{
			name: "maxduration no seconds",
			data: "name: d\ncases:\n  - inputs: x\nevaluators:\n  - MaxDuration: {}\n",
			want: `failed to instantiate evaluator "MaxDuration" for dataset: MaxDuration requires seconds`,
		},
		{
			name: "maxduration non-numeric seconds",
			data: "name: d\ncases:\n  - inputs: x\nevaluators:\n  - MaxDuration: notanumber\n",
			want: `failed to instantiate evaluator "MaxDuration" for dataset: MaxDuration seconds: expected a number, got string`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := loadStringErr(tt.data, LoadOptions[string, string, string]{})
			if err == nil {
				t.Fatal("expected error")
			}
			assertContains(t, err.Error(), tt.want)
		})
	}
}

// The factory error message uses the per-case context when the failure happens
// on a case-level evaluator rather than a dataset-level one.
func TestLoadFactoryErrorCaseContext(t *testing.T) {
	data := "name: d\ncases:\n  - name: c1\n    inputs: x\n    evaluators:\n      - MaxDuration: {}\n"
	err := loadStringErr(data, LoadOptions[string, string, string]{})
	if err == nil {
		t.Fatal("expected error")
	}
	assertContains(t, err.Error(), `failed to instantiate evaluator "MaxDuration" for case "c1": MaxDuration requires seconds`)
}

// A malformed spec on a case-level evaluator (as opposed to a dataset-level one)
// surfaces the parse error too.
func TestLoadCaseLevelSpecParseError(t *testing.T) {
	data := "name: d\ncases:\n  - name: c1\n    inputs: x\n    evaluators:\n      - {Equals: 1, Extra: 2}\n"
	err := loadStringErr(data, LoadOptions[string, string, string]{})
	if err == nil {
		t.Fatal("expected error")
	}
	assertContains(t, err.Error(), "expected a single key containing the evaluator name, found keys [Equals Extra]")
}

// MaxDuration via kwargs rejects a non-numeric seconds value.
func TestLoadMaxDurationKwargsNonNumeric(t *testing.T) {
	data := "name: d\ncases:\n  - inputs: x\nevaluators:\n  - MaxDuration:\n      seconds: notanumber\n"
	err := loadStringErr(data, LoadOptions[string, string, string]{})
	if err == nil {
		t.Fatal("expected error")
	}
	assertContains(t, err.Error(), `failed to instantiate evaluator "MaxDuration" for dataset: MaxDuration seconds: expected a number, got string`)
}

// EqualsExpected carries its evaluation_name through the kwargs form on load.
func TestLoadEqualsExpectedKwargsEvaluationName(t *testing.T) {
	data := "name: d\ncases:\n  - inputs: x\nevaluators:\n  - EqualsExpected:\n      evaluation_name: en\n"
	ds := loadString(t, data, LoadOptions[string, string, string]{})
	e, ok := ds.Evaluators[0].(EqualsExpected[string, string, string])
	if !ok {
		t.Fatalf("expected EqualsExpected, got %T", ds.Evaluators[0])
	}
	if e.Name != "en" {
		t.Fatalf("EqualsExpected.Name = %q, want en", e.Name)
	}
}

// Equals carries its evaluation_name through the kwargs form on load.
func TestLoadEqualsKwargsEvaluationName(t *testing.T) {
	data := "name: d\ncases:\n  - inputs: x\nevaluators:\n  - Equals:\n      value: hi\n      evaluation_name: en\n"
	ds := loadString(t, data, LoadOptions[string, string, string]{})
	e, ok := ds.Evaluators[0].(Equals[string, string, string])
	if !ok {
		t.Fatalf("expected Equals, got %T", ds.Evaluators[0])
	}
	if e.Value != "hi" || e.Name != "en" {
		t.Fatalf("Equals = %+v", e)
	}
	if e.EvaluationName() != "en" {
		t.Fatalf("EvaluationName = %q, want en", e.EvaluationName())
	}
}

// Equals' value must be JSON-compatible with the output type O.
func TestLoadEqualsIncompatibleValue(t *testing.T) {
	reg := NewRegistry[string, int, string]()
	reg.RegisterDefaults()
	_, err := LoadDataset(
		[]byte("name: d\ncases:\n  - inputs: x\nevaluators:\n  - Equals: notanint\n"),
		reg,
		LoadOptions[string, int, string]{},
	)
	if err == nil {
		t.Fatal("expected error")
	}
	assertContains(t, err.Error(), `failed to instantiate evaluator "Equals" for dataset: Equals value:`)
	assertContains(t, err.Error(), "cannot unmarshal string into Go value of type int")
}

// MaxDuration accepts seconds via kwargs as well as a single positional arg.
func TestLoadMaxDurationKwargs(t *testing.T) {
	ds := loadString(t, "name: d\ncases:\n  - inputs: x\nevaluators:\n  - MaxDuration:\n      seconds: 2.5\n", LoadOptions[string, string, string]{})
	md, ok := ds.Evaluators[0].(MaxDuration[string, string, string])
	if !ok {
		t.Fatalf("expected MaxDuration, got %T", ds.Evaluators[0])
	}
	if md.Max != 2500*time.Millisecond {
		t.Fatalf("expected 2.5s, got %v", md.Max)
	}
}

// MaxDuration accepts an integer seconds value (YAML decodes it as int).
func TestLoadMaxDurationIntSeconds(t *testing.T) {
	ds := loadString(t, "name: d\ncases:\n  - inputs: x\nevaluators:\n  - MaxDuration: 3\n", LoadOptions[string, string, string]{})
	md := ds.Evaluators[0].(MaxDuration[string, string, string])
	if md.Max != 3*time.Second {
		t.Fatalf("expected 3s, got %v", md.Max)
	}
}

// --- parse forms: bare name, positional arg, kwargs --------------------------

func TestLoadParseForms(t *testing.T) {
	data := "name: d\ncases:\n  - inputs: x\nevaluators:\n" +
		"  - EqualsExpected\n" + // bare name
		"  - IsInstance: string\n" + // single positional (scalar)
		"  - Equals: HELLO\n" + // single positional (scalar)
		"  - Contains: ell\n" + // single positional (scalar)
		"  - EqualsExpected: posname\n" + // positional string -> Name
		"  - Equals:\n      value: hi\n      evaluation_name: en\n" + // kwargs (string-keyed map)
		"  - Contains:\n      value: hi\n      case_sensitive: true\n      as_strings: true\n      evaluation_name: cn\n"
	ds := loadString(t, data, LoadOptions[string, string, string]{})
	if len(ds.Evaluators) != 7 {
		t.Fatalf("expected 7 evaluators, got %d", len(ds.Evaluators))
	}

	if _, ok := ds.Evaluators[0].(EqualsExpected[string, string, string]); !ok {
		t.Fatalf("evaluators[0] = %T, want EqualsExpected", ds.Evaluators[0])
	}
	if e := ds.Evaluators[1].(IsInstance[string, string, string]); e.TypeName != "string" {
		t.Fatalf("evaluators[1] = %+v", e)
	}
	if e := ds.Evaluators[2].(Equals[string, string, string]); e.Value != "HELLO" {
		t.Fatalf("evaluators[2] = %+v", e)
	}
	if e := ds.Evaluators[3].(Contains[string, string, string]); e.Value != "ell" {
		t.Fatalf("evaluators[3] = %+v", e)
	}
	if e := ds.Evaluators[4].(EqualsExpected[string, string, string]); e.Name != "posname" {
		t.Fatalf("evaluators[4] = %+v", e)
	}
	if e := ds.Evaluators[5].(Equals[string, string, string]); e.Value != "hi" || e.Name != "en" {
		t.Fatalf("evaluators[5] = %+v", e)
	}
	if e := ds.Evaluators[6].(Contains[string, string, string]); e.Value != "hi" || !e.CaseSensitive || !e.AsStrings || e.Name != "cn" {
		t.Fatalf("evaluators[6] = %+v", e)
	}
}

// --- a custom factory used during a load -------------------------------------

type wordCount struct {
	Min int
}

func (w wordCount) Evaluate(_ context.Context, ec *EvaluatorContext[string, string, string]) (Output, error) {
	return Assertion(len(strings.Fields(ec.Output)) >= w.Min), nil
}

func (w wordCount) Spec() EvaluatorSpec {
	return NewSpecKwargs("WordCount", map[string]any{"min": w.Min})
}

func TestLoadCustomFactory(t *testing.T) {
	reg := NewRegistry[string, string, string]()
	reg.RegisterDefaults()
	reg.Register("WordCount", func(spec EvaluatorSpec) (Evaluator[string, string, string], error) {
		min := 0
		switch v := spec.Kwargs["min"].(type) {
		case int:
			min = v
		case float64:
			min = int(v)
		}
		return wordCount{Min: min}, nil
	})

	data := []byte("name: d\ncases:\n  - inputs: x\n    expected_output: hello world\nevaluators:\n  - WordCount:\n      min: 2\n")
	ds, err := LoadDataset(data, reg, LoadOptions[string, string, string]{})
	if err != nil {
		t.Fatalf("LoadDataset: %v", err)
	}
	rep, err := ds.Evaluate(context.Background(), func(_ context.Context, in string) (string, error) {
		return "hello world", nil
	})
	if err != nil {
		t.Fatalf("Evaluate: %v", err)
	}
	res, ok := rep.Cases[0].Assertions["WordCount"]
	if !ok {
		t.Fatalf("missing WordCount assertion: %+v", rep.Cases[0].Assertions)
	}
	if res.Value != Bool(true) {
		t.Fatalf("WordCount assertion = %v, want True", res.Value)
	}
	if res.Source.Name != "WordCount" || res.Source.Kwargs["min"] != 2 {
		t.Fatalf("unexpected source spec: %+v", res.Source)
	}
}

// Register overrides an existing factory for the same name.
func TestRegisterOverride(t *testing.T) {
	reg := NewRegistry[string, string, string]()
	reg.RegisterDefaults()
	reg.Register("Equals", func(spec EvaluatorSpec) (Evaluator[string, string, string], error) {
		return EqualsExpected[string, string, string]{Name: "overridden"}, nil
	})
	ds, err := LoadDataset(
		[]byte("name: d\ncases:\n  - inputs: x\nevaluators:\n  - Equals: ignored\n"),
		reg,
		LoadOptions[string, string, string]{},
	)
	if err != nil {
		t.Fatalf("LoadDataset: %v", err)
	}
	if e, ok := ds.Evaluators[0].(EqualsExpected[string, string, string]); !ok || e.Name != "overridden" {
		t.Fatalf("override not applied: %T %+v", ds.Evaluators[0], ds.Evaluators[0])
	}
}

// --- full round trips: Save -> LoadDataset -> Evaluate -----------------------

func TestRoundTripBuiltins(t *testing.T) {
	src, err := NewDataset(
		"rt",
		[]Case[string, string, string]{
			NewCase[string, string, string](
				"in",
				WithCaseName[string, string, string]("c1"),
				WithExpectedOutput[string, string, string]("hello"),
			),
		},
		Equals[string, string, string]{Value: "hello"},             // positional kwargs spec
		EqualsExpected[string, string, string]{},                   // bare name
		Contains[string, string, string]{Value: "ell"},             // kwargs spec
		IsInstance[string, string, string]{TypeName: "string"},     // single positional arg spec
		MaxDuration[string, string, string]{Max: 10 * time.Second}, // single positional arg spec
	)
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}

	for _, format := range []string{"yaml", "json"} {
		t.Run(format, func(t *testing.T) {
			data, err := src.Save(SaveOptions{Format: format})
			if err != nil {
				t.Fatalf("Save: %v", err)
			}
			reg := NewRegistry[string, string, string]()
			reg.RegisterDefaults()
			ds, err := LoadDataset(data, reg, LoadOptions[string, string, string]{Format: format})
			if err != nil {
				t.Fatalf("LoadDataset: %v", err)
			}
			rep, err := ds.Evaluate(context.Background(), func(_ context.Context, in string) (string, error) {
				return "hello", nil
			})
			if err != nil {
				t.Fatalf("Evaluate: %v", err)
			}
			c := rep.Cases[0]
			want := map[string]bool{
				"Equals": true, "EqualsExpected": true, "Contains": true,
				"IsInstance": true, "MaxDuration": true,
			}
			for name := range want {
				res, ok := c.Assertions[name]
				if !ok {
					t.Fatalf("missing assertion %q in %v", name, c.Assertions)
				}
				if res.Value != Bool(true) {
					t.Fatalf("assertion %q = %v, want True", name, res.Value)
				}
			}
			if len(c.Assertions) != len(want) {
				t.Fatalf("unexpected assertions: %+v", c.Assertions)
			}
			if c.ExpectedOutput != "hello" || !c.HasExpectedOutput {
				t.Fatalf("expected_output not round-tripped: %+v", c)
			}
		})
	}
}

// --- struct I/O/M with custom Decode hooks -----------------------------------

type box struct {
	V string
}

func TestLoadDecodeHooks(t *testing.T) {
	data := []byte("name: d\ncases:\n  - inputs: raw\n    expected_output: exp\n    metadata: md\n")
	reg := NewRegistry[box, box, box]()
	reg.RegisterDefaults()
	ds, err := LoadDataset(data, reg, LoadOptions[box, box, box]{
		DecodeInputs:   func(v any) (box, error) { return box{V: "I:" + v.(string)}, nil },
		DecodeOutput:   func(v any) (box, error) { return box{V: "O:" + v.(string)}, nil },
		DecodeMetadata: func(v any) (box, error) { return box{V: "M:" + v.(string)}, nil },
	})
	if err != nil {
		t.Fatalf("LoadDataset: %v", err)
	}
	c := ds.Cases[0]
	if c.Inputs.V != "I:raw" || c.ExpectedOutput.V != "O:exp" || c.Metadata.V != "M:md" {
		t.Fatalf("decode hooks not applied: %+v", c)
	}
	if !c.HasExpectedOutput || !c.HasMetadata {
		t.Fatalf("Has* flags not set: %+v", c)
	}
}

func TestLoadDecodeHookErrors(t *testing.T) {
	reg := NewRegistry[box, box, box]()
	reg.RegisterDefaults()

	boom := func(any) (box, error) { return box{}, errBoom }
	ok := func(v any) (box, error) { return box{V: v.(string)}, nil }

	tests := []struct {
		name string
		data string
		opts LoadOptions[box, box, box]
		want string
	}{
		{
			name: "inputs",
			data: "name: d\ncases:\n  - name: c1\n    inputs: raw\n",
			opts: LoadOptions[box, box, box]{DecodeInputs: boom},
			want: `case "c1" inputs: boom`,
		},
		{
			name: "metadata",
			data: "name: d\ncases:\n  - name: c1\n    inputs: raw\n    metadata: md\n",
			opts: LoadOptions[box, box, box]{DecodeInputs: ok, DecodeMetadata: boom},
			want: `case "c1" metadata: boom`,
		},
		{
			name: "expected_output",
			data: "name: d\ncases:\n  - name: c1\n    inputs: raw\n    expected_output: exp\n",
			opts: LoadOptions[box, box, box]{DecodeInputs: ok, DecodeOutput: boom},
			want: `case "c1" expected_output: boom`,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := LoadDataset([]byte(tt.data), reg, tt.opts)
			if err == nil {
				t.Fatal("expected error")
			}
			assertContains(t, err.Error(), tt.want)
		})
	}
}

var errBoom = errBoomType{}

type errBoomType struct{}

func (errBoomType) Error() string { return "boom" }

// --- default JSON-roundtrip decoder for struct types with nested YAML maps ---

type person struct {
	Name string         `json:"name"`
	Tags map[string]int `json:"tags"`
}

func TestLoadDefaultDecoderStructNestedMaps(t *testing.T) {
	data := []byte(`name: people
cases:
  - name: alice
    inputs:
      name: Alice
      tags:
        a: 1
        b: 2
    expected_output:
      name: ALICE
      tags:
        x: 9
    metadata:
      name: meta
      tags:
        m: 5
    evaluators:
      - EqualsExpected
`)
	reg := NewRegistry[person, person, person]()
	reg.RegisterDefaults()
	ds, err := LoadDataset(data, reg, LoadOptions[person, person, person]{})
	if err != nil {
		t.Fatalf("LoadDataset: %v", err)
	}
	c := ds.Cases[0]
	wantIn := person{Name: "Alice", Tags: map[string]int{"a": 1, "b": 2}}
	if c.Inputs.Name != wantIn.Name || c.Inputs.Tags["a"] != 1 || c.Inputs.Tags["b"] != 2 {
		t.Fatalf("inputs = %+v, want %+v", c.Inputs, wantIn)
	}
	if c.ExpectedOutput.Name != "ALICE" || c.ExpectedOutput.Tags["x"] != 9 {
		t.Fatalf("expected_output = %+v", c.ExpectedOutput)
	}
	if c.Metadata.Name != "meta" || c.Metadata.Tags["m"] != 5 {
		t.Fatalf("metadata = %+v", c.Metadata)
	}
}

// --- default JSON-roundtrip decoder fast path: value already of target type --

func TestLoadDefaultDecoderAnyFastPath(t *testing.T) {
	reg := NewRegistry[any, any, any]()
	reg.RegisterDefaults()
	ds, err := LoadDataset(
		[]byte("name: d\ncases:\n  - inputs: 5\n    expected_output: hi\n    metadata: true\n"),
		reg,
		LoadOptions[any, any, any]{},
	)
	if err != nil {
		t.Fatalf("LoadDataset: %v", err)
	}
	c := ds.Cases[0]
	if c.Inputs != 5 {
		t.Fatalf("inputs = %#v, want int 5", c.Inputs)
	}
	if c.ExpectedOutput != "hi" || !c.HasExpectedOutput {
		t.Fatalf("expected_output = %#v has=%v", c.ExpectedOutput, c.HasExpectedOutput)
	}
	if c.Metadata != true || !c.HasMetadata {
		t.Fatalf("metadata = %#v has=%v", c.Metadata, c.HasMetadata)
	}
}

// --- cases with and without metadata / expected_output -----------------------

func TestSaveCasesWithAndWithoutOptionalFields(t *testing.T) {
	ds, err := NewDataset[string, string, string](
		"d",
		[]Case[string, string, string]{
			NewCase[string, string, string](
				"with",
				WithCaseName[string, string, string]("full"),
				WithMetadata[string, string, string]("md"),
				WithExpectedOutput[string, string, string]("eo"),
			),
			NewCase[string, string, string](
				"without",
				WithCaseName[string, string, string]("bare"),
			),
		},
	)
	if err != nil {
		t.Fatalf("NewDataset: %v", err)
	}
	const want = `name: d
cases:
  - name: full
    inputs: with
    metadata: md
    expected_output: eo
  - name: bare
    inputs: without
`
	if got := mustSave(t, ds, SaveOptions{}); got != want {
		t.Fatalf("optional fields YAML mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}

	// And the round trip preserves the Has* flags.
	reg := NewRegistry[string, string, string]()
	reg.RegisterDefaults()
	loaded, err := LoadDataset([]byte(want), reg, LoadOptions[string, string, string]{})
	if err != nil {
		t.Fatalf("LoadDataset: %v", err)
	}
	if !loaded.Cases[0].HasMetadata || !loaded.Cases[0].HasExpectedOutput {
		t.Fatalf("full case lost Has* flags: %+v", loaded.Cases[0])
	}
	if loaded.Cases[1].HasMetadata || loaded.Cases[1].HasExpectedOutput {
		t.Fatalf("bare case gained Has* flags: %+v", loaded.Cases[1])
	}
}
