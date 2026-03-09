## Why

The restack → conflict → continue → restore flow is the most critical user-facing workflow in staccato. Users depend on it to safely rebase entire stacks, recover from conflicts without data loss, and never resolve the same conflict twice. Currently there are gaps in test coverage for multi-conflict scenarios, graph state consistency after partial failures, and the continue/restore interaction. This change hardens the flow with comprehensive tests and fixes edge cases to make it truly bulletproof.

## What Changes

- Add comprehensive E2E tests covering the full restack → conflict → resolve → continue cycle across multi-branch stacks
- Add tests for multiple sequential conflicts (conflict at s1, resolve, continue, conflict at s2, resolve, continue)
- Add tests verifying `st restore --all` returns the exact pre-restack state (graph SHAs, branch contents, working tree)
- Add tests for abort-and-restore mid-conflict (restack hits conflict, user gives up, restores backup)
- Verify rerere integration works (same conflict doesn't require re-resolution on subsequent restacks)
- Add tests for graph state consistency after partial restack failure
- Fix any edge cases or bugs discovered during test hardening
- Ensure `st continue` after sync conflict works correctly

## Capabilities

### New Capabilities
- `restack-conflict-resolution`: End-to-end tests and hardening for the restack → conflict → continue cycle, including multi-conflict scenarios, graph consistency, and rerere integration
- `restack-restore-integrity`: Tests and fixes ensuring `st restore --all` returns to exact pre-restack state with correct graph SHAs, branch contents, and clean working tree

### Modified Capabilities
- `restack`: May need fixes for graph state consistency during partial failures
- `backup-restore`: May need fixes for children's BaseSHA updates after restore

## Impact

- **Code**: `pkg/restack/`, `pkg/backup/`, `cmd/st/restack.go`, `cmd/st/continue.go`, `cmd/st/restore.go`
- **Tests**: `cmd/st/main_test.go` — significant new E2E test cases
- **Dependencies**: None — all changes are internal
- **Risk**: Low — primarily adding tests and fixing edge cases found during testing
