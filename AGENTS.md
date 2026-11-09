# pydantic-evals-go

An idiomatic Go port of [`pydantic_evals`](https://github.com/pydantic/pydantic-ai/tree/main/pydantic_evals). `.upstream-sync.json` pins the upstream commit the port is synced against.

## Code

Write and review all Go code as if you were **Dave Cheney**: simplicity first, small interfaces discovered at the point of use, usable zero values, explicit errors handled once, no speculative abstraction, clarity over cleverness. Before finishing any change, re-read it with that eye and remove what a careful reviewer would question.

- The library lives under `evals/`. Go files mirror the upstream Python modules (`dataset.py` -> `evals/dataset.go`).
- Examples live under `examples/`. Examples with third-party dependencies are their own modules, so `evals` stays dependency-light.
- `context.Context` is always the first parameter and the only cancellation carrier - never stored in structs.
- Avoid comments; a clear name and a precise type are the documentation.

## Docs

Write all documentation (README, doc comments that surface in godoc) as if you were **Sebastián Ramírez** writing the FastAPI docs, minus the emoji: short plain sentences, second person, lead with a complete runnable example, explain the why behind every default, structure as reference not narrative.

## Testing

- Test through the public API only - never import internals or test private functions.
- 100% coverage, enforced by `scripts/coverage.sh`; exclusions need a `// pragma: no cover - <reason>` comment.

## Workflow

- `gofmt`, `go vet`, `golangci-lint`, `go test ./...`, and `scripts/coverage.sh` must pass before any commit.
- Never rebase, never force push, never amend pushed commits.
