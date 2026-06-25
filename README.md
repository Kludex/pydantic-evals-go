# Pydantic Evals (Go)

A Go port of [Pydantic Evals](https://ai.pydantic.dev/evals), a library for
evaluating non-deterministic ("stochastic") functions such as LLM calls and
agents.

It provides a small, idiomatic Go API for defining datasets of test cases,
running a task against them, scoring the results with built-in or custom
evaluators, and rendering a report.

This is a community port of the core of the Python library. It does not depend
on any LLM SDK and can be used to evaluate arbitrary functions.

## Install

```bash
go get github.com/pydantic/pydantic-evals-go
```

## Example

```go
package main

import (
	"context"
	"strings"

	evals "github.com/pydantic/pydantic-evals-go"
)

// matchAnswer is a custom evaluator: 1.0 for an exact match, 0.8 for a
// case-insensitive substring match, 0.0 otherwise.
type matchAnswer struct{}

func (matchAnswer) Evaluate(_ context.Context, ec *evals.EvaluatorContext[string, string, any]) (evals.EvaluatorOutput, error) {
	switch {
	case ec.Output == ec.ExpectedOutput:
		return evals.ScalarValue(evals.Float(1.0)), nil
	case strings.Contains(strings.ToLower(ec.Output), strings.ToLower(ec.ExpectedOutput)):
		return evals.ScalarValue(evals.Float(0.8)), nil
	default:
		return evals.ScalarValue(evals.Float(0.0)), nil
	}
}

func (matchAnswer) Spec() evals.EvaluatorSpec { return evals.NewSpec("MatchAnswer") }

func main() {
	c := evals.NewCase[string, string, any](
		"What is the capital of France?",
		evals.WithCaseName[string, string, any]("capital_question"),
		evals.WithExpectedOutput[string, string, any]("Paris"),
	)

	dataset, err := evals.NewDataset[string, string, any](
		"capital_eval",
		[]evals.Case[string, string, any]{c},
		evals.IsInstance[string, string, any]{TypeName: "string"},
		matchAnswer{},
	)
	if err != nil {
		panic(err)
	}

	answer := func(_ context.Context, question string) (string, error) {
		return "Paris", nil
	}

	report, err := dataset.Evaluate(context.Background(), answer)
	if err != nil {
		panic(err)
	}

	report.Print(evals.RenderOptions{
		IncludeInput:  true,
		IncludeOutput: true,
	})
}
```

This prints:

```
                                    Evaluation Summary: task
┏━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━━━━┳━━━━━━━━━━┓
┃ Case ID          ┃ Inputs                         ┃ Outputs ┃ Scores            ┃ Assertions ┃ Duration ┃
┡━━━━━━━━━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━━━━╇━━━━━━━━━━┩
│ capital_question │ What is the capital of France? │ Paris   │ MatchAnswer: 1.00 │ ✔          │    ...   │
├──────────────────┼────────────────────────────────┼─────────┼───────────────────┼────────────┼──────────┤
│ Averages         │                                │         │ MatchAnswer: 1.00 │ 100.0% ✔   │    ...   │
└──────────────────┴────────────────────────────────┴─────────┴───────────────────┴────────────┴──────────┘
```

## Concepts

- **`Case[I, O, M]`** — one test scenario: `Inputs` (type `I`), an optional
  `ExpectedOutput` (`O`), optional `Metadata` (`M`), and optional case-specific
  evaluators. Build one with `NewCase` and the `With*` options.
- **`Dataset[I, O, M]`** — a named collection of cases plus dataset-level
  evaluators. Build one with `NewDataset`.
- **`TaskFunc[I, O]`** — the function under evaluation,
  `func(ctx, inputs I) (O, error)`.
- **`Evaluator[I, O, M]`** — scores a task result. It receives an
  `*EvaluatorContext[I, O, M]` and returns an `EvaluatorOutput`. Results that are
  `Bool` become assertions, `Int`/`Float` become scores, and `Label` (string)
  become categorical labels.
- **`EvaluationReport[I, O, M]`** — the result of `Dataset.Evaluate`, with the
  per-case results, failures, and an `Averages` summary. Render it with
  `Render`/`Print`.

The three type parameters are inputs, output, and metadata. Use `any` for any of
them you don't need to type precisely.

### Built-in evaluators

- `Equals` — output equals a fixed value.
- `EqualsExpected` — output equals the case's expected output.
- `Contains` — output contains a value (substring, slice membership, or map
  key/value containment).
- `IsInstance` — the output's Go type name matches.
- `MaxDuration` — the task ran within a time limit.

### Custom evaluators

Implement the `Evaluator[I, O, M]` interface (`Evaluate` + `Spec`). Return one of
`ScalarValue`, `Reason`, `ScalarMapOutput`, or `ReasonMapOutput`. Optionally
implement `NamedEvaluator` to customize the report name or `VersionedEvaluator`
to tag results with a version.

### Metrics and attributes

Inside a task, call `evals.IncrementMetric(ctx, name, amount)` and
`evals.SetAttribute(ctx, name, value)` to record data that surfaces on the
`EvaluatorContext` and the `ReportCase`. Metrics are averaged in the report
summary.

### Lifecycle hooks

Pass `WithLifecycle` to `Evaluate` to run per-case `Setup`, `PrepareContext`
(enrich the context before evaluators), and `Teardown` hooks. Embed
`BaseLifecycle[I, O, M]` for no-op defaults.

### Serialization

Datasets can be saved to and loaded from YAML or JSON with `Dataset.Save` and
`LoadDataset`. Loading reconstructs evaluators through a `Registry`; call
`RegisterDefaults` for the built-ins, or `Register` your own factories.

## Scope

This port covers the core of Pydantic Evals: cases, datasets, the evaluation
engine, the non-LLM built-in evaluators, metrics/attributes, lifecycle hooks,
reporting, and YAML/JSON serialization. It intentionally omits the parts of the
Python library that are tied to Python-specific infrastructure: OpenTelemetry
span trees (and the `HasMatchingSpan` evaluator), the `LLMJudge` evaluator,
online evaluation, and the statistical report evaluators. The `Registry`
extension point lets you add such evaluators yourself.

## License

MIT, same as Pydantic Evals. See [LICENSE](./LICENSE).
