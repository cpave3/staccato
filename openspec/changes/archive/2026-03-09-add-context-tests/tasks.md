## 1. Test Setup

- [x] 1.1 Create `pkg/context/context_test.go` with imports and test helpers using `internal/testutil`

## 2. IsTrunkBranch Tests

- [x] 2.1 Test `IsTrunkBranch` returns true for `main`, `master`, `develop`, `trunk`
- [x] 2.2 Test `IsTrunkBranch` returns false for non-trunk names like `feature-x`

## 3. Load Tests

- [x] 3.1 Test `Load` reads graph from local `.git/stack/graph.json` file
- [x] 3.2 Test `Load` reads graph from shared ref `refs/staccato/graph`
- [x] 3.3 Test `Load` creates new graph when no graph exists (fallback)
- [x] 3.4 Test `Load` returns error for non-git directory

## 4. Save Tests

- [x] 4.1 Test `Save` writes graph to local file when no shared ref exists
- [x] 4.2 Test `Save` writes graph to shared ref when shared ref exists

## 5. Verify

- [x] 5.1 Run `go test ./pkg/context/ -v -count=1` and confirm all tests pass
- [x] 5.2 Run `task check` to confirm build + lint passes
