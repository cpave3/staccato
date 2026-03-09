## Context

The staccato CLI has a solid engine but a thin DX layer. The audit identified ~30 issues ranging from missing commands to cryptic errors. This change targets the highest-impact items — the ones that block users or cause confusion daily.

Current state:
- No way to remove a branch from the stack (`st detach` doesn't exist)
- `st continue` doesn't verify it's continuing an `st` operation vs a manual rebase
- Branch-already-exists errors show raw git output
- No guard against detached HEAD or dirty working tree
- Error messages inconsistent — some suggest next steps, others don't

## Goals / Non-Goals

**Goals:**
- Add `st detach` command for removing branches from the stack graph
- Add detached HEAD guard as early check in command execution
- Add dirty working tree warning before rebase-triggering operations
- Make `st continue` verify restack-state.json exists
- Improve branch-already-exists error to suggest `st attach`
- Consistent error message pattern across commands

**Non-Goals:**
- Redesigning the TUI (attach/switch) — separate effort
- Adding `st branches` flat list view — nice-to-have for later
- Confirmation prompts for multi-branch ops — needs design discussion
- Fixing verbose flag inconsistencies — separate cleanup
- Corrupt graph recovery — separate robustness effort

## Decisions

### `st detach` reparents children to the detached branch's parent
When removing a branch from the graph, its children need somewhere to go. Reparenting to the detached branch's parent is the simplest and most predictable behavior. The reparented children may need restacking, but we won't auto-restack — the user should run `st restack` explicitly if needed, keeping `detach` fast and non-destructive.

**Alternative considered**: Delete children from graph too. Rejected — too destructive, and the user may want to keep them.

### Detached HEAD guard lives in `getContext` wrapper
Rather than adding the check to every command individually, add a helper that commands call after `getContext`. This centralizes the check. Commands that don't need a branch (like `graph which`) skip it.

**Alternative considered**: Cobra PersistentPreRun hook. Rejected — some commands like `graph which` don't need a current branch.

### Dirty working tree check is a warning, not an error
Blocking on dirty tree would be too aggressive — users often have local changes they don't want to commit yet. A warning lets them proceed but makes the risk visible. Only rebase-triggering commands warn.

**Alternative considered**: Hard error requiring stash/commit. Rejected — too disruptive to workflow.

### `st continue` checks restack-state.json before proceeding
If no restack state file exists AND a rebase is in progress, the user is likely in a manual rebase. We error with: "no st restack in progress — did you mean 'git rebase --continue'?" If the state file exists, proceed as before.

**Alternative considered**: Always continue any rebase. Rejected — silently adopting a manual rebase into the graph is dangerous.

### Error messages follow "what → why → what to do" pattern
Example: `"branch 'feature-1' already exists — use 'st attach feature-1' to add it to the stack"`. Not all errors need all three parts, but the "what to do" part is the key addition.

## Risks / Trade-offs

- **[Risk] `st detach` without auto-restack may leave stale BaseSHAs on children** → Acceptable — user runs `st restack` to fix, and we print a message suggesting it.
- **[Risk] Dirty tree warning might be noisy** → Only shown before rebase operations, and only once per command. Users who don't have dirty trees never see it.
- **[Risk] `st continue` rejecting non-st rebases is a behavior change** → Low risk — the old behavior (continuing any rebase) was a bug. Users who hit this will get a clear error pointing them to `git rebase --continue`.
