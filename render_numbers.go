package evals

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
)

const valueSigFigs = 3

// formatNumber formats a numeric value for reports: integers as integers (with
// thousands separators), floats with at least one decimal and at least three
// significant figures. This mirrors Python's `default_render_number`.
func formatNumber(value float64, isInt bool) string {
	if isInt {
		return withThousands(strconv.FormatInt(int64(value), 10))
	}

	absVal := math.Abs(value)
	if absVal == 0 {
		return formatFixed(value, valueSigFigs)
	}

	var decimals int
	if absVal >= 1 {
		digits := int(math.Floor(math.Log10(absVal))) + 1
		decimals = valueSigFigs - digits
		if decimals < 1 {
			decimals = 1
		}
	} else {
		exponent := int(math.Floor(math.Log10(absVal)))
		decimals = -exponent + valueSigFigs - 1
	}
	return formatFixed(value, decimals)
}

// formatPercentage formats a fraction as a percentage with one decimal place,
// matching Python's `default_render_percentage` (`{:.1%}`).
func formatPercentage(value float64) string {
	return strconv.FormatFloat(value*100, 'f', valueSigFigs-2, 64) + "%"
}

// formatDuration formats a duration for reports, choosing µs/ms/s units to match
// Python's `default_render_duration`.
func formatDuration(d time.Duration) string {
	seconds := d.Seconds()
	if seconds == 0 {
		return "0s"
	}
	precision := 1
	absSeconds := math.Abs(seconds)
	var value float64
	var unit string
	switch {
	case absSeconds < 1e-3:
		value = seconds * 1_000_000
		unit = "µs"
		if math.Abs(value) >= 1 {
			precision = 0
		}
	case absSeconds < 1:
		value = seconds * 1_000
		unit = "ms"
	default:
		value = seconds
		unit = "s"
	}
	return fmt.Sprintf("%.*f%s", precision, value, unit)
}

// formatFixed formats with the given number of decimals plus thousands separators.
func formatFixed(value float64, decimals int) string {
	return withThousands(strconv.FormatFloat(value, 'f', decimals, 64))
}

// withThousands inserts comma thousands separators into the integer part of a
// numeric string, matching Python's ',' format option.
func withThousands(s string) string {
	neg := strings.HasPrefix(s, "-")
	if neg {
		s = s[1:]
	}
	intPart := s
	rest := ""
	if i := strings.IndexAny(s, ".eE"); i >= 0 {
		intPart = s[:i]
		rest = s[i:]
	}
	if len(intPart) > 3 {
		var b strings.Builder
		lead := len(intPart) % 3
		if lead > 0 {
			b.WriteString(intPart[:lead])
		}
		for i := lead; i < len(intPart); i += 3 {
			if b.Len() > 0 {
				b.WriteByte(',')
			}
			b.WriteString(intPart[i : i+3])
		}
		intPart = b.String()
	}
	out := intPart + rest
	if neg {
		out = "-" + out
	}
	return out
}
