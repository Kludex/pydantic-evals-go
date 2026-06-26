/*
Package evals is a small, type-safe toolkit for evaluating non-deterministic
functions — LLM calls, agents, retrieval pipelines, anything whose output you
can't assert with a single ==.

It's a Go port of [Pydantic Evals]. You bring a function to test; the library
runs it over a set of cases, scores each result with evaluators you choose, and
gives you a report you can print, diff, serialize, or ship to an
OpenTelemetry backend like [Pydantic Logfire].

# The mental model

Think of it as a test framework for probabilistic code:

  - A [Case] is one scenario: an input, an optional expected output, optional
    metadata.
  - A [Dataset] is a named collection of cases plus the evaluators that score
    them — your test suite.
  - A [TaskFunc] is the function under test: func(ctx, input) (output, error).
  - An [Evaluator] looks at a task's output and returns a result. Booleans
    become pass/fail assertions, numbers become scores, strings become labels.
  - An [EvaluationReport] collects every result, with per-case detail and an
    aggregate summary.

Everything is generic over three type parameters — the input (I), the output
(O), and the metadata (M) — so your evaluators receive already-typed values with
no casting.

# Your first evaluation

Bind the three types once with [For], then build a dataset and run it. Use [any]
for any type parameter you don't need to pin down:

	s := evals.For[string, string, any]()

	dataset := s.Dataset("capitals",
		s.Case("What is the capital of France?").Expect("Paris"),
	).With(s.EqualsExpected())

	answer := func(ctx context.Context, q string) (string, error) {
		return "Paris", nil
	}

	report, err := dataset.Evaluate(context.Background(), answer)
	if err != nil {
		log.Fatal(err)
	}
	report.Print()

[For] returns a [Suite] that carries the type parameters, so you never repeat
[string, string, any] at the call sites. See the [Suite] examples for the full
builder.

# Built-in evaluators

The package ships evaluators for the common deterministic checks: [Equals],
[EqualsExpected], [Contains], [IsInstance], and [MaxDuration]. Add them to a
dataset with [Dataset.With], or to a single case with [CaseBuilder.Eval].

# Custom evaluators

An evaluator is any type with an Evaluate method. Return a result with [Score],
[Assertion], or [Category] — add a “why” with WithReason, or return several
named results at once with [Named]:

	type Closeness struct{}

	func (Closeness) Evaluate(ctx context.Context, ec *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
		if ec.Output == ec.ExpectedOutput {
			return evals.Score(1.0), nil
		}
		return evals.Score(0.0).WithReason("not an exact match"), nil
	}

That's the whole contract. The evaluator's name in the report defaults to its Go
type name; implement [NamedEvaluator] to choose another, [VersionedEvaluator] to
tag results with a version, or [SpecEvaluator] to make it serializable (see
below).

# Reports

[Dataset.Evaluate] returns an [EvaluationReport]. Render it as a table with
[EvaluationReport.Render], write it anywhere with [EvaluationReport.Fprint], or
print it to stdout with [EvaluationReport.Print]. [RenderOptions] controls which
columns appear. [EvaluationReport.Averages] gives you the aggregate summary, and
[EvaluationReport.CaseGroups] groups repeated runs when you set [Config.Repeat].

# Metrics and attributes

Inside a task, record extra data that flows into the report and to evaluators:
[IncrementMetric] for numeric counters (averaged across cases) and [SetAttribute]
for arbitrary values. Both are no-ops outside an evaluation, so instrumented code
is safe to call anywhere.

# Lifecycle hooks

For per-case setup and teardown — spin up a fixture, enrich the context before
evaluators run, clean up afterwards — implement [Lifecycle] (embed
[BaseLifecycle] for no-op defaults) and pass a factory to
[Dataset.EvaluateWithLifecycle].

# Saving and loading datasets

Datasets serialize to YAML or JSON with [Dataset.Save], and load back with
[LoadDataset]. Loading rebuilds evaluators through a [Registry]: call
[Registry.RegisterDefaults] for the built-ins, or [Registry.Register] your own.
Custom evaluators that should round-trip implement [SpecEvaluator].

# OpenTelemetry and Logfire

[Dataset.Evaluate] emits an OpenTelemetry span tree — an experiment span, a span
per case, a span per task run, and a span per evaluator — using the global
tracer provider. It costs nothing until you configure one. Point an OTLP exporter
at Logfire and call otel.SetTracerProvider to see your experiments in Logfire's
evaluation views. See the example under examples/logfire in the repository.

[Pydantic Evals]: https://ai.pydantic.dev/evals
[Pydantic Logfire]: https://logfire.pydantic.dev
*/
package evals
