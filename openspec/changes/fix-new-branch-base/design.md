## Context

`st new` currently calls `git.CreateAndCheckoutBranch(branchName)` which runs `git checkout -b <name>` — creating the branch from the current HEAD. If the user is on a feature branch, the new branch inherits that branch's commits instead of starting clean from the root.

## Goals / Non-Goals

**Goals:**
- `st new <branch>` always creates the new branch from the root branch's HEAD, regardless of which branch is currently checked out.

**Non-Goals:**
- Changing `append` or `insert` behavior (they correctly branch from current/parent).
- Adding a `--from` flag or other options to `st new`.

## Decisions

**Decision: Add a start-point parameter to `CreateAndCheckoutBranch`**

`git checkout -b <name> <start-point>` already supports specifying a start point. Add a `CreateAndCheckoutBranchFrom(name, startPoint string)` method to `pkg/git/git.go`, then use it in `new.go` with `g.Root` as the start point.

Alternative considered: Checkout root first, then create branch. This works but changes the user's checkout state as a side effect if branch creation fails, and is two git operations instead of one.

## Risks / Trade-offs

- [Minimal risk] This is a small, isolated change. The only risk is if `g.Root` doesn't resolve to a valid ref, but `getContext()` already validates the graph.
