## Why

The `pkg/output` package (68 lines, 20+ methods) handles all CLI output formatting but has no unit tests. It is only tested indirectly through E2E tests, which makes it fragile to refactor and easy to break silently.

## What Changes

- Add a comprehensive unit test file for `pkg/output` covering:
  - Core formatting methods (`Success`, `Warning`, `Error`, `Info`, `Print`, `Println`)
  - Verbose/non-verbose behavior of `Info()`
  - Convenience methods (`BranchCreated`, `RestackComplete`, `ConflictDetected`, etc.)
  - `StackLog` tree rendering with nested branches
  - `StackStatus` with PR status annotations
  - `formatPRStatus` edge cases (merged, closed, open/draft/approved/changes-requested, CI status)

## Capabilities

### New Capabilities
- `output-unit-tests`: Unit test coverage for the `pkg/output` package, validating formatting, icon usage, verbose gating, and tree rendering.

### Modified Capabilities

## Impact

- `pkg/output/output_test.go`: New test file
