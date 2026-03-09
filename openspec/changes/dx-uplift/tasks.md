## 1. Detached HEAD Guard

- [x] 1.1 Add `requireBranch` helper in `cmd/st/main.go` that checks `GetCurrentBranch()` returns a real branch (not "HEAD") and returns clear error
- [x] 1.2 Add `requireBranch` call to all commands that operate on current branch: `new`, `append`, `insert`, `restack`, `continue`, `attach`, `sync`, `log`, `detach`, `status`
- [x] 1.3 Add E2E test: run `st log` with detached HEAD, verify clear error message

## 2. Branch Already Exists — Better Error

- [x] 2.1 In `cmd/st/new.go`, check if branch exists before calling `CreateAndCheckoutBranch`; return "branch 'X' already exists — use 'st attach X' to add it to the stack"
- [x] 2.2 In `cmd/st/append.go`, same check and improved error message
- [x] 2.3 Add E2E tests: `st new` and `st append` with existing branch, verify error suggests `st attach`

## 3. `st continue` Safety Check

- [x] 3.1 In `cmd/st/continue.go`, after verifying rebase is in progress, check for `.git/stack/restack-state.json`; if missing, error: "no st restack in progress — did you mean 'git rebase --continue'?"
- [x] 3.2 Add E2E test: start a manual git rebase, run `st continue`, verify error directs to `git rebase --continue`

## 4. Dirty Working Tree Warning

- [x] 4.1 Add `warnDirtyTree` helper in `cmd/st/main.go` that checks `git status --porcelain` and prints warning via printer
- [x] 4.2 Call `warnDirtyTree` in `restack`, `insert`, and `attach` (relocation path) before rebase operations
- [x] 4.3 Add E2E test: run `st restack` with uncommitted changes, verify warning is printed (and operation still proceeds)

## 5. `st detach` Command

- [x] 5.1 Create `cmd/st/detach.go` with cobra command: accepts 0-1 args, uses current branch if no arg
- [x] 5.2 Implement detach logic: validate branch is in graph, not root; reparent children to detached branch's parent; remove from graph; save
- [x] 5.3 Add output messages: "Detached 'X' from stack", children reparented message, suggest `st restack` if children exist
- [x] 5.4 Register command in `cmd/st/main.go`
- [x] 5.5 Add E2E tests: detach leaf branch, detach branch with children (verify reparenting), detach root (error), detach branch not in graph (error), detach current branch (no arg)

## 6. Improved Conflict Error Messages

- [x] 6.1 Update `cmd/st/restack.go` conflict error to include branch name: "conflict during restack at 'X' — resolve and run 'st continue'"
- [x] 6.2 Verify existing tests still pass with updated error message format
