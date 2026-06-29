---
emoji: "🔄"
name: "Agentic Evals Sync"
description: "Detect changes in upstream pydantic_evals (pydantic/pydantic-ai) since the last sync and open a draft PR porting them into this Go repo."
on:
  workflow_dispatch:
  schedule: weekly on monday
# Dormant until enabled. Set the AGENTIC_WORKFLOWS_ENABLED repo variable to
# 'true' once the engine + keys are wired up. Acts as a global kill switch.
if: ${{ vars.AGENTIC_WORKFLOWS_ENABLED == 'true' }}
runs-on: ubuntu-latest
permissions:
  contents: read
  pull-requests: read
concurrency:
  group: ${{ github.workflow }}-evals-sync
  cancel-in-progress: true
tools:
  bash:
    - "git status"
    - "git log:*"
    - "git diff:*"
    - "git show:*"
    - "git rev-parse:*"
    - "git ls-files:*"
    - "rg:*"
    - "gofmt:*"
    - "go build:*"
    - "go vet:*"
    - "go test:*"
  github:
    toolsets: [repos, pull_requests]
safe-outputs:
  threat-detection: false
  noop:
  create-pull-request:
    max: 1
    draft: true
    title-prefix: "[evals-sync] "
    labels: [agentic-workflows, upstream-sync]
    base-branch: main
    branch-prefix: evals-sync/
timeout-minutes: 60
max-turns: 200
# Disable AI Credits accounting — Fireworks bills directly. Drop these two
# lines if you wire up first-party Anthropic billing instead.
max-ai-credits: -1
max-daily-ai-credits: -1
engine:
  id: claude
  # Talks to Fireworks's Anthropic-compatible endpoint, mirroring the platform
  # repo's setup. To use first-party Anthropic instead, delete `api-target`,
  # the `env` block, and the `api.fireworks.ai` network entry, then set the
  # ANTHROPIC_API_KEY secret directly. A port is translation-heavy — prefer a
  # capable model.
  model: claude-sonnet-4-5
  api-target: api.fireworks.ai
  env:
    ANTHROPIC_BASE_URL: https://api.fireworks.ai/inference
    ANTHROPIC_API_KEY: ${{ secrets.FIREWORKS_API_KEY }}
    ANTHROPIC_MODEL: accounts/fireworks/models/minimax-m3
network:
  allowed:
    - defaults
    - api.fireworks.ai
imports:
  - shared/checkout.md
  - shared/rigor.md
runtimes:
  go:
    version: "1.25"
