## Why

There's no way to remove an entire stack (lineage) from the graph in one operation. When a stack is abandoned or fully merged, users must manually `detach` or `delete` each branch individually. A single command to tear down the current lineage would make cleanup fast and predictable.

## What Changes

- Add a new `st delete-stack` command that removes every branch in the current lineage from the graph
- By default, only the graph is modified (git branches are left intact), matching `detach` semantics
- Add an optional `--branches` flag to also delete the git branches
- After removal, check out the lineage's root branch (main/develop/etc.)
- Warn (and require `--force`) if any branches in the lineage have unpushed commits

## Capabilities

### New Capabilities
- `delete-stack`: Command to remove an entire lineage from the stack graph, with options to also delete git branches

### Modified Capabilities

## Impact

- New command in `cmd/st/` alongside existing `delete.go` and `detach.go`
- Touches `pkg/graph/` for bulk branch removal
- No breaking changes to existing commands or graph format
