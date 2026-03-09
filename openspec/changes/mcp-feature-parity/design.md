## Context

Staccato's MCP server currently has 20 individually coded tools. Graphite's MCP uses a single universal command wrapper (`run_gt_cmd`) + a learning resource (`learn_gt`). This means every Graphite CLI feature is instantly available via MCP, while Staccato has permanent feature gaps between CLI and MCP.

Additionally, Staccato lacks several CLI commands that Graphite offers: stack navigation (up/down/top/bottom), branch modification (amend + auto-restack), branch deletion with reparenting, move/reparent, and abort.

The existing MCP tools live in `pkg/mcp/` across 5 files (tools_stack.go, tools_branch.go, tools_management.go, tools_git.go, tools_sync.go). The CLI commands are in `cmd/st/` with cobra.

## Goals / Non-Goals

**Goals:**
- Add `st_run` universal wrapper tool so every CLI command is MCP-accessible without per-tool code
- Add `st_learn` resource to teach LLMs the stacking workflow
- Add new CLI commands (`up`, `down`, `top`, `bottom`, `modify`, `delete`, `move`, `abort`) usable by both humans and LLMs
- Keep high-value typed tools where structured JSON output matters

**Non-Goals:**
- Removing existing typed tools — they continue to work alongside `st_run`
- Replicating Graphite's exact command syntax or behavior
- Adding `gt submit`-style PR creation (GitHub API integration beyond current `st pr`)
- Adding `gt reorder` (interactive editor-based — not suitable for MCP and can be done via move)

## Decisions

### 1. Hybrid MCP architecture: universal wrapper + selective typed tools

**Decision**: Add `st_run` alongside existing typed tools, not replacing them.

**Rationale**: Typed tools (`st_log`, `st_status`, `st_current`, `st_pr`) return structured JSON that LLMs can parse reliably. A universal wrapper returns text output that requires parsing. The hybrid approach gives structured output where it matters and universal access everywhere else.

**Alternative considered**: Pure wrapper (like Graphite). Rejected because we lose structured JSON output and per-tool MCP annotations (read-only vs destructive).

**Alternative considered**: Keep adding typed tools for everything. Rejected because it's unsustainable maintenance-wise and creates permanent feature gaps.

### 2. `st_run` implementation: subprocess execution

**Decision**: `st_run` SHALL execute the `st` binary as a subprocess, capturing stdout/stderr. It takes a `command` string (the full command after `st`, e.g. `"log"`, `"up"`, `"restack --to-current"`).

**Rationale**: Reusing the compiled binary means the wrapper automatically picks up all commands including future ones. Calling Go functions directly would require importing and wiring every command handler, defeating the purpose.

**Trade-off**: Subprocess execution is slightly slower than direct function calls, but the simplicity and automatic forward-compatibility outweigh this.

**Safety**: `st_run` SHALL be annotated as mutating (not read-only, not destructive). The tool description SHALL list dangerous subcommands. The wrapper SHALL refuse to run `st mcp` recursively.

### 3. `st_learn` as embedded prompt + resource

**Decision**: Add a comprehensive markdown document at `pkg/mcp/prompts/learn-staccato.md` registered as both an MCP prompt and resource (like the existing `split-monolithic-pr` pattern).

**Rationale**: Follows the established pattern in `prompts.go`. The learn resource covers: what stacking is, st command reference, common workflows, and how the MCP tools map to the CLI.

### 4. New CLI commands as cobra commands in `cmd/st/`

**Decision**: Each new command (`up`, `down`, `top`, `bottom`, `modify`, `delete`, `move`, `abort`) gets its own file in `cmd/st/` following existing patterns (e.g., `up.go`, `down.go`). They use `getContext`/`saveContext` and `pkg/` libraries.

**Rationale**: Consistent with existing codebase. Once they exist as CLI commands, they're automatically available via `st_run` — no additional MCP work needed.

### 5. Navigation commands: use graph traversal

**Decision**: `st up` checks out the child branch (if exactly one child; prompts/errors if multiple). `st down` checks out the parent. `st top` follows children to the tip. `st bottom` checks out the first branch above root in the current lineage.

**Rationale**: Matches Graphite's semantics. Multiple children on `up`/`top` is an edge case — error with guidance to use `st switch` is the simplest approach.

### 6. `st modify`: amend + auto-restack

**Decision**: `st modify` stages all changes (with `--all` flag, or expects pre-staged), amends the HEAD commit, then restacks all downstream branches. Uses existing `restack.RestackLineage`.

**Rationale**: This is the most-requested workflow gap vs Graphite. The implementation is a composition of existing primitives: `git commit --amend` + `RestackLineage`.

### 7. `st delete`: reparent children to deleted branch's parent

**Decision**: `st delete <branch>` removes the branch from the graph. Children of the deleted branch are reparented to the deleted branch's parent. The git branch is also deleted. If the branch is current, checkout parent first.

**Rationale**: Matches Graphite's `gt delete` behavior. Reparenting to parent is the only sensible default — orphaning children would break the stack.

### 8. `st move`: reparent + restack

**Decision**: `st move --onto <target>` changes the current branch's parent to `<target>` in the graph, then restacks. Target must be in the stack.

**Rationale**: Enables restructuring stacks without manual graph editing. Implementation is: update `branch.Parent` and `branch.BaseSHA`, then call `RestackLineage`.

### 9. `st abort`: cancel in-progress rebase

**Decision**: `st abort` checks if a rebase is in progress, runs `git rebase --abort`, clears restack state, and restores from automatic backups if available.

**Rationale**: Currently the only way to abort is manual git commands. This gives a clean, integrated abort that also handles staccato's restack state file.

## Risks / Trade-offs

- **`st_run` subprocess overhead** → Acceptable for MCP use; sub-100ms per call. Typed tools remain for latency-sensitive structured queries.
- **`st_run` security** → Must block recursive `st mcp` calls. Must not allow shell injection (args passed as array, not shell string).
- **`st modify` data loss on amend** → Mitigated by creating automatic backups before amend (same pattern as `st insert`).
- **`st delete` irreversibility** → Could lose work if branch has unpushed commits. Mitigate with a confirmation flag (`--force`) or pre-deletion backup.
- **Multiple children ambiguity in `st up`/`st top`** → Error with helpful message listing children. User can then `st switch` or specify directly.
