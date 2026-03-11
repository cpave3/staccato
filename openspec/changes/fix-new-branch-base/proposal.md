## Why

`st new <branch>` creates the new branch from the current HEAD instead of from the root branch. This means if you're on branch `foo` and run `st new bar`, `bar` inherits all of `foo`'s commits. The graph correctly shows `bar` parented to the root, but the actual git branch points to the wrong commit. This violates the spec and produces confusing behavior.

## What Changes

- Fix `st new` to checkout the root branch before creating the new branch, ensuring the new branch starts from the root's HEAD — not from whatever branch is currently checked out.

## Capabilities

### New Capabilities

(none)

### Modified Capabilities

- `branch-creation`: The `st new` command must create the branch from the root branch's commit, not from the current HEAD.

## Impact

- `cmd/st/new.go`: Change branch creation to start from root
- Possibly `pkg/git/git.go`: May need a `CreateAndCheckoutBranchFrom(name, startPoint)` variant
- Tests in `cmd/st/main_test.go`: Add/update test covering this scenario
