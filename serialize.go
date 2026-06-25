package evals

import (
	"encoding/json"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// EvaluatorFactory builds an [Evaluator] from a deserialized [EvaluatorSpec].
type EvaluatorFactory[I, O, M any] func(spec EvaluatorSpec) (Evaluator[I, O, M], error)

// Registry maps evaluator names to factories used when loading a [Dataset] from
// YAML or JSON. Construct one with [NewRegistry] and register built-ins via
// [Registry.RegisterDefaults] or custom evaluators via [Registry.Register].
type Registry[I, O, M any] struct {
	factories map[string]EvaluatorFactory[I, O, M]
}

// NewRegistry returns an empty [Registry].
func NewRegistry[I, O, M any]() *Registry[I, O, M] {
	return &Registry[I, O, M]{factories: map[string]EvaluatorFactory[I, O, M]{}}
}

// Register associates an evaluator name with a factory. It overrides any
// existing registration for that name.
func (r *Registry[I, O, M]) Register(name string, factory EvaluatorFactory[I, O, M]) {
	r.factories[name] = factory
}

// RegisterDefaults registers the built-in evaluators that do not require an LLM
// or telemetry: Equals, EqualsExpected, Contains, IsInstance and MaxDuration.
func (r *Registry[I, O, M]) RegisterDefaults() {
	r.Register("Equals", func(spec EvaluatorSpec) (Evaluator[I, O, M], error) {
		e := Equals[I, O, M]{}
		raw := spec.Kwargs["value"]
		if len(spec.Args) == 1 {
			raw = spec.Args[0]
		} else {
			e.Name, _ = spec.Kwargs["evaluation_name"].(string)
		}
		value, err := convertVia[O](raw)
		if err != nil {
			return nil, fmt.Errorf("Equals value: %w", err)
		}
		e.Value = value
		return e, nil
	})
	r.Register("EqualsExpected", func(spec EvaluatorSpec) (Evaluator[I, O, M], error) {
		e := EqualsExpected[I, O, M]{}
		if name, ok := spec.Kwargs["evaluation_name"].(string); ok {
			e.Name = name
		} else if len(spec.Args) == 1 {
			e.Name, _ = spec.Args[0].(string)
		}
		return e, nil
	})
	r.Register("Contains", func(spec EvaluatorSpec) (Evaluator[I, O, M], error) {
		e := Contains[I, O, M]{}
		if len(spec.Args) == 1 {
			e.Value = spec.Args[0]
		} else {
			e.Value = spec.Kwargs["value"]
			e.CaseSensitive, _ = spec.Kwargs["case_sensitive"].(bool)
			e.AsStrings, _ = spec.Kwargs["as_strings"].(bool)
			e.Name, _ = spec.Kwargs["evaluation_name"].(string)
		}
		return e, nil
	})
	r.Register("IsInstance", func(spec EvaluatorSpec) (Evaluator[I, O, M], error) {
		e := IsInstance[I, O, M]{}
		if len(spec.Args) == 1 {
			name, ok := spec.Args[0].(string)
			if !ok {
				return nil, fmt.Errorf("IsInstance type_name must be a string, got %T", spec.Args[0])
			}
			e.TypeName = name
		} else {
			e.TypeName, _ = spec.Kwargs["type_name"].(string)
			e.Name, _ = spec.Kwargs["evaluation_name"].(string)
		}
		if e.TypeName == "" {
			return nil, fmt.Errorf("IsInstance requires a type_name")
		}
		return e, nil
	})
	r.Register("MaxDuration", func(spec EvaluatorSpec) (Evaluator[I, O, M], error) {
		e := MaxDuration[I, O, M]{}
		var seconds float64
		if len(spec.Args) == 1 {
			f, err := toFloat(spec.Args[0])
			if err != nil {
				return nil, fmt.Errorf("MaxDuration seconds: %w", err)
			}
			seconds = f
		} else if v, ok := spec.Kwargs["seconds"]; ok {
			f, err := toFloat(v)
			if err != nil {
				return nil, fmt.Errorf("MaxDuration seconds: %w", err)
			}
			seconds = f
		} else {
			return nil, fmt.Errorf("MaxDuration requires seconds")
		}
		e.Max = secondsToDuration(seconds)
		return e, nil
	})
}

func (r *Registry[I, O, M]) load(spec EvaluatorSpec, context string) (Evaluator[I, O, M], error) {
	factory, ok := r.factories[spec.Name]
	if !ok {
		names := make([]string, 0, len(r.factories))
		for n := range r.factories {
			names = append(names, n)
		}
		return nil, fmt.Errorf("evaluator %q is not registered (valid choices: %v); register it before loading", spec.Name, names)
	}
	e, err := factory(spec)
	if err != nil {
		detail := ""
		if context != "" {
			detail = " for " + context
		}
		return nil, fmt.Errorf("failed to instantiate evaluator %q%s: %w", spec.Name, detail, err)
	}
	return e, nil
}

// datasetFile is the on-disk shape of a serialized dataset.
type datasetFile struct {
	Schema     string     `json:"$schema,omitempty" yaml:"$schema,omitempty"`
	Name       string     `json:"name,omitempty" yaml:"name,omitempty"`
	Cases      []caseFile `json:"cases" yaml:"cases"`
	Evaluators []any      `json:"evaluators,omitempty" yaml:"evaluators,omitempty"`
}

type caseFile struct {
	Name           string `json:"name,omitempty" yaml:"name,omitempty"`
	Inputs         any    `json:"inputs" yaml:"inputs"`
	Metadata       any    `json:"metadata,omitempty" yaml:"metadata,omitempty"`
	ExpectedOutput any    `json:"expected_output,omitempty" yaml:"expected_output,omitempty"`
	Evaluators     []any  `json:"evaluators,omitempty" yaml:"evaluators,omitempty"`
}

// LoadOptions configures dataset loading.
type LoadOptions[I, O, M any] struct {
	// Format is "yaml" or "json". When empty, it is inferred from the data: a
	// leading '{' implies JSON, otherwise YAML.
	Format string
	// DefaultName is used when the serialized data has no name.
	DefaultName string
	// DecodeInputs converts a deserialized inputs value into I.
	DecodeInputs func(any) (I, error)
	// DecodeOutput converts a deserialized expected_output value into O.
	DecodeOutput func(any) (O, error)
	// DecodeMetadata converts a deserialized metadata value into M.
	DecodeMetadata func(any) (M, error)
}

// LoadDataset parses a [Dataset] from YAML or JSON bytes, constructing
// case-level and dataset-level evaluators via the registry.
//
// Inputs, expected outputs and metadata are decoded with the Decode* hooks in
// opts; when a hook is nil the deserialized value is converted to the target
// type via a JSON round-trip.
func LoadDataset[I, O, M any](data []byte, reg *Registry[I, O, M], opts LoadOptions[I, O, M]) (*Dataset[I, O, M], error) {
	format := opts.Format
	if format == "" {
		if strings.HasPrefix(strings.TrimSpace(string(data)), "{") {
			format = "json"
		} else {
			format = "yaml"
		}
	}

	var df datasetFile
	switch format {
	case "json":
		if err := json.Unmarshal(data, &df); err != nil {
			return nil, fmt.Errorf("parsing dataset JSON: %w", err)
		}
	case "yaml":
		if err := yaml.Unmarshal(data, &df); err != nil {
			return nil, fmt.Errorf("parsing dataset YAML: %w", err)
		}
	default:
		return nil, fmt.Errorf("unknown format %q (want \"yaml\" or \"json\")", format)
	}

	name := df.Name
	if name == "" {
		name = opts.DefaultName
	}
	if name == "" {
		return nil, fmt.Errorf("dataset name is required: provide one in the data or via DefaultName")
	}

	decodeInputs := opts.DecodeInputs
	if decodeInputs == nil {
		decodeInputs = func(v any) (I, error) { return convertVia[I](v) }
	}
	decodeOutput := opts.DecodeOutput
	if decodeOutput == nil {
		decodeOutput = func(v any) (O, error) { return convertVia[O](v) }
	}
	decodeMetadata := opts.DecodeMetadata
	if decodeMetadata == nil {
		decodeMetadata = func(v any) (M, error) { return convertVia[M](v) }
	}

	var datasetEvaluators []Evaluator[I, O, M]
	for _, raw := range df.Evaluators {
		spec, err := parseSpec(normalizeYAML(raw))
		if err != nil {
			return nil, err
		}
		e, err := reg.load(spec, "dataset")
		if err != nil {
			return nil, err
		}
		datasetEvaluators = append(datasetEvaluators, e)
	}

	var cases []Case[I, O, M]
	for _, cf := range df.Cases {
		c := Case[I, O, M]{Name: cf.Name}
		in, err := decodeInputs(normalizeYAML(cf.Inputs))
		if err != nil {
			return nil, fmt.Errorf("case %q inputs: %w", cf.Name, err)
		}
		c.Inputs = in
		if cf.Metadata != nil {
			md, err := decodeMetadata(normalizeYAML(cf.Metadata))
			if err != nil {
				return nil, fmt.Errorf("case %q metadata: %w", cf.Name, err)
			}
			c.Metadata = md
			c.HasMetadata = true
		}
		if cf.ExpectedOutput != nil {
			eo, err := decodeOutput(normalizeYAML(cf.ExpectedOutput))
			if err != nil {
				return nil, fmt.Errorf("case %q expected_output: %w", cf.Name, err)
			}
			c.ExpectedOutput = eo
			c.HasExpectedOutput = true
		}
		for _, raw := range cf.Evaluators {
			spec, err := parseSpec(normalizeYAML(raw))
			if err != nil {
				return nil, err
			}
			e, err := reg.load(spec, fmt.Sprintf("case %q", cf.Name))
			if err != nil {
				return nil, err
			}
			c.Evaluators = append(c.Evaluators, e)
		}
		cases = append(cases, c)
	}

	return NewDataset(name, cases, datasetEvaluators...)
}

// SaveOptions configures dataset serialization.
type SaveOptions struct {
	// Format is "yaml" or "json". Defaults to "yaml".
	Format string
	// Schema, when set, is written as the "$schema" reference.
	Schema string
}

// Save serializes the dataset to YAML or JSON bytes, using the short form for
// each evaluator spec.
func (d *Dataset[I, O, M]) Save(opts SaveOptions) ([]byte, error) {
	format := opts.Format
	if format == "" {
		format = "yaml"
	}

	df := datasetFile{Schema: opts.Schema, Name: d.Name}
	for _, e := range d.Evaluators {
		df.Evaluators = append(df.Evaluators, evaluatorSpec[I, O, M](e).shortForm())
	}
	for _, c := range d.Cases {
		cf := caseFile{Name: c.Name, Inputs: c.Inputs}
		if c.HasMetadata {
			cf.Metadata = c.Metadata
		}
		if c.HasExpectedOutput {
			cf.ExpectedOutput = c.ExpectedOutput
		}
		for _, e := range c.Evaluators {
			cf.Evaluators = append(cf.Evaluators, evaluatorSpec[I, O, M](e).shortForm())
		}
		df.Cases = append(df.Cases, cf)
	}

	switch format {
	case "json":
		out, err := json.MarshalIndent(df, "", "  ")
		if err != nil {
			return nil, err
		}
		return append(out, '\n'), nil
	case "yaml":
		var b strings.Builder
		if opts.Schema != "" {
			b.WriteString("# yaml-language-server: $schema=" + opts.Schema + "\n")
		}
		if err := encodeYAML(&b, df); err != nil {
			return nil, err
		}
		return []byte(b.String()), nil
	default:
		return nil, fmt.Errorf("unknown format %q (want \"yaml\" or \"json\")", format)
	}
}

// encodeYAML encodes df into b. yaml.v3 reports a value it cannot marshal by
// panicking rather than returning an error, so we recover that panic into an
// error to keep Save's contract consistent with the JSON path and avoid crashing
// callers.
func encodeYAML(b *strings.Builder, df datasetFile) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("encoding dataset YAML: %v", r)
		}
	}()
	enc := yaml.NewEncoder(b)
	enc.SetIndent(2)
	_ = enc.Encode(df)
	return enc.Close()
}

