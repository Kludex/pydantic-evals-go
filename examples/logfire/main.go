// Command logfire runs a multi-faceted evaluation and exports the traces to
// Pydantic Logfire via OTLP, so the experiment shows up in Logfire's eval views.
//
// It reads the write token from the LOGFIRE_TOKEN env var (US region endpoint).
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	evals "github.com/pydantic/pydantic-evals-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

// configureLogfire wires the global OTel tracer provider to export to Logfire.
func configureLogfire(ctx context.Context, token string) (func(context.Context) error, error) {
	exp, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL("https://logfire-us.pydantic.dev/v1/traces"),
		otlptracehttp.WithHeaders(map[string]string{"Authorization": token}),
	)
	if err != nil {
		return nil, err
	}
	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName("evals-go-demo"),
		attribute.String("deployment.environment", "development"),
	))
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exp),
		sdktrace.WithResource(res),
	)
	otel.SetTracerProvider(tp)
	return tp.Shutdown, nil
}

// answerLength scores by closeness of the output length to the expected length.
type answerLength struct{}

func (answerLength) Evaluate(_ context.Context, ec *evals.EvaluatorContext[string, string, map[string]any]) (evals.EvaluatorOutput, error) {
	if !ec.HasExpectedOutput {
		return evals.ScalarMapOutput{}, nil
	}
	diff := len(ec.Output) - len(ec.ExpectedOutput)
	if diff < 0 {
		diff = -diff
	}
	score := 1.0 - float64(diff)/10.0
	if score < 0 {
		score = 0
	}
	return evals.Reason(evals.Float(score), fmt.Sprintf("length diff %d", diff)), nil
}

func (answerLength) Spec() evals.EvaluatorSpec { return evals.NewSpec("AnswerLength") }

// sentiment labels the output as a category.
type sentiment struct{}

func (sentiment) Evaluate(_ context.Context, ec *evals.EvaluatorContext[string, string, map[string]any]) (evals.EvaluatorOutput, error) {
	switch {
	case strings.Contains(ec.Output, "!"):
		return evals.ScalarValue(evals.Label("excited")), nil
	default:
		return evals.ScalarValue(evals.Label("neutral")), nil
	}
}

func (sentiment) Spec() evals.EvaluatorSpec { return evals.NewSpec("Sentiment") }

// flaky errors for a specific input, to exercise the evaluator-failure path.
type flaky struct{}

func (flaky) Evaluate(_ context.Context, ec *evals.EvaluatorContext[string, string, map[string]any]) (evals.EvaluatorOutput, error) {
	if ec.Inputs == "boom" {
		return nil, fmt.Errorf("flaky evaluator exploded on %q", ec.Inputs)
	}
	return evals.ScalarValue(evals.Bool(true)), nil
}

func (flaky) Spec() evals.EvaluatorSpec { return evals.NewSpec("Flaky") }

func main() {
	token := os.Getenv("LOGFIRE_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "LOGFIRE_TOKEN is not set")
		os.Exit(1)
	}

	ctx := context.Background()
	shutdown, err := configureLogfire(ctx, token)
	if err != nil {
		fmt.Fprintln(os.Stderr, "configure logfire:", err)
		os.Exit(1)
	}
	defer func() {
		// Flush all buffered spans before exit.
		flushCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if err := shutdown(flushCtx); err != nil {
			fmt.Fprintln(os.Stderr, "shutdown:", err)
		}
	}()

	cases := []evals.Case[string, string, map[string]any]{
		evals.NewCase[string, string, map[string]any]("hi",
			evals.WithCaseName[string, string, map[string]any]("greeting"),
			evals.WithExpectedOutput[string, string, map[string]any]("hello"),
			evals.WithMetadata[string, string, map[string]any](map[string]any{"difficulty": "easy"}),
		),
		evals.NewCase[string, string, map[string]any]("loud",
			evals.WithCaseName[string, string, map[string]any]("exclaim"),
			evals.WithExpectedOutput[string, string, map[string]any]("WOW!"),
		),
		evals.NewCase[string, string, map[string]any]("boom",
			evals.WithCaseName[string, string, map[string]any]("evaluator_error"),
			evals.WithExpectedOutput[string, string, map[string]any]("kaboom"),
		),
		evals.NewCase[string, string, map[string]any]("explode",
			evals.WithCaseName[string, string, map[string]any]("task_error"),
		),
	}

	dataset, err := evals.NewDataset[string, string, map[string]any](
		"demo_suite", cases,
		evals.EqualsExpected[string, string, map[string]any]{},
		answerLength{},
		sentiment{},
		flaky{},
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "new dataset:", err)
		os.Exit(1)
	}

	task := func(ctx context.Context, input string) (string, error) {
		if input == "explode" {
			return "", fmt.Errorf("task failed on %q", input)
		}
		evals.IncrementMetric(ctx, "input_chars", float64(len(input)))
		evals.SetAttribute(ctx, "echoed", true)
		switch input {
		case "hi":
			return "hello", nil
		case "loud":
			return "WOW!", nil
		case "boom":
			return "kaboom", nil
		default:
			return input, nil
		}
	}

	report, err := dataset.Evaluate(ctx, task,
		evals.WithName[string, string, map[string]any]("demo_experiment"),
		evals.WithTaskName[string, string, map[string]any]("echo_task"),
		evals.WithExperimentMetadata[string, string, map[string]any](map[string]any{"model": "demo-v1", "run": "ci"}),
		evals.WithRepeat[string, string, map[string]any](2),
	)
	if err != nil {
		fmt.Fprintln(os.Stderr, "evaluate:", err)
		os.Exit(1)
	}

	report.Print(evals.RenderOptions{
		IncludeInput:          true,
		IncludeOutput:         true,
		IncludeExpectedOutput: true,
		IncludeReasons:        true,
		IncludeAverages:       true,
		IncludeDurations:      true,
	})
	fmt.Printf("\n%d cases, %d failures\n", len(report.Cases), len(report.Failures))
}
