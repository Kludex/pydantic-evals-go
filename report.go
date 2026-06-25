package evals

import (
	"sort"
	"time"
)

// ReportCase is a single successfully-evaluated case in an [EvaluationReport].
type ReportCase[I, O, M any] struct {
	// Name of the case in the report.
	Name string
	// Inputs to the task.
	Inputs I
	// Metadata associated with the case.
	Metadata    M
	HasMetadata bool
	// ExpectedOutput of the task.
	ExpectedOutput    O
	HasExpectedOutput bool
	// Output produced by the task.
	Output O

	// Metrics recorded during the task run.
	Metrics map[string]float64
	// Attributes recorded during the task run.
	Attributes map[string]any

	// Scores are evaluation results with numeric values.
	Scores map[string]EvaluationResult
	// Labels are evaluation results with string values.
	Labels map[string]EvaluationResult
	// Assertions are evaluation results with boolean values.
	Assertions map[string]EvaluationResult

	// TaskDuration is the duration of the task run.
	TaskDuration time.Duration
	// TotalDuration includes evaluator execution time.
	TotalDuration time.Duration

	// SourceCaseName is the original case name before run-indexing, used as the
	// aggregation key for multi-run (repeat) experiments. Empty when repeat == 1.
	SourceCaseName string

	// EvaluatorFailures are evaluators that errored for this case.
	EvaluatorFailures []EvaluatorFailure
}

// ReportCaseFailure is a case whose task execution raised an error.
type ReportCaseFailure[I, O, M any] struct {
	Name              string
	Inputs            I
	Metadata          M
	HasMetadata       bool
	ExpectedOutput    O
	HasExpectedOutput bool

	// ErrorMessage is the message of the error that caused the failure.
	ErrorMessage string
	// ErrorType is the Go type name of the error.
	ErrorType string

	// SourceCaseName mirrors [ReportCase.SourceCaseName].
	SourceCaseName string
}

// ReportCaseAggregate summarizes a set of cases by averaging their quantitative
// attributes. The synthetic "Averages" row in a report is a ReportCaseAggregate.
type ReportCaseAggregate struct {
	Name string

	Scores  map[string]float64
	Labels  map[string]map[string]float64
	Metrics map[string]float64
	// Assertions is the pass rate across all assertions, or nil if there were none.
	Assertions    *float64
	TaskDuration  time.Duration
	TotalDuration time.Duration
}

// ReportCaseGroup is the grouped result of running the same case multiple times
// (when repeat > 1). It is a computed view obtained via [EvaluationReport.CaseGroups].
type ReportCaseGroup[I, O, M any] struct {
	Name              string
	Inputs            I
	Metadata          M
	HasMetadata       bool
	ExpectedOutput    O
	HasExpectedOutput bool

	Runs     []ReportCase[I, O, M]
	Failures []ReportCaseFailure[I, O, M]
	Summary  ReportCaseAggregate
}

// EvaluationReport is the result of evaluating a task over a [Dataset].
type EvaluationReport[I, O, M any] struct {
	// Name of the report (the experiment name).
	Name string
	// Cases that were evaluated successfully.
	Cases []ReportCase[I, O, M]
	// Failures are cases whose task execution raised an error.
	Failures []ReportCaseFailure[I, O, M]
	// ExperimentMetadata associated with this experiment, if any.
	ExperimentMetadata map[string]any
}

