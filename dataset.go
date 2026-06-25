package evals

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// TaskFunc is the function under evaluation. It receives the task-run context
// (on which [SetAttribute] and [IncrementMetric] may be called) and a case's
// inputs, and returns the produced output.
type TaskFunc[I, O any] func(ctx context.Context, inputs I) (O, error)

// Lifecycle provides per-case setup, context preparation, and teardown hooks.
// A fresh instance is created for each case via the factory passed to
// [WithLifecycle]. All methods are optional; embed [BaseLifecycle] to get no-op
// defaults.
type Lifecycle[I, O, M any] interface {
	// Setup runs before task execution.
	Setup(ctx context.Context) error
	// PrepareContext runs after the task, before evaluators, and may enrich the
	// evaluator context (e.g. add metrics or attributes).
	PrepareContext(ctx context.Context, ec *EvaluatorContext[I, O, M]) (*EvaluatorContext[I, O, M], error)
	// Teardown runs after evaluators complete. result is nil if the case ended
	// without producing a report (e.g. cancellation).
	Teardown(ctx context.Context, result *ReportCase[I, O, M], failure *ReportCaseFailure[I, O, M]) error
}

// BaseLifecycle is a no-op [Lifecycle] suitable for embedding.
type BaseLifecycle[I, O, M any] struct{}

func (BaseLifecycle[I, O, M]) Setup(context.Context) error { return nil }
func (BaseLifecycle[I, O, M]) PrepareContext(_ context.Context, ec *EvaluatorContext[I, O, M]) (*EvaluatorContext[I, O, M], error) {
	return ec, nil
}
func (BaseLifecycle[I, O, M]) Teardown(context.Context, *ReportCase[I, O, M], *ReportCaseFailure[I, O, M]) error {
	return nil
}

// Dataset is a collection of test [Case]s evaluated against a task.
type Dataset[I, O, M any] struct {
	// Name of the dataset.
	Name string
	// Cases in the dataset.
	Cases []Case[I, O, M]
	// Evaluators applied to all cases.
	Evaluators []Evaluator[I, O, M]
}

// NewDataset creates a [Dataset], validating that case names are unique.
func NewDataset[I, O, M any](name string, cases []Case[I, O, M], evaluators ...Evaluator[I, O, M]) (*Dataset[I, O, M], error) {
	seen := map[string]bool{}
	for _, c := range cases {
		if c.Name == "" {
			continue
		}
		if seen[c.Name] {
			return nil, fmt.Errorf("duplicate case name: %q", c.Name)
		}
		seen[c.Name] = true
	}
	return &Dataset[I, O, M]{Name: name, Cases: cases, Evaluators: evaluators}, nil
}

// AddCase appends a case, validating that its name is unique.
func (d *Dataset[I, O, M]) AddCase(c Case[I, O, M]) error {
	if c.Name != "" {
		for _, existing := range d.Cases {
			if existing.Name == c.Name {
				return fmt.Errorf("duplicate case name: %q", c.Name)
			}
		}
	}
	d.Cases = append(d.Cases, c)
	return nil
}

// AddEvaluator adds an evaluator to all cases in the dataset.
func (d *Dataset[I, O, M]) AddEvaluator(e Evaluator[I, O, M]) {
	d.Evaluators = append(d.Evaluators, e)
}

// AddEvaluatorForCase adds an evaluator to the case with the given name.
// It returns an error if no such case exists.
func (d *Dataset[I, O, M]) AddEvaluatorForCase(caseName string, e Evaluator[I, O, M]) error {
	for i := range d.Cases {
		if d.Cases[i].Name == caseName {
			d.Cases[i].Evaluators = append(d.Cases[i].Evaluators, e)
			return nil
		}
	}
	return fmt.Errorf("case %q not found in the dataset", caseName)
}