pre-agent-steps:
  - name: Fetch upstream pydantic_evals diff since last sync
    # Reads the pinned upstream SHA from .upstream-sync.json, shallow-clones
    # just enough of pydantic/pydantic-ai to reach it, and dumps the diff of
    # the evals subpath (pinned SHA -> main) plus the file inventory to disk.
    # The agent reads these files instead of cloning/diffing itself.
    shell: bash
    env:
      GH_TOKEN: ${{ github.token }}
    run: |
      set -euo pipefail
      mkdir -p /tmp/gh-aw/agent

      REPO="$(jq -r .repo .upstream-sync.json)"
      SUBPATH="$(jq -r .subpath .upstream-sync.json)"
      BASE_SHA="$(jq -r .commit .upstream-sync.json)"
      echo "Upstream: $REPO  subpath: $SUBPATH  pinned: $BASE_SHA"

      # Pin the upstream surface. The agent has write access via safe-outputs,
      # so a tampered .upstream-sync.json must not be able to redirect the clone
      # at an attacker-controlled repo or widen the ingested path.
      if [ "$REPO" != "pydantic/pydantic-ai" ]; then
        echo "::error::unexpected upstream repo: $REPO"; exit 1
      fi
      if [ "$SUBPATH" != "pydantic_evals/pydantic_evals" ]; then
        echo "::error::unexpected upstream subpath: $SUBPATH"; exit 1
      fi
      if ! printf '%s' "$BASE_SHA" | grep -Eq '^[0-9a-f]{40}$'; then
        echo "::error::pinned commit is not a 40-char SHA: $BASE_SHA"; exit 1
      fi

      git clone --filter=blob:none "https://github.com/${REPO}.git" /tmp/upstream
      HEAD_SHA="$(git -C /tmp/upstream rev-parse origin/main)"
      if ! printf '%s' "$HEAD_SHA" | grep -Eq '^[0-9a-f]{40}$'; then
        echo "::error::resolved HEAD is not a 40-char SHA: $HEAD_SHA"; exit 1
      fi
      echo "$HEAD_SHA" > /tmp/gh-aw/agent/upstream-head-sha.txt
      echo "Upstream main HEAD: $HEAD_SHA"

      if [ "$BASE_SHA" = "$HEAD_SHA" ]; then
        echo "Already at upstream HEAD — nothing to sync." \
          > /tmp/gh-aw/agent/sync-status.txt
        : > /tmp/gh-aw/agent/upstream.diff
        : > /tmp/gh-aw/agent/upstream-changed-files.txt
      else
        echo "Upstream advanced ${BASE_SHA}..${HEAD_SHA}" \
          > /tmp/gh-aw/agent/sync-status.txt
        # Fail closed: a swallowed diff/log error would hand the agent an empty
        # or partial diff while sync-status claims upstream advanced.
        git -C /tmp/upstream diff --stat "${BASE_SHA}..${HEAD_SHA}" -- "$SUBPATH" \
          > /tmp/gh-aw/agent/upstream-changed-files.txt
        git -C /tmp/upstream diff "${BASE_SHA}..${HEAD_SHA}" -- "$SUBPATH" \
          > /tmp/gh-aw/agent/upstream.diff
        # Commit subjects in the window, for PR-body context.
        git -C /tmp/upstream log --oneline "${BASE_SHA}..${HEAD_SHA}" -- "$SUBPATH" \
          > /tmp/gh-aw/agent/upstream-commits.txt
      fi

      echo "Wrote:"; wc -l /tmp/gh-aw/agent/* 2>/dev/null || true
---

# Agentic Evals Sync

This repo (`${{ github.repository }}`) is a Go port of **`pydantic_evals`**,
which lives upstream inside `pydantic/pydantic-ai` at
`pydantic_evals/pydantic_evals/`. The last upstream commit this port was synced
against is pinned in `.upstream-sync.json`.

Your job: port any upstream changes since that pin into idiomatic Go and open a
**draft PR** that includes the bumped pin. Most weeks there may be nothing
behavioral to port — `safeoutputs noop` is a perfectly good outcome.

## Process

1. **Read the sync state.**
   - `Read /tmp/gh-aw/agent/sync-status.txt`. If it says "Already at upstream
     HEAD", call `safeoutputs noop` and stop.
   - `Read /tmp/gh-aw/agent/upstream-changed-files.txt` — the `--stat` of the
     upstream evals subpath between the pinned SHA and main.
   - `Read /tmp/gh-aw/agent/upstream-commits.txt` — commit subjects in the
     window (use these for the PR body).
   - `Read /tmp/gh-aw/agent/upstream.diff` — the full upstream diff. This is
     your source of truth for what changed.

2. **Triage each upstream change.** For every changed upstream file, decide:
   - **Port it** — the change alters behavior the Go port reproduces (an
     evaluator's logic, report rendering, serialization shape, dataset
     handling, a bug fix). Find the corresponding Go file in this repo (the Go
     names mirror the Python module names: `dataset.py` -> `dataset.go`,
     `reporting/` -> `render.go`/`report.go`, `evaluators/common.py` ->
     `builtin.go`, etc. — grep to confirm) and translate the change.
   - **Skip it** — Python-only with no Go counterpart (typing/`Protocol`
     gymnastics, the `pytest` plugin, packaging, pure-docs). Note it as skipped.

3. **Implement the smallest faithful port.** Match the existing Go style: error
   handling, naming, comment density, the generic `[I, O, M]` parameterization.
   No drive-by refactors, no new dependencies unless the change requires one.

4. **Mirror tests when they exist.** If the upstream diff touched a test that
   covers behavior the Go port has, extend or add the matching `_test.go` case.

5. **Validate.** Run `gofmt -l .` (expect no output), `go vet ./...`,
   `go build ./...`, and `go test ./...`. Fix what you broke.

6. **Bump the pin.** Edit `.upstream-sync.json`: set `commit` to the SHA in
   `/tmp/gh-aw/agent/upstream-head-sha.txt` and `synced_at` to today's date.
   Do this **even if you skipped every change** as Python-only — the pin should
   advance so the next run doesn't re-surface the same already-triaged diff.
   (In that case, the PR is just the pin bump with a body explaining why nothing
   was ported.)

7. **Open a draft PR** via `safeoutputs create_pull_request`. **Leave your file
   edits uncommitted in the working tree** — gh-aw branches, commits, and pushes
   for you. Do not run `git checkout -b` / `git add` / `git commit` yourself.
   - **Title:** one line, e.g. `Sync pydantic_evals up to <short-sha>`.
   - **Body:**
     > Ports upstream `pydantic_evals` changes (`<base>..<head>`) into the Go port.
     >
     > ## Ported
     > - [upstream file] -> [Go file]: what changed and why.
     >
     > ## Skipped (Python-only)
     > - [upstream file]: reason.
     >
     > ## Validation
     > `gofmt` / `go vet` / `go build` / `go test` results.

## When to bail (`safeoutputs noop`)

- `sync-status.txt` says we're already at HEAD, or the diff is empty.
- Every upstream change is Python-only **and** you'd rather a human decide
  whether to bump the pin (otherwise prefer the pin-only PR in step 6).
- A change needs a design decision you can't make confidently — open the PR
  with the mechanical parts done and call out the open question in the body
  instead of guessing, or `noop` with a one-line reason if there's nothing
  mechanical to land.

## Final action — mandatory

Your **last** action MUST be a `safeoutputs` call: `create_pull_request` if you
have a port (or a pin bump) to land, or `noop` (with a brief `--message`)
otherwise. `safeoutputs --help` lists the sub-commands.
