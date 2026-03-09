## Why

During recursive `st attach` of a chain of branches, the TUI shows all local branches as parent candidates — including branches already placed in the stack during the current attach session. This creates confusion and risk: a user can accidentally select an already-stacked branch as a parent, creating an incorrect graph topology. Filtering out already-stacked branches reduces noise and prevents mistakes.

## What Changes

- During the recursive attach TUI flow, exclude branches that are already tracked in the graph from the candidate list
- The branch being attached is already excluded; this extends that logic to all graph-tracked branches (except the root, which remains a valid parent choice)
- Non-recursive attach (top-level `st attach`) continues to show all branches to support relocation use cases

## Capabilities

### New Capabilities
_(none)_

### Modified Capabilities
- `branch-attach`: TUI candidate list during recursive attachment filters out branches already in the stack graph

## Impact

- **Code**: `cmd/st/attach.go` — modify `doAttachRecursively` to pass graph-awareness into candidate filtering
- **Tests**: New E2E test verifying stacked branches are hidden during recursive attach; model-level TUI test
- **Dependencies**: None
- **Risk**: Very low — additive filter on existing candidate list. Top-level attach behavior unchanged.