// Config configures a call to [Dataset.Evaluate]. The zero value is valid: it
// runs each case once, unbounded, with the experiment named after the task.
//
// Config is intentionally non-generic so that Evaluate calls need no type
// parameters. For per-case lifecycle hooks, use [Dataset.EvaluateWithLifecycle].
type Config struct {
	// Name of the experiment; defaults to TaskName.
	Name string
	// TaskName is the displayed task name; defaults to "task".
	TaskName string
	// MaxConcurrency limits concurrent case evaluations; <= 0 means unlimited.
	MaxConcurrency int
	// Repeat runs each case this many times (default/zero is treated as 1).
	// When > 1, results are grouped by the original case name for aggregation.
	Repeat int
	// Metadata is arbitrary experiment metadata recorded on the report.
	Metadata map[string]any
}

type taskToRun[I, O, M any] struct {
	c              Case[I, O, M]
	reportCaseName string
	sourceCaseName string
}

func (d *Dataset[I, O, M]) buildTasksToRun(repeat int) []taskToRun[I, O, M] {
	var tasks []taskToRun[I, O, M]
	for i, c := range d.Cases {
		caseName := c.Name
		if caseName == "" {
			caseName = fmt.Sprintf("Case %d", i+1)
		}
		if repeat > 1 {
			for runIdx := 1; runIdx <= repeat; runIdx++ {
				tasks = append(tasks, taskToRun[I, O, M]{
					c:              c,
					reportCaseName: fmt.Sprintf("%s [%d/%d]", caseName, runIdx, repeat),
					sourceCaseName: caseName,
				})
			}
		} else {
			tasks = append(tasks, taskToRun[I, O, M]{c: c, reportCaseName: caseName})
		}
	}
	return tasks
}

// Evaluate runs the task against every case in the dataset, applying the
// dataset-level and case-level evaluators, and returns an [EvaluationReport].
//
// Cases run concurrently (bounded by cfg.MaxConcurrency). A task error produces
// a [ReportCaseFailure]; an evaluator error produces an [EvaluatorFailure] on
// the case rather than failing the case. Evaluate itself only returns an error
// if the config is invalid or ctx is cancelled.
//
// cfg is optional; pass at most one [Config]. For per-case lifecycle hooks, use
// [Dataset.EvaluateWithLifecycle].
func (d *Dataset[I, O, M]) Evaluate(ctx context.Context, task TaskFunc[I, O], cfg ...Config) (*EvaluationReport[I, O, M], error) {
	var c Config
	if len(cfg) > 0 {
		c = cfg[0]
	}
	return d.evaluate(ctx, task, c, nil)
}

// EvaluateWithLifecycle is like [Dataset.Evaluate] but invokes newLifecycle once
// per case to obtain its [Lifecycle] hooks (Setup, PrepareContext, Teardown).
func (d *Dataset[I, O, M]) EvaluateWithLifecycle(ctx context.Context, task TaskFunc[I, O], newLifecycle func(c Case[I, O, M]) Lifecycle[I, O, M], cfg ...Config) (*EvaluationReport[I, O, M], error) {
	var c Config
	if len(cfg) > 0 {
		c = cfg[0]
	}
	return d.evaluate(ctx, task, c, newLifecycle)
}

