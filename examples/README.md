# Examples

Each example is its own Go module (with a `replace` pointing at the local
library) so the core package stays dependency-free. Run any of them from its own
directory with `go run .`.

| Example | What it shows |
| --- | --- |
| [`capitals`](./capitals) | The smallest possible evaluation — a deterministic task, no external services. |
| [`logfire`](./logfire) | A multi-faceted evaluation (scores, labels, metrics, failures, `Repeat`) exporting its trace tree to Logfire. |
| [`openai`](./openai) | Evaluating an agent built on the official [OpenAI Go SDK](https://github.com/openai/openai-go). |
| [`genkit`](./genkit) | Evaluating an agent built on [Firebase Genkit](https://genkit.dev) (OpenAI plugin). |
| [`eino`](./eino) | Evaluating an agent built on [CloudWeGo Eino](https://github.com/cloudwego/eino). |

## Agent examples (openai / genkit / eino)

These three evaluate the *same* task — "what is the capital of this country?" —
against a real LLM through three different Go agent frameworks, and send the
evaluation traces to [Pydantic Logfire](https://logfire.pydantic.dev). They share
the same dataset, the same custom evaluator (`mentions_city` assertion +
`conciseness` score), and the same Logfire wiring, so you can compare the
frameworks side by side.

They require two environment variables:

```bash
export OPENAI_API_KEY=sk-...
export LOGFIRE_TOKEN=pylf_v1_...   # a Logfire write token

cd examples/openai && go run .
```

Because the agent runs under the evaluation's OpenTelemetry context, each
framework's own instrumentation (e.g. Genkit's `generate` and model spans) nests
neatly inside the per-case spans in Logfire — you get the eval structure and the
agent internals in one trace.
