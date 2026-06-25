package evals

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
)

// stdout is the destination for [EvaluationReport.Print]. It is a variable so
// tests can capture output.
var stdout io.Writer = os.Stdout

// jsonString renders a value as compact JSON for use as a span attribute,
// falling back to fmt for values JSON cannot marshal.
func jsonString(v any) string {
	b, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(b)
}

// sprintValue renders an arbitrary value for a report cell. Maps are rendered
// with sorted keys for deterministic output; everything else uses fmt's default.
func sprintValue(v any) string {
	switch t := v.(type) {
	case map[string]any:
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		parts := make([]string, 0, len(t))
		for _, k := range keys {
			parts = append(parts, fmt.Sprintf("%s: %v", k, t[k]))
		}
		return "{" + strings.Join(parts, ", ") + "}"
	default:
		return fmt.Sprintf("%v", v)
	}
}