func (d *Dataset[I, O, M]) evaluate(ctx context.Context, task TaskFunc[I, O], c Config, newLifecycle func(c Case[I, O, M]) Lifecycle[I, O, M]) (*EvaluationReport[I, O, M], error) {
	if c.Repeat < 0 {
		return nil, fmt.Errorf("repeat must be >= 0, got %d", c.Repeat)
	}
	repeat := c.Repeat
	if repeat == 0 {
		repeat = 1
	}
	taskName := c.TaskName
	if taskName == "" {
		taskName = "task"
	}
	name := c.Name
	if name == "" {
		name = taskName
	}

	tasks := d.buildTasksToRun(repeat)
	results := make([]caseResult[I, O, M], len(tasks))

	expCtx, expSpan := startExperimentSpan(ctx, name, taskName, d.Name, len(d.Cases), repeat, c.Metadata)
	defer expSpan.End()

	var sem chan struct{}
	if c.MaxConcurrency > 0 {
		sem = make(chan struct{}, c.MaxConcurrency)
	}

	var wg sync.WaitGroup
	for i := range tasks {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			if sem != nil {
				sem <- struct{}{}
				defer func() { <-sem }()
			}
			results[idx] = d.runCase(expCtx, task, tasks[idx], taskName, newLifecycle)
		}(i)
	}
	wg.Wait()

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	report := &EvaluationReport[I, O, M]{Name: name, ExperimentMetadata: c.Metadata}
	for _, r := range results {
		if r.failure != nil {
			report.Failures = append(report.Failures, *r.failure)
		} else if r.success != nil {
			report.Cases = append(report.Cases, *r.success)
		}
	}
	if avg := report.Averages(); avg != nil && avg.Assertions != nil {
		setExperimentResultAttributes(expSpan, *avg.Assertions)
	}
	return report, nil
}

type caseResult[I, O, M any] struct {
	success *ReportCase[I, O, M]
	failure *ReportCaseFailure[I, O, M]
}

func (d *Dataset[I, O, M]) runCase(
	ctx context.Context,
	task TaskFunc[I, O],
	t taskToRun[I, O, M],
	taskName string,
	newLifecycle func(c Case[I, O, M]) Lifecycle[I, O, M],
) caseResult[I, O, M] {
	var lc Lifecycle[I, O, M]
	if newLifecycle != nil {
		lc = newLifecycle(t.c)
	}

	caseCtx, caseSpan := startCaseSpan(ctx, taskName, t.reportCaseName,
		t.c.Inputs, t.c.Metadata, t.c.ExpectedOutput, t.c.HasMetadata, t.c.HasExpectedOutput, t.sourceCaseName)
	defer caseSpan.End()

	start := time.Now()
	var result caseResult[I, O, M]

	func() {
		if lc != nil {
			if err := lc.Setup(caseCtx); err != nil {
				result.failure = d.newFailure(t, fmt.Errorf("setup: %w", err))
				return
			}
		}

		ec, err := d.runTask(caseCtx, task, t.c, taskName)
		if err != nil {
			result.failure = d.newFailure(t, err)
			return
		}

		if lc != nil {
			ec, err = lc.PrepareContext(caseCtx, ec)
			if err != nil {
				result.failure = d.newFailure(t, fmt.Errorf("prepare context: %w", err))
				return
			}
		}

		evaluators := append(append([]Evaluator[I, O, M]{}, t.c.Evaluators...), d.Evaluators...)
		evalResults, failures := runEvaluators(caseCtx, evaluators, ec)
		assertions, scores, labels := groupResults(evalResults)

		result.success = &ReportCase[I, O, M]{
			Name:              t.reportCaseName,
			Inputs:            t.c.Inputs,
			Metadata:          t.c.Metadata,
			HasMetadata:       t.c.HasMetadata,
			ExpectedOutput:    t.c.ExpectedOutput,
			HasExpectedOutput: t.c.HasExpectedOutput,
			Output:            ec.Output,
			Metrics:           ec.Metrics,
			Attributes:        ec.Attributes,
			Scores:            scores,
			Labels:            labels,
			Assertions:        assertions,
			TaskDuration:      ec.Duration,
			TotalDuration:     time.Since(start),
			SourceCaseName:    t.sourceCaseName,
			EvaluatorFailures: failures,
		}
	}()

	if lc != nil {
		if err := lc.Teardown(caseCtx, result.success, result.failure); err != nil {
			result.success = nil
			result.failure = d.newFailure(t, fmt.Errorf("teardown: %w", err))
		}
	}

	if result.failure != nil {
		recordSpanError(caseSpan, fmt.Errorf("%s", result.failure.ErrorMessage))
	}
	if result.success != nil {
		result.success.TotalDuration = time.Since(start)
		setCaseResultAttributes(caseSpan, result.success)
	}
	return result
}

