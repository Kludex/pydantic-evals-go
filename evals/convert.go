package evals

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
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

var isoDuration = regexp.MustCompile(`^(-)?P(?:([\d.]+)W)?(?:([\d.]+)D)?(?:T(?:([\d.]+)H)?(?:([\d.]+)M)?(?:([\d.]+)S)?)?$`)

// decodeDuration decodes a number of seconds or an ISO-8601 duration string
// like "PT5S", the form Python writes timedelta values in.
func decodeDuration(v any) (time.Duration, error) {
	s, ok := v.(string)
	if !ok {
		f, err := toFloat(v)
		return secondsToDuration(f), err
	}
	m := isoDuration.FindStringSubmatch(s)
	if m == nil || strings.HasSuffix(s, "P") || strings.HasSuffix(s, "T") {
		return 0, fmt.Errorf("expected a number or ISO-8601 duration, got %q", s)
	}
	var seconds float64
	for i, unit := range []float64{7 * 86400, 86400, 3600, 60, 1} {
		if m[i+2] == "" {
			continue
		}
		n, err := strconv.ParseFloat(m[i+2], 64)
		if err != nil {
			return 0, fmt.Errorf("expected a number or ISO-8601 duration, got %q", s)
		}
		seconds += n * unit
	}
	if m[1] != "" {
		seconds = -seconds
	}
	return secondsToDuration(seconds), nil
}
