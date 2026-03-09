## 1. Filter stacked branches from recursive attach candidates

- [x] 1.1 In `doAttachRecursively`, when `stopIfTracked` is true, filter the candidate list to exclude branches already tracked in `g.Branches` (but keep the root branch as a valid candidate)
- [x] 1.2 Verify top-level `attachInteractive` path (`stopIfTracked=false`) still shows all branches including stacked ones

## 2. Tests

- [x] 2.1 Add E2E test: attach a chain of 3+ untracked branches recursively, verify that already-attached branches do not appear as candidates in subsequent TUI prompts (use model-level TUI test)
- [x] 2.2 Add model-level test: construct `attachTUI` with graph containing tracked branches, verify tracked branches are excluded from candidates when filtering is active
- [x] 2.3 Verify existing attach tests still pass
