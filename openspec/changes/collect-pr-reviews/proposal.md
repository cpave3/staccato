## Why

When working with stacked PRs, review feedback is scattered across multiple GitHub PRs. Developers need to see all feedback from their stack in one place — grouped by severity, deduplicated, and ready for action. Currently this requires manually opening each PR and reading comments one by one.

## What Changes

- Add `st reviews` CLI command that collects review comments from PRs associated with stack branches
- Support three scopes: current branch (`--current`), ancestors up to current (`--to-current`), or the whole stack (default)
- Fetch inline comments, review submissions, and general PR comments via `gh api`
- Filter out bot noise (keep substantive review bots like coderabbit, greptile)
- Thread reply comments under their parents
- Output structured markdown with severity classification hints for AI consumption
- Support `--out <path>` flag to write to disk; without it, output to stdout (useful for MCP where the AI agent receives output directly)
- Expose as `st_reviews` MCP tool so AI agents can pull review feedback into context

## Capabilities

### New Capabilities
- `pr-reviews`: Collecting, filtering, threading, and outputting PR review feedback from GitHub for stack branches

### Modified Capabilities
- `mcp-server`: Adding `st_reviews` tool registration
- `pull-requests`: Adding `reviews` subcommand to `st pr` (or top-level `st reviews`)

## Impact

- **Code**: New `pkg/reviews/` package for fetching/parsing/formatting. New `cmd/st/reviews.go` command. New MCP tool registration in `pkg/mcp/`.
- **Dependencies**: Uses `gh api` (already a dependency pattern via `pkg/forge`). No new Go dependencies.
- **APIs**: Calls GitHub REST API via `gh api` for `pulls/{n}/comments`, `pulls/{n}/reviews`, `issues/{n}/comments`.
- **Output**: Markdown document with severity sections. Severity classification and deduplication are hinted via embedded prompt instructions — the Go code does not do AI classification itself, but structures output so an AI agent can classify when processing the result.
