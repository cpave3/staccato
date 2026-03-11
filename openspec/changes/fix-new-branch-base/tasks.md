## 1. Git Layer

- [x] 1.1 Add `CreateAndCheckoutBranchFrom(name, startPoint string)` method to `pkg/git/git.go` that runs `git checkout -b <name> <startPoint>`

## 2. Command Fix

- [x] 2.1 Update `cmd/st/new.go` to use `CreateAndCheckoutBranchFrom(branchName, g.Root)` instead of `CreateAndCheckoutBranch(branchName)`

## 3. Tests

- [x] 3.1 Add E2E test in `cmd/st/main_test.go`: run `st new bar` while on a non-root branch with commits, verify `bar` does not contain those commits
- [x] 3.2 Run `task check` to verify all tests pass and linter is clean
