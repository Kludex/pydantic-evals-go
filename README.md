# Pydantic Evals for Go

<p align="center"><em>Evaluate non-deterministic functions — LLM calls, agents, pipelines — with type-safe, idiomatic Go.</em></p>

---

**Documentation**: <a href="https://pkg.go.dev/github.com/Kludex/pydantic-evals-go">pkg.go.dev/github.com/Kludex/pydantic-evals-go</a>

**Source**: <a href="https://github.com/Kludex/pydantic-evals-go">github.com/Kludex/pydantic-evals-go</a>

---

A Go port of [Pydantic Evals](https://ai.pydantic.dev/evals). You bring a function to test; it runs over a set of cases, scores each result with evaluators you choose, and gives you a report you can print, diff, serialize, or ship to [Pydantic Logfire](https://logfire.pydantic.dev).

It's like a test framework — but for code whose output you *can't* pin down with a single `==`.

## Install

```bash
go get github.com/Kludex/pydantic-evals-go
```

## Example

Bind your input, output, and metadata types once with `For`, build a dataset, and run it. Point an OTLP exporter at [Pydantic Logfire](https://logfire.pydantic.dev) and every run shows up in Logfire's evaluation views — the experiment, each case, each task call, each evaluator:

```go
package main

import (
	"context"
	"os"

	evals "github.com/Kludex/pydantic-evals-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

func main() {
	ctx := context.Background()

	// Send the evaluation trace tree to Logfire (set LOGFIRE_TOKEN to a write token).
	exporter, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL("https://logfire-us.pydantic.dev/v1/traces"),
		otlptracehttp.WithHeaders(map[string]string{"Authorization": os.Getenv("LOGFIRE_TOKEN")}),
	)
	if err != nil {
		panic(err)
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exporter))
	otel.SetTracerProvider(tp)
	defer tp.Shutdown(ctx) // flush before exit

	s := evals.For[string, string, any]()

	dataset := s.Dataset("capitals",
		s.Case("What is the capital of France?").Name("france").Expect("Paris"),
	).With(s.EqualsExpected())

	answer := func(ctx context.Context, question string) (string, error) {
		return "Paris", nil
	}

	report, err := dataset.Evaluate(ctx, answer, evals.Config{Name: "capitals-experiment"})
	if err != nil {
		panic(err)
	}
	report.Print(evals.RenderOptions{IncludeInput: true, IncludeOutput: true, IncludeAverages: true})
}
```

It prints the summary locally and sends the full trace to Logfire:

```
          Evaluation Summary: capitals-experiment
┏━━━━━━━━━━┳━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━┳━━━━━━━━━┳━━━━━━━━━━━━┓
┃ Case ID  ┃ Inputs                         ┃ Outputs ┃ Assertions ┃
┡━━━━━━━━━━╇━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━╇━━━━━━━━━╇━━━━━━━━━━━━┩
│ france   │ What is the capital of France? │ Paris   │ ✔          │
├──────────┼────────────────────────────────┼─────────┼────────────┤
│ Averages │                                │         │ 100.0% ✔   │
└──────────┴────────────────────────────────┴─────────┴────────────┘
```

Tracing is optional and zero-cost until you configure a provider — drop the exporter lines and the same program just prints the table. The OpenTelemetry packages live in their own modules:

```bash
go get go.opentelemetry.io/otel \
       go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp \
       go.opentelemetry.io/otel/sdk
```

## Custom evaluators

An evaluator is any type with an `Evaluate` method. Return `Score`, `Assertion`, or `Category`:

```go
type Closeness struct{}

func (Closeness) Evaluate(ctx context.Context, ec *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	if ec.Output == ec.ExpectedOutput {
		return evals.Score(1.0), nil
	}
	return evals.Score(0.0).WithReason("not an exact match"), nil
}
```

Add it with `.With(Closeness{})`. Booleans become assertions, numbers become scores, strings become labels — and the report figures out the columns.

## What's in the box

- **Built-in evaluators** — `Equals`, `EqualsExpected`, `Contains`, `IsInstance`, `MaxDuration`.
- **Concurrent evaluation** with `Config{MaxConcurrency, Repeat, ...}`.
- **Metrics & attributes** recorded from inside your task (`IncrementMetric`, `SetAttribute`).
- **Lifecycle hooks** for per-case setup and teardown.
- **YAML/JSON** dataset save & load.
- **OpenTelemetry** tracing, exportable to Logfire (shown above) — no-op until you configure a provider.

See the [package documentation](https://pkg.go.dev/github.com/Kludex/pydantic-evals-go) for the full guide and runnable examples, and [`examples/`](./examples) for complete programs — including agents built on the OpenAI Go SDK, Genkit, and Eino, each evaluated and traced to Logfire.

## Scope

This port covers the core of Pydantic Evals: cases, datasets, the evaluation engine, the non-LLM built-in evaluators, metrics, lifecycle hooks, reporting, serialization, and OpenTelemetry tracing. It omits the Python-infrastructure-specific pieces — the in-evaluator span-tree query API (and `HasMatchingSpan`), the `LLMJudge` evaluator, online evaluation, and statistical report evaluators. The `Registry` lets you add your own.

## License

MIT, same as Pydantic Evals. See [LICENSE](./LICENSE).
