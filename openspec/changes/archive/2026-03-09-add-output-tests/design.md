## Context

The `pkg/output` package provides a `Printer` struct that wraps `fmt.Printf` with icon prefixes and a verbose flag. It has 20+ methods covering success/warning/error/info messages, branch-specific helpers, stack tree rendering, and PR status formatting. Currently it has zero unit tests — only indirect coverage through E2E tests in `cmd/st/main_test.go`.

## Goals / Non-Goals

**Goals:**
- Achieve unit test coverage for all public methods on `Printer`
- Verify verbose gating behavior (`Info` only prints when verbose is true)
- Test tree rendering (`StackLog`, `StackStatus`) with nested graph structures
- Test `formatPRStatus` for all PR state combinations

**Non-Goals:**
- Testing the `Printer` in integration with commands (already covered by E2E tests)
- Refactoring `Printer` to use `io.Writer` (useful but out of scope)
- Testing private methods beyond `formatPRStatus` (which is package-level)

## Decisions

### Decision 1: Capture stdout for assertions

Since `Printer` writes directly to `os.Stdout` via `fmt.Printf`, tests will capture stdout by redirecting `os.Stdout` to a pipe, running the method, then reading the output. This avoids refactoring `Printer` to accept an `io.Writer`.

Alternative considered: Refactoring to use `io.Writer` — cleaner but changes the public API and is out of scope for a test-only change.

### Decision 2: Use real `graph.Graph` for tree tests

`StackLog` and `StackStatus` take a `*graph.Graph`. Tests will construct real graph objects with known structures rather than mocking, since graph construction is simple and deterministic.

## Risks / Trade-offs

- **Stdout capture is slightly fragile** — if tests run in parallel, captured output could interleave. Mitigation: Don't use `t.Parallel()` for stdout-capturing tests.
- **`formatPRStatus` is unexported** — it's package-level (lowercase) so it's accessible from `output_test.go` in the same package. No issue.
