package evals

import (
	"fmt"
	"time"
)

// toFloat coerces a deserialized numeric value into a float64. YAML decodes
// numbers as int or float64, and JSON (without UseNumber) decodes them as
// float64, so those two kinds cover every value that can reach here.
func toFloat(v any) (float64, error) {
	switch n := v.(type) {
	case float64:
		return n, nil
	case int:
		return float64(n), nil
	default:
		return 0, fmt.Errorf("expected a number, got %T", v)
	}
}

// secondsToDuration converts a floating-point number of seconds to a
// time.Duration without losing sub-second precision.
func secondsToDuration(seconds float64) time.Duration {
	return time.Duration(seconds * float64(time.Second))
}
