package evals

import (
	"context"
	"reflect"
)

type taskRunKey struct{}

// withTaskRun returns a context carrying the given task run, so that
// [SetAttribute] and [IncrementMetric] called within the task can record onto it.
func withTaskRun(ctx context.Context, tr *taskRun) context.Context {
	return context.WithValue(ctx, taskRunKey{}, tr)
}

func currentTaskRun(ctx context.Context) *taskRun {
	tr, _ := ctx.Value(taskRunKey{}).(*taskRun)
	return tr
}

// SetAttribute records an arbitrary attribute on the current task run.
//
// It is a no-op if ctx is not a task-run context (i.e. not the context passed to
// a task during [Dataset.Evaluate]). The recorded attributes are exposed to
// evaluators via [EvaluatorContext.Attributes] and on the resulting [ReportCase].
func SetAttribute(ctx context.Context, name string, value any) {
	if tr := currentTaskRun(ctx); tr != nil {
		tr.recordAttribute(name, value)
	}
}

// IncrementMetric increments a numeric metric on the current task run by amount.
//
// It is a no-op if ctx is not a task-run context. The accumulated metrics are
// exposed to evaluators via [EvaluatorContext.Metrics] and on the resulting
// [ReportCase], and are averaged in the report summary.
func IncrementMetric(ctx context.Context, name string, amount float64) {
	if tr := currentTaskRun(ctx); tr != nil {
		tr.incrementMetric(name, amount)
	}
}

// errorType returns a short type name for a non-nil error, used to populate
// [EvaluatorFailure.ErrorType] and [ReportCaseFailure] diagnostics. Pointer
// error types are unwrapped to their element so, e.g., *fmt.wrapError reports as
// "wrapError".
func errorType(err error) string {
	t := reflect.TypeOf(err)
	for t.Kind() == reflect.Ptr {
		t = t.Elem()
	}
	return t.Name()
}
