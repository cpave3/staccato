## Context

The `cmd/st/attach.go` file contains 12+ raw `fmt.Printf` calls with hardcoded icons (`✔`, etc.) for user-facing output. Every other command in the codebase uses the `output.Printer` abstraction, which provides consistent formatting and verbose-gated output. The attach command already receives a `Printer` via `getContext()` in `attachCmd()` but doesn't pass it to the helper functions.

## Goals / Non-Goals

**Goals:**
- Replace all `fmt.Printf` calls in `attach.go` with equivalent `Printer` methods
- Thread the `Printer` through all attach helper functions
- Maintain identical user-visible output (same icons, same messages)

**Non-Goals:**
- Changing the TUI output (rendered by Bubble Tea, not `fmt.Printf`)
- Adding verbose-only output where there currently is none
- Refactoring the attach logic itself

## Decisions

### Decision 1: Add `*output.Printer` parameter to helper functions

Functions `attachInteractive`, `attachRecursively`, `doAttachRecursively`, and `attachWithParent` will receive a `*output.Printer` parameter. The printer is already available in `attachCmd()` from `getContext()`.

Alternative considered: Storing printer as a field on `attachTUI` — this would only cover the TUI path, not `attachWithParent`.

### Decision 2: Map `fmt.Printf` calls to existing Printer methods

- `fmt.Printf("✔ ...")` → `printer.Success(...)`
- `fmt.Printf("...")` with no icon → `printer.Println(...)`
- Messages about parent not in stack → `printer.Println(...)`

This maintains the same output while using the standard abstraction.

## Risks / Trade-offs

- **Function signature changes** — callers of these helpers must be updated. Mitigation: all call sites are within the same file.
- **E2E test output may shift slightly** — `Printer.Success()` uses `SuccessIcon` constant which should be identical to `"✔"`. Low risk.
