package evals

import (
	"fmt"
	"sort"
)

// EvaluatorSpec is the serializable specification of an evaluator.
//
// It supports three short forms when (de)serialized to YAML/JSON, matching the
// Python `EvaluatorSpec`:
//
//   - "MyEvaluator" — the bare name, when there are no arguments;
//   - {"MyEvaluator": arg} — a single positional argument;
//   - {"MyEvaluator": {k1: v1, k2: v2}} — keyword arguments.
//
// Args holds a single positional argument (length 0 or 1); Kwargs holds keyword
// arguments. At most one of them is non-empty.
type EvaluatorSpec struct {
	// Name of the evaluator class to construct.
	Name string
	// Args holds the single positional argument, if any (length 0 or 1).
	Args []any
	// Kwargs holds the keyword arguments, if any.
	Kwargs map[string]any
}

// NewSpec builds an [EvaluatorSpec] with no arguments.
func NewSpec(name string) EvaluatorSpec {
	return EvaluatorSpec{Name: name}
}

// NewSpecArg builds an [EvaluatorSpec] with a single positional argument.
func NewSpecArg(name string, arg any) EvaluatorSpec {
	return EvaluatorSpec{Name: name, Args: []any{arg}}
}

// NewSpecKwargs builds an [EvaluatorSpec] with keyword arguments.
func NewSpecKwargs(name string, kwargs map[string]any) EvaluatorSpec {
	return EvaluatorSpec{Name: name, Kwargs: kwargs}
}

// shortForm renders the spec as the most compact YAML/JSON-compatible value:
// the bare name when there are no arguments, {name: arg} for a single positional
// argument, or {name: {kwargs}} for keyword arguments.
//
// A single positional argument that itself is a string-keyed map would be
// misread as kwargs on the round-trip; callers building specs for such
// arguments should use kwargs instead. The built-in evaluators only ever use
// scalar positional arguments, so they round-trip cleanly.
func (s EvaluatorSpec) shortForm() any {
	switch {
	case len(s.Kwargs) > 0:
		return map[string]any{s.Name: s.Kwargs}
	case len(s.Args) == 1:
		return map[string]any{s.Name: s.Args[0]}
	default:
		return s.Name
	}
}

// parseSpec parses an [EvaluatorSpec] from a deserialized YAML/JSON value, which
// is either a string (bare name) or a single-key map.
func parseSpec(value any) (EvaluatorSpec, error) {
	switch v := value.(type) {
	case string:
		return EvaluatorSpec{Name: v}, nil
	case map[string]any:
		if len(v) != 1 {
			keys := make([]string, 0, len(v))
			for k := range v {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			return EvaluatorSpec{}, fmt.Errorf("expected a single key containing the evaluator name, found keys %v", keys)
		}
		var name string
		var inner any
		for k, val := range v {
			name, inner = k, val
		}
		return specFromNameValue(name, inner), nil
	default:
		return EvaluatorSpec{}, fmt.Errorf("invalid evaluator spec: expected string or single-key mapping, got %T", value)
	}
}

// specFromNameValue interprets a {name: value} mapping. A string-keyed map value
// is treated as keyword arguments; anything else is a single positional argument.
// Callers pass values already run through normalizeYAML, so a kwargs mapping is
// always a map[string]any by the time it reaches here.
func specFromNameValue(name string, value any) EvaluatorSpec {
	if m, ok := value.(map[string]any); ok {
		return EvaluatorSpec{Name: name, Kwargs: m}
	}
	return EvaluatorSpec{Name: name, Args: []any{value}}
}