// averageCases produces a synthetic summary case by averaging the given cases.
func averageCases[I, O, M any](cases []ReportCase[I, O, M]) ReportCaseAggregate {
	if len(cases) == 0 {
		return ReportCaseAggregate{Name: "Averages"}
	}
	n := float64(len(cases))

	scoresByName := make([]map[string]float64, 0, len(cases))
	metricsByName := make([]map[string]float64, 0, len(cases))
	labelsByName := make([]map[string]string, 0, len(cases))
	var taskSum, totalSum time.Duration
	for _, c := range cases {
		s := make(map[string]float64, len(c.Scores))
		for k, v := range c.Scores {
			s[k] = scalarToFloat(v.Value)
		}
		scoresByName = append(scoresByName, s)
		metricsByName = append(metricsByName, c.Metrics)
		l := make(map[string]string, len(c.Labels))
		for k, v := range c.Labels {
			l[k] = string(v.Value.(Label))
		}
		labelsByName = append(labelsByName, l)
		taskSum += c.TaskDuration
		totalSum += c.TotalDuration
	}

	var assertions *float64
	nAssertions := 0
	nPassing := 0
	for _, c := range cases {
		nAssertions += len(c.Assertions)
		for _, a := range c.Assertions {
			if bool(a.Value.(Bool)) {
				nPassing++
			}
		}
	}
	if nAssertions > 0 {
		rate := float64(nPassing) / float64(nAssertions)
		assertions = &rate
	}

	return ReportCaseAggregate{
		Name:          "Averages",
		Scores:        averageNumericMaps(scoresByName),
		Labels:        averageLabelMaps(labelsByName),
		Metrics:       averageNumericMaps(metricsByName),
		Assertions:    assertions,
		TaskDuration:  time.Duration(float64(taskSum) / n),
		TotalDuration: time.Duration(float64(totalSum) / n),
	}
}

// averageAggregates averages across multiple aggregates, used for multi-run
// experiment summaries.
func averageAggregates(aggregates []ReportCaseAggregate) ReportCaseAggregate {
	scoreMaps := make([]map[string]float64, len(aggregates))
	metricMaps := make([]map[string]float64, len(aggregates))
	for i, a := range aggregates {
		scoreMaps[i] = a.Scores
		metricMaps[i] = a.Metrics
	}

	labelKeys := map[string]bool{}
	for _, a := range aggregates {
		for k := range a.Labels {
			labelKeys[k] = true
		}
	}
	avgLabels := map[string]map[string]float64{}
	for key := range labelKeys {
		combined := map[string]float64{}
		count := 0
		for _, a := range aggregates {
			if dist, ok := a.Labels[key]; ok {
				count++
				for labelVal, freq := range dist {
					combined[labelVal] += freq
				}
			}
		}
		out := make(map[string]float64, len(combined))
		for k, v := range combined {
			out[k] = v / float64(count)
		}
		avgLabels[key] = out
	}

	var assertionVals []float64
	for _, a := range aggregates {
		if a.Assertions != nil {
			assertionVals = append(assertionVals, *a.Assertions)
		}
	}
	var avgAssertions *float64
	if len(assertionVals) > 0 {
		sum := 0.0
		for _, v := range assertionVals {
			sum += v
		}
		mean := sum / float64(len(assertionVals))
		avgAssertions = &mean
	}

	var taskSum, totalSum time.Duration
	for _, a := range aggregates {
		taskSum += a.TaskDuration
		totalSum += a.TotalDuration
	}
	n := float64(len(aggregates))

	return ReportCaseAggregate{
		Name:          "Averages",
		Scores:        averagePresentKeys(scoreMaps),
		Labels:        avgLabels,
		Metrics:       averagePresentKeys(metricMaps),
		Assertions:    avgAssertions,
		TaskDuration:  time.Duration(float64(taskSum) / n),
		TotalDuration: time.Duration(float64(totalSum) / n),
	}
}

// averageNumericMaps averages values per key, counting only entries where the
// key is present (matching Python's defaultdict-based `_scores_averages`).
func averageNumericMaps(maps []map[string]float64) map[string]float64 {
	counts := map[string]int{}
	sums := map[string]float64{}
	for _, m := range maps {
		for name, v := range m {
			counts[name]++
			sums[name] += v
		}
	}
	out := make(map[string]float64, len(sums))
	for name, sum := range sums {
		out[name] = sum / float64(counts[name])
	}
	return out
}

// averagePresentKeys averages values across maps, dividing by the number of maps
// that contain each key (matching Python's `_avg_numeric_dicts`).
func averagePresentKeys(maps []map[string]float64) map[string]float64 {
	allKeys := map[string]bool{}
	for _, m := range maps {
		for k := range m {
			allKeys[k] = true
		}
	}
	out := map[string]float64{}
	for key := range allKeys {
		var vals []float64
		for _, m := range maps {
			if v, ok := m[key]; ok {
				vals = append(vals, v)
			}
		}
		sum := 0.0
		for _, v := range vals {
			sum += v
		}
		out[key] = sum / float64(len(vals))
	}
	return out
}