func (d *Dataset[I, O, M]) runTask(ctx context.Context, task TaskFunc[I, O], c Case[I, O, M], taskName string) (*EvaluatorContext[I, O, M], error) {
	taskCtx, taskSpan := startTaskSpan(ctx, taskName)
	defer taskSpan.End()

	tr := newTaskRun()
	taskCtx = withTaskRun(taskCtx, tr)

	t0 := time.Now()
	output, err := task(taskCtx, c.Inputs)
	duration := time.Since(t0)
	if err != nil {
		recordSpanError(taskSpan, err)
		return nil, err
	}

	return &EvaluatorContext[I, O, M]{
		Name:              c.Name,
		Inputs:            c.Inputs,
		Metadata:          c.Metadata,
		HasMetadata:       c.HasMetadata,
		ExpectedOutput:    c.ExpectedOutput,
		HasExpectedOutput: c.HasExpectedOutput,
		Output:            output,
		Duration:          duration,
		Attributes:        tr.attributes,
		Metrics:           tr.metrics,
	}, nil
}

func (d *Dataset[I, O, M]) newFailure(t taskToRun[I, O, M], err error) *ReportCaseFailure[I, O, M] {
	return &ReportCaseFailure[I, O, M]{
		Name:              t.reportCaseName,
		Inputs:            t.c.Inputs,
		Metadata:          t.c.Metadata,
		HasMetadata:       t.c.HasMetadata,
		ExpectedOutput:    t.c.ExpectedOutput,
		HasExpectedOutput: t.c.HasExpectedOutput,
		ErrorMessage:      err.Error(),
		ErrorType:         errorType(err),
		SourceCaseName:    t.sourceCaseName,
	}
}

// runEvaluators runs all evaluators for a case concurrently, preserving order.
func runEvaluators[I, O, M any](ctx context.Context, evaluators []Evaluator[I, O, M], ec *EvaluatorContext[I, O, M]) ([]EvaluationResult, []EvaluatorFailure) {
	type outcome struct {
		results []EvaluationResult
		failure *EvaluatorFailure
	}
	outcomes := make([]outcome, len(evaluators))
	var wg sync.WaitGroup
	for i, e := range evaluators {
		wg.Add(1)
		go func(idx int, ev Evaluator[I, O, M]) {
			defer wg.Done()
			evalCtx, evalSpan := startEvaluatorSpan(ctx, defaultEvaluationName(ev))
			defer evalSpan.End()
			res, fail := runEvaluator(evalCtx, ev, ec)
			if fail != nil {
				recordSpanError(evalSpan, fmt.Errorf("%s", fail.ErrorMessage))
			}
			outcomes[idx] = outcome{results: res, failure: fail}
		}(i, e)
	}
	wg.Wait()

	var results []EvaluationResult
	var failures []EvaluatorFailure
	for _, o := range outcomes {
		if o.failure != nil {
			failures = append(failures, *o.failure)
		} else {
			results = append(results, o.results...)
		}
	}
	return results, failures
}

// groupResults partitions results into assertions (bool), scores (int/float) and
// labels (string), deduping repeated names with a numeric suffix.
func groupResults(results []EvaluationResult) (assertions, scores, labels map[string]EvaluationResult) {
	assertions = map[string]EvaluationResult{}
	scores = map[string]EvaluationResult{}
	labels = map[string]EvaluationResult{}
	seen := map[string]bool{}
	for _, r := range results {
		name := r.Name
		if seen[name] {
			suffix := 2
			for seen[fmt.Sprintf("%s_%d", name, suffix)] {
				suffix++
			}
			name = fmt.Sprintf("%s_%d", name, suffix)
		}
		seen[name] = true
		r.Name = name
		switch r.Value.(type) {
		case Bool:
			assertions[name] = r
		case Int, Float:
			scores[name] = r
		case Label:
			labels[name] = r
		}
	}
	return assertions, scores, labels
}
