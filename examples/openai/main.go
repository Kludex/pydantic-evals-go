// Command openai evaluates a tiny "capital city" agent built on the official
// OpenAI Go SDK, sending the evaluation traces to Pydantic Logfire.
//
// It reads OPENAI_API_KEY and LOGFIRE_TOKEN from the environment.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	evals "github.com/Kludex/pydantic-evals-go"
	"github.com/openai/openai-go/v3"
	"github.com/openai/openai-go/v3/option"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

const model = openai.ChatModelGPT4_1Mini

// containsCity is a custom evaluator: the answer should mention the expected
// city (case-insensitive), and we score how concise it is.
type containsCity struct{}

func (containsCity) Evaluate(_ context.Context, ec *evals.EvaluatorContext[string, string, any]) (evals.Output, error) {
	hit := strings.Contains(strings.ToLower(ec.Output), strings.ToLower(ec.ExpectedOutput))
	conciseness := 1.0
	if words := len(strings.Fields(ec.Output)); words > 1 {
		conciseness = 1.0 / float64(words)
	}
	return evals.Named(
		"mentions_city", evals.Assertion(hit).WithReason("expected "+ec.ExpectedOutput),
		"conciseness", evals.Score(conciseness),
	), nil
}

func main() {
	shutdown, err := configureLogfire(context.Background())
	if err != nil {
		fmt.Fprintln(os.Stderr, "logfire:", err)
		os.Exit(1)
	}
	defer shutdown()

	client := openai.NewClient(option.WithAPIKey(os.Getenv("OPENAI_API_KEY")))

	// The agent: ask OpenAI for the capital city of a country.
	agent := func(ctx context.Context, country string) (string, error) {
		evals.SetAttribute(ctx, "country", country)
		completion, err := client.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
			Model: model,
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.SystemMessage("You are a geography expert. Answer with only the city name."),
				openai.UserMessage("What is the capital of " + country + "?"),
			},
		})
		if err != nil {
			return "", err
		}
		evals.IncrementMetric(ctx, "completion_tokens", float64(completion.Usage.CompletionTokens))
		return strings.TrimSpace(completion.Choices[0].Message.Content), nil
	}

	s := evals.For[string, string, any]()
	dataset := s.Dataset("capitals_openai",
		s.Case("France").Name("france").Expect("Paris"),
		s.Case("Japan").Name("japan").Expect("Tokyo"),
		s.Case("Brazil").Name("brazil").Expect("Brasília"),
		s.Case("Australia").Name("australia").Expect("Canberra"),
	).With(containsCity{})

	report, err := dataset.Evaluate(context.Background(), agent, evals.Config{
		Name:           "capitals (openai-go)",
		TaskName:       "openai_agent",
		MaxConcurrency: 4,
		Metadata:       map[string]any{"sdk": "openai-go", "model": string(model)},
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "evaluate:", err)
		os.Exit(1)
	}

	report.Print(evals.RenderOptions{
		IncludeInput:    true,
		IncludeOutput:   true,
		IncludeReasons:  true,
		IncludeAverages: true,
	})
}

func configureLogfire(ctx context.Context) (func(), error) {
	token := os.Getenv("LOGFIRE_TOKEN")
	if token == "" {
		return nil, fmt.Errorf("LOGFIRE_TOKEN is not set")
	}
	exp, err := otlptracehttp.New(ctx,
		otlptracehttp.WithEndpointURL("https://logfire-us.pydantic.dev/v1/traces"),
		otlptracehttp.WithHeaders(map[string]string{"Authorization": token}),
	)
	if err != nil {
		return nil, err
	}
	res, err := resource.New(ctx, resource.WithAttributes(
		semconv.ServiceName("evals-go-openai"),
		attribute.String("deployment.environment", "development"),
	))
	if err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(sdktrace.WithBatcher(exp), sdktrace.WithResource(res))
	otel.SetTracerProvider(tp)
	return func() {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = tp.Shutdown(ctx)
	}, nil
}
