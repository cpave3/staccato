## Context

Staccato manages stacked branches and their PRs via `st pr make` and `st sync`. Review feedback lives on GitHub across multiple PRs. Currently there's no way to aggregate this feedback. The forge package already shells out to `gh` for PR operations, and the MCP server exposes stack tools over stdio.

The user's workflow description specifies fetching three GitHub API endpoints per PR (inline comments, review submissions, issue comments), filtering bots, threading replies, and producing a severity-classified markdown document. Severity classification and deduplication require AI judgment — the Go code will structure the raw data and include classification prompt hints, leaving actual classification to the consuming AI agent.

## Goals / Non-Goals

**Goals:**
- Fetch all review comments for stack branches that have open PRs
- Support three scopes: current branch only, ancestors-to-current, whole stack (default)
- Filter bot noise while preserving substantive review bots (coderabbit, greptile)
- Thread reply comments under their parent inline comments
- Output structured markdown to stdout (for MCP/AI consumption) or to a file via `--out`
- Expose as `st_reviews` MCP tool with scope and optional output path parameters
- Keep the implementation deterministic — no AI calls in Go code

**Non-Goals:**
- AI-powered severity classification in Go (left to consuming agent)
- Fuzzy deduplication across PRs (hints provided, not implemented)
- Cross-referencing feedback with current codebase state (optional future work)
- Supporting forges other than GitHub

## Decisions

### 1. Use `gh api` for fetching, not Go HTTP client

**Decision**: Shell out to `gh api` via `exec.Command`, consistent with `pkg/forge`.

**Rationale**: `gh` handles auth, pagination (`--paginate`), and rate limiting. No need to manage GitHub tokens or implement pagination. This is the established pattern in `pkg/forge/github.go`.

**Alternative**: Use `go-github` or raw HTTP. Rejected — adds dependency, auth complexity, and diverges from existing patterns.

### 2. New `pkg/reviews` package

**Decision**: Create `pkg/reviews/` with `Fetch`, `Parse`, and `Format` functions.

**Rationale**: Separates concern from forge (which handles PR creation/status) and keeps the package testable. The forge interface doesn't need to grow — reviews are a read-only aggregation, not a PR operation.

### 3. Command placement: `st reviews` (top-level)

**Decision**: Top-level `st reviews` command rather than `st pr reviews`.

**Rationale**: Reviews span multiple PRs across the stack — it's a stack-level operation, not a single-PR operation. Consistent with `st log`, `st status` which are also stack-level views.

### 4. Scope selection via flags

**Decision**: `--current` for current branch only, `--to-current` for ancestors up to current, default is whole stack.

**Rationale**: Matches existing flag patterns (`st restack --to-current`). Whole stack is the most common use case for "what feedback do I need to address?"

### 5. Structured output with AI classification hints

**Decision**: Go code outputs all feedback items grouped by PR and type, with a preamble section containing classification instructions for AI consumers. The markdown includes a `<!-- CLASSIFICATION PROMPT -->` block that tells the AI agent how to classify severity and deduplicate.

**Rationale**: Severity classification requires understanding code context and intent — not feasible deterministically. By structuring the raw data well and including prompt instructions, the MCP consumer (an AI agent) can classify in-context. For CLI use with `--out`, the raw grouped output is still useful for human review.

### 6. Parallel fetching per PR

**Decision**: Use goroutines to fetch all three endpoints per PR concurrently, and fetch multiple PRs concurrently (with a concurrency limit of 5).

**Rationale**: A stack with 10 branches means 30 API calls. Serial fetching would be slow. The `gh` CLI handles rate limiting per-call.

## Risks / Trade-offs

- **[Rate limiting]** → Concurrency limit of 5 parallel `gh api` calls. GitHub's rate limit is 5000/hr for authenticated users; a 10-PR stack uses 30 calls.
- **[gh CLI dependency]** → Already required for `st pr make` and `st sync`. Not a new dependency.
- **[Large review volumes]** → Some PRs may have hundreds of comments (especially from bots). Bot filtering helps. The `--current` flag limits scope.
- **[No offline support]** → This command requires network access. Consistent with `st sync` and `st pr` which also require it.
- **[Classification quality]** → Depends on consuming AI agent. The prompt hints improve consistency but aren't guaranteed. Acceptable since the raw data is always available.
