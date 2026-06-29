# Agentic workflows

This repo uses [GitHub Agentic Workflows (`gh-aw`)](https://github.com/githubnext/gh-aw),
the same pattern as `pydantic/platform`. A workflow is a Markdown file with YAML
frontmatter (`*.md`) that **compiles** to a runnable GitHub Actions workflow
(`*.lock.yml`). The `.lock.yml` is what actually runs — both files are committed.

## Workflows

- **`agentic-evals-sync.md`** — weekly (and on-demand) check for changes in
  upstream `pydantic_evals` (`pydantic/pydantic-ai`) since the commit pinned in
  [`.upstream-sync.json`](../../.upstream-sync.json). If upstream advanced, an
  agent ports the relevant changes into Go, bumps the pin, and opens a **draft
  PR**. Python-only changes are skipped (and noted in the PR body).

## Enabling it

The workflow is **dormant** until you do all of the following:

1. **Install the CLI and compile** so the `.lock.yml` exists:
   ```bash
   gh extension install githubnext/gh-aw
   gh aw compile
   ```
   Commit the generated `agentic-evals-sync.lock.yml`. (CI / a human must run
   this — it isn't checked in yet because `gh-aw` wasn't available locally.)

2. **Set the kill switch.** Add a repo **variable** `AGENTIC_WORKFLOWS_ENABLED`
   = `true` (Settings → Secrets and variables → Actions → Variables). Setting it
   back to `false` disables every agentic workflow at once.

3. **Provide the model key.** As written, the engine talks to Fireworks's
   Anthropic-compatible endpoint (mirroring `platform`): add a repo **secret**
   `FIREWORKS_API_KEY`. To use first-party Anthropic instead, edit the `engine`
   block in `agentic-evals-sync.md` (drop `api-target`/`env`/the Fireworks
   network entry and the `max-ai-credits` lines), add an `ANTHROPIC_API_KEY`
   secret, then recompile.

## Maintaining the pin

`.upstream-sync.json` records the last-synced `pydantic/pydantic-ai` commit. The
sync workflow bumps it inside the PR it opens. If you ever port changes by hand,
update `commit` + `synced_at` there too so the next run starts from the right base.
