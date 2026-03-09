## Why

The `pkg/context` package (89 lines) is the critical persistence layer that loads and saves the stack graph from either shared refs or local files, yet it has zero test coverage. It also contains the `IsTrunkBranch()` helper used for root auto-detection. Bugs here could silently corrupt graph state.

## What Changes

- Add a unit test file for `pkg/context` covering:
  - `IsTrunkBranch()` for known trunk names and non-trunk names
  - `Load()` from local file (default path)
  - `Load()` from shared ref when present
  - `Load()` fallback to new graph when no graph exists
  - `Load()` error when not in a git repository
  - `Save()` to local file
  - `Save()` to shared ref when shared mode is active

## Capabilities

### New Capabilities
- `context-unit-tests`: Unit test coverage for the `pkg/context` package, validating graph loading/saving from both local files and shared refs, and trunk branch detection.

### Modified Capabilities

## Impact

- `pkg/context/context_test.go`: New test file
