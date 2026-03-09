## Why

The Graphite GT MCP uses a pragmatic architecture: 1 universal command wrapper + 1 learning resource. This means every CLI feature is instantly available to LLMs with zero MCP-specific code per command. Staccato's MCP has 20 individually coded tools, creating maintenance burden and feature gaps — the CLI already has capabilities (backup, restore) that aren't exposed via MCP, and adding new features requires writing both CLI and MCP code. By adopting a hybrid architecture (universal wrapper + selective typed tools + learning resource), we eliminate the gap permanently and reduce maintenance. New CLI commands added for humans automatically become available to LLMs.

Additionally, Graphite offers several CLI commands that Staccato lacks entirely — stack navigation (up/down/top/bottom), branch modification (amend + auto-restack), branch deletion, move/reparent, and abort. These should be added as first-class CLI commands for both human and MCP use.

## What Changes

### Architecture: Hybrid MCP model
- Add **`st_run`** universal wrapper tool that can execute any `st` subcommand (like Graphite's `run_gt_cmd`)
- Add **`st_learn`** prompt/resource that teaches LLMs the stacking workflow and command reference
- **Keep** high-value typed tools where structured JSON output matters (`st_log`, `st_status`, `st_current`, `st_pr`)
- **Deprecate/remove** typed MCP tools that are thin wrappers with no structured output benefit — these become accessible through `st_run`

### New CLI commands (available to both humans and LLMs)
- **`st up`** / **`st down`**: Navigate one level up/down the stack
- **`st top`** / **`st bottom`**: Jump to the tip or base of the current stack
- **`st modify`**: Amend current branch commit and auto-restack descendants
- **`st delete`**: Remove a branch from the stack with child reparenting
- **`st move`**: Reparent a branch onto a different parent with auto-restacking
- **`st abort`**: Cancel an in-progress rebase and restore pre-operation state

## Capabilities

### New Capabilities
- `mcp-universal-tool`: Universal `st_run` wrapper tool that executes any st subcommand, eliminating per-command MCP maintenance
- `mcp-learn-prompt`: Comprehensive learning resource teaching LLMs the stacking workflow, command semantics, and best practices
- `stack-navigation`: CLI commands for navigating the stack (up, down, top, bottom)
- `branch-modify`: Amend current branch commit with automatic downstream restacking
- `branch-delete`: Remove branch from stack with proper child reparenting and cleanup
- `branch-move`: Reparent a branch onto a different parent within the stack
- `rebase-abort`: Cancel in-progress rebase operations and restore pre-operation state

### Modified Capabilities
- `mcp-server`: Simplified to hybrid architecture — universal wrapper + selective typed tools + learning resource

## Impact

- **`pkg/mcp/`**: Add `st_run` tool, add `st_learn` prompt, remove thin-wrapper tools that `st_run` subsumes
- **`cmd/st/`**: New command files for `up`, `down`, `top`, `bottom`, `modify`, `delete`, `move`, `abort`
- **`pkg/restack/`**: May need `Abort()` function for cancelling in-progress rebases
- **`pkg/git/`**: May need `CommitAmend()`, `RebaseAbort()` methods
- **`pkg/graph/`**: May need `DeleteBranch()` with child reparenting logic
- **Backward compatibility**: Existing MCP typed tools (`st_log`, `st_status`, etc.) remain — this is additive, not breaking
