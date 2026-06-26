// Command eino evaluates a tiny "capital city" agent built on CloudWeGo's Eino
// framework (with its OpenAI chat model), sending the traces to Pydantic Logfire.
//
// It reads OPENAI_API_KEY and LOGFIRE_TOKEN from the environment.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/cloudwego/eino-ext/components/model/openai"
	"github.com/cloudwego/eino/schema"
	evals "github.com/pydantic/pydantic-evals-go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
)

const model = "gpt-4o-mini"

// containsCity asserts the answer mentions the expected city and scores brevity.
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
	ctx := context.Background()

	shutdown, err := configureLogfire(ctx)
	if err != nil {
		fmt.Fprintln(os.Stderr, "logfire:", err)
		os.Exit(1)
	}
	defer shutdown()

	chatModel, err := openai.NewChatModel(ctx, &openai.ChatModelConfig{
		APIKey: os.Getenv("OPENAI_API_KEY"),
		Model:  model,
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, "new chat model:", err)
		os.Exit(1)
	}

	// The agent: ask Eino's chat model for the capital city of a country.
	agent := func(ctx context.Context, country string) (string, error) {
		evals.SetAttribute(ctx, "country", country)
		resp, err := chatModel.Generate(ctx, []*schema.Message{
			schema.SystemMessage("You are a geography expert. Answer with only the city name."),
			schema.UserMessage("What is the capital of " + country + "?"),
		})
		if err != nil {
			return "", err
		}
		return strings.TrimSpace(resp.Content), nil
	}

	s := evals.For[string, string, any]()
	dataset := s.Dataset("capitals_eino",
		s.Case("France").Name("france").Expect("Paris"),
		s.Case("Japan").Name("japan").Expect("Tokyo"),
		s.Case("Brazil").Name("brazil").Expect("Brasília"),
		s.Case("Australia").Name("australia").Expect("Canberra"),
	).With(containsCity{})

	report, err := dataset.Evaluate(ctx, agent, evals.Config{
		Name:           "capitals (eino)",
		TaskName:       "eino_agent",
		MaxConcurrency: 4,
		Metadata:       map[string]any{"sdk": "eino", "model": model},
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
		semconv.ServiceName("evals-go-eino"),
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
