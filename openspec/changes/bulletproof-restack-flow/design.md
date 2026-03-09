## Context

The restack → conflict → continue → restore flow is staccato's most critical user path. A user with a stack like `main → s1 → s2 → s3` needs to:
1. Run `st restack` (or `st sync`) and have all branches rebase cleanly, OR
2. Hit a conflict, resolve it, run `st continue`, and have the rest of the stack rebase — potentially hitting more conflicts along the way
3. If things get too hairy, run `st restore --all` and get back to exactly where they started

The current implementation handles the happy path and basic conflict cases well. However, several edge cases lack test coverage: multi-conflict sequences, graph state consistency after partial failures, rerere integration, and the restore-after-partial-continue scenario.

## Goals / Non-Goals

**Goals:**
- Comprehensive E2E test coverage for every step of the restack → conflict → continue cycle
- Test multi-conflict scenarios (conflict at s1, resolve, continue, conflict at s2, resolve, continue to completion)
- Test abort-and-restore mid-flow (restack conflicts, user restores instead of continuing)
- Test restore-after-partial-continue (restack conflicts at s1, continue succeeds, conflicts at s2, user restores)
- Verify graph state consistency at every step of a partial restack
- Verify rerere prevents re-resolving the same conflict
- Fix any bugs discovered during test hardening
- Test sync conflict → continue interaction

**Non-Goals:**
- Redesigning the restack engine architecture
- Adding new CLI commands or flags
- Changing backup naming conventions
- Adding interactive conflict resolution UI
- Performance optimization of restack operations

## Decisions

### Test-first approach using existing E2E patterns
Tests will follow the existing pattern in `main_test.go`: build binary, set up repo with `setupRepoWithStack`, create conflicting commits, run `st` commands, and verify git state + graph JSON. This keeps all tests consistent and exercises the real binary.

**Alternative considered**: Unit-testing the restack engine in isolation. Rejected because the critical bugs are in the integration between restack, graph persistence, backup creation, and git state — which only E2E tests can catch.

### Create conflicts via divergent file edits
To reliably create rebase conflicts in tests, we'll use the pattern: commit a change to a file on the parent branch after the child was created, where the child also modified the same file. This is deterministic and mirrors real usage.

### Verify state at intermediate points
Tests will check graph SHAs, branch HEAD SHAs, and file contents at intermediate points (after conflict, after continue, after second conflict, after second continue). This catches the graph-desync issues identified in the analysis.

### Test rerere by restacking twice with same conflict
To verify rerere works: restack → conflict → resolve → continue → make same conflict happen again → restack again → verify no manual resolution needed. This matches the real scenario where a user restacks, resolves, then needs to restack again later.

## Risks / Trade-offs

- **[Risk] Tests may be slow due to multiple git operations** → Acceptable since these are E2E tests that need real git state. Keep test count reasonable (target 6-8 new test functions).
- **[Risk] Discovered bugs may require non-trivial fixes** → Handle incrementally — fix bugs in separate tasks after test failures identify them.
- **[Risk] Rerere behavior may vary across git versions** → Test defensively by checking if rerere auto-resolved rather than asserting it must.
- **[Risk] Graph state fixes could change existing behavior** → Any fixes will preserve the current spec behavior; only fix genuinely broken invariants.
