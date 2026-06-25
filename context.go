package evals

import (
	"sync"
	"time"
)

// EvaluatorContext carries everything an [Evaluator] needs to score one case.
//
// It holds the case inputs, the actual and expected outputs, metadata, the task
// duration, and any attributes and metrics recorded during the task run via
// [SetAttribute] and [IncrementMetric].
type EvaluatorContext[I, O, M any] struct {
	// Name is the name of the case, if any.
	Name string
	// Inputs is the input provided to the task for this case.
	Inputs I
	// Metadata associated with the case.
	Metadata M
	// HasMetadata reports whether Metadata was set on the case.
	HasMetadata bool
	// ExpectedOutput is the expected output for the case.
	ExpectedOutput O
	// HasExpectedOutput reports whether ExpectedOutput was set on the case.
	HasExpectedOutput bool
	// Output is the actual output produced by the task.
	Output O
	// Duration is how long the task run took.
	Duration time.Duration
	// Attributes recorded during the task run via [SetAttribute].
	Attributes map[string]any
	// Metrics recorded during the task run via [IncrementMetric].
	Metrics map[string]float64
}

// taskRun accumulates attributes and metrics recorded during one task run.
type taskRun struct {
	mu         sync.Mutex
	attributes map[string]any
	metrics    map[string]float64
}

func newTaskRun() *taskRun {
	return &taskRun{attributes: map[string]any{}, metrics: map[string]float64{}}
}

func (t *taskRun) recordAttribute(name string, value any) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.attributes[name] = value
}

func (t *taskRun) incrementMetric(name string, amount float64) {
	t.mu.Lock()
	defer t.mu.Unlock()
	current := t.metrics[name]
	incremented := current + amount
	if current == 0 && incremented == 0 {
		return
	}
	t.metrics[name] = incremented
}