func averageLabelMaps(maps []map[string]string) map[string]map[string]float64 {
	counts := map[string]int{}
	sums := map[string]map[string]float64{}
	for _, m := range maps {
		for name, label := range m {
			counts[name]++
			if sums[name] == nil {
				sums[name] = map[string]float64{}
			}
			sums[name][label]++
		}
	}
	out := make(map[string]map[string]float64, len(sums))
	for name, dist := range sums {
		inner := make(map[string]float64, len(dist))
		for value, count := range dist {
			inner[value] = count / float64(counts[name])
		}
		out[name] = inner
	}
	return out
}

func scalarToFloat(s Scalar) float64 {
	switch v := s.(type) {
	case Int:
		return float64(v)
	case Float:
		return float64(v)
	case Bool:
		if v {
			return 1
		}
		return 0
	default:
		return 0
	}
}

// CaseGroups groups cases by SourceCaseName and computes per-group aggregates.
// It returns nil for a single-run experiment (no case has a SourceCaseName).
func (r *EvaluationReport[I, O, M]) CaseGroups() []ReportCaseGroup[I, O, M] {
	hasSource := false
	for _, c := range r.Cases {
		if c.SourceCaseName != "" {
			hasSource = true
			break
		}
	}
	if !hasSource {
		for _, f := range r.Failures {
			if f.SourceCaseName != "" {
				hasSource = true
				break
			}
		}
	}
	if !hasSource {
		return nil
	}

	type bucket struct {
		runs     []ReportCase[I, O, M]
		failures []ReportCaseFailure[I, O, M]
	}
	order := []string{}
	groups := map[string]*bucket{}
	get := func(key string) *bucket {
		b, ok := groups[key]
		if !ok {
			b = &bucket{}
			groups[key] = b
			order = append(order, key)
		}
		return b
	}
	for _, c := range r.Cases {
		key := c.SourceCaseName
		if key == "" {
			key = c.Name
		}
		b := get(key)
		b.runs = append(b.runs, c)
	}
	for _, f := range r.Failures {
		key := f.SourceCaseName
		if key == "" {
			key = f.Name
		}
		b := get(key)
		b.failures = append(b.failures, f)
	}

	result := make([]ReportCaseGroup[I, O, M], 0, len(order))
	for _, key := range order {
		b := groups[key]
		var inputs I
		var metadata M
		var hasMeta, hasExp bool
		var expected O
		if len(b.runs) > 0 {
			inputs = b.runs[0].Inputs
			metadata = b.runs[0].Metadata
			hasMeta = b.runs[0].HasMetadata
			expected = b.runs[0].ExpectedOutput
			hasExp = b.runs[0].HasExpectedOutput
		} else {
			inputs = b.failures[0].Inputs
			metadata = b.failures[0].Metadata
			hasMeta = b.failures[0].HasMetadata
			expected = b.failures[0].ExpectedOutput
			hasExp = b.failures[0].HasExpectedOutput
		}
		result = append(result, ReportCaseGroup[I, O, M]{
			Name:              key,
			Inputs:            inputs,
			Metadata:          metadata,
			HasMetadata:       hasMeta,
			ExpectedOutput:    expected,
			HasExpectedOutput: hasExp,
			Runs:              b.runs,
			Failures:          b.failures,
			Summary:           averageCases(b.runs),
		})
	}
	return result
}

// Averages returns the overall summary aggregate for the report, or nil if there
// are no cases. For multi-run experiments it averages the per-group summaries.
func (r *EvaluationReport[I, O, M]) Averages() *ReportCaseAggregate {
	groups := r.CaseGroups()
	if groups != nil {
		var summaries []ReportCaseAggregate
		for _, g := range groups {
			if len(g.Runs) > 0 {
				summaries = append(summaries, g.Summary)
			}
		}
		if len(summaries) == 0 {
			return nil
		}
		agg := averageAggregates(summaries)
		return &agg
	}
	if len(r.Cases) > 0 {
		agg := averageCases(r.Cases)
		return &agg
	}
	return nil
}

// sortedKeys returns the keys of a map in sorted order, for deterministic output.
func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
