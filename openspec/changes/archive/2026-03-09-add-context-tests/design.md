## Context

The `pkg/context` package provides `Load()` and `Save()` for the stack graph, supporting both local file storage (`DefaultGraphPath`) and shared git ref storage (`SharedGraphRef`). It also has `IsTrunkBranch()` for root auto-detection. The package depends on `pkg/git.Runner` and `pkg/graph` for actual git operations and graph serialization.

## Goals / Non-Goals

**Goals:**
- Test `IsTrunkBranch()` for all known trunk names and non-trunk names
- Test `Load()` for local file, shared ref, and new-graph fallback paths
- Test `Save()` for both local and shared storage modes
- Test error cases (not a git repo, corrupt graph data)

**Non-Goals:**
- Testing the underlying `git.Runner` or `graph` packages (they have their own tests)
- Mocking git — tests will use real git repos via `internal/testutil`

## Decisions

### Decision 1: Use `internal/testutil.NewGitRepo()` for test isolation

Tests will create real temporary git repos using the existing test helper. This ensures git operations work end-to-end and matches the testing pattern used by other packages (`pkg/git`, `pkg/backup`, etc.).

Alternative considered: Mocking `git.Runner` — would require an interface extraction and wouldn't test the actual integration.

### Decision 2: Test shared ref path by writing a blob ref manually

To test loading from shared refs, tests will manually write a graph JSON blob to `refs/staccato/graph` using `git.Runner.WriteBlobRef()`, then call `Load()`. This simulates the state after `st graph share`.

## Risks / Trade-offs

- **Tests depend on real git** — requires git to be installed. Acceptable since all existing tests have this dependency.
- **No parallel tests** — uses `os.Chdir` indirectly through `testutil`. Mitigation: don't use `t.Parallel()`.
