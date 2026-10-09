## Why

A branch in a stack sometimes grows too large to review. Splitting it into smaller stacked branches is today a prompt-only workflow: the `split-monolithic-pr` prompt tells an agent to soft-reset, stage, commit and `st append` by hand. That prompt refers to tools that do not exist, leaves the working tree uncompilable partway through, and only advises the "nothing was lost" check. An agent can lose changes or strand the repository mid-split.

An agent is good at deciding where the boundaries go and poor at the git choreography. Staccato should own the choreography and guarantee the outcome.

## What Changes

- Add `st split describe [branch]`: a JSON inventory of a branch (commits, changed files, which commits touch which files, net totals against per-commit churn, rename pairs).
- Add `st split apply <plan.json | ->`: split a branch into stacked branches from a declarative JSON plan, with `--dry-run`, `--exec <cmd>` and `--json`.
- A plan lists slices bottom to top. Each slice takes its content from `commits` (existing commits, kept as they are), `paths` (pathspecs brought to their final content, with optional `overrides` for a file whose changes belong to two slices), or `rest` (everything left).
- Enforce that the top slice holds exactly the content the branch had, and build every slice before any branch moves, so a plan that cannot be carried out changes nothing.
- Re-parent descendant branches onto the top slice without a checkout.
- Add MCP tools `st_split_describe` and `st_split_apply`.
- Rewrite the `split-monolithic-pr` prompt around the new commands.
- `st_run` refuses `--exec`.

## Capabilities

### New Capabilities
- `branch-split`: describing a branch for planning, validating and applying a split plan, the completeness and atomicity guarantees, descendant handling, verification, and worktree safety

### Modified Capabilities
- `mcp-server`: adds `st_split_describe` and `st_split_apply`, changes the content of the `split-monolithic-pr` prompt, and restricts `st_run`

## Impact

- **Code**: new `pkg/split/` package shared by the CLI and the MCP tools. New `cmd/st/split.go` and `pkg/mcp/tools_split.go`. `pkg/git` gains `Output`, `RunWith` and `CherryPickTree`. `pkg/context` gains `Reload`.
- **Dependencies**: none added. Reordering commits needs git 2.40 or newer (`git merge-tree --write-tree --merge-base`); every other part works with older git.
- **Behavior of existing commands**: unchanged, except that `st_run` now rejects a command containing `--exec`.
- **Platforms**: `--exec` uses process groups and signals, which matches the linux and darwin build targets.