// convertVia converts a deserialized value to T. If the value is already a T
// (the common case when T is `any` or matches the deserialized type), it is
// returned directly; otherwise it is converted via a JSON round-trip so that,
// e.g., a map[string]any can populate a struct T.
func convertVia[T any](v any) (T, error) {
	if t, ok := v.(T); ok {
		return t, nil
	}
	// v is always a value deserialized from YAML/JSON (run through normalizeYAML),
	// so it is composed solely of JSON-marshalable primitives and never errors here.
	raw, _ := json.Marshal(v)
	var out T
	if err := json.Unmarshal(raw, &out); err != nil {
		var zero T
		return zero, err
	}
	return out, nil
}

// normalizeYAML converts yaml.v3's map[interface{}]interface{} (and nested
// values) into map[string]any so the rest of the code can rely on string keys.
func normalizeYAML(v any) any {
	switch t := v.(type) {
	case map[any]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[fmt.Sprintf("%v", k)] = normalizeYAML(val)
		}
		return out
	case map[string]any:
		out := make(map[string]any, len(t))
		for k, val := range t {
			out[k] = normalizeYAML(val)
		}
		return out
	case []any:
		out := make([]any, len(t))
		for i, val := range t {
			out[i] = normalizeYAML(val)
		}
		return out
	default:
		return v
	}
}
