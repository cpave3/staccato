## Why

The core stacking engine is solid, but the DX layer is thin. Error messages rarely suggest next steps, edge cases return raw git errors, and there are missing escape hatches. Users discover constraints through trial-and-error rather than helpful guidance. This change addresses the highest-impact DX gaps to make `st` feel polished and trustworthy for daily use.

## What Changes

- Add `st detach <branch>` command to remove a branch from the stack graph (reparents children, keeps git branch)
- Add detached HEAD guard across all commands — early, clear error instead of cryptic failures
- Improve `st new`/`st append` error when branch already exists — suggest `st attach` instead of showing raw git error
- Make `st continue` verify that an `st` restack is in progress (check for restack-state.json), not just any git rebase
- Add dirty working tree warning before rebase-triggering commands (`restack`, `insert`, `attach --parent`)
- Improve error messages to follow consistent pattern: what failed → why → what to do next
- Improve conflict error messages to include the conflicting branch name clearly

## Capabilities

### New Capabilities
- `branch-detach`: Command to remove a branch from the stack graph while keeping it in git, with child reparenting
- `input-guards`: Detached HEAD check and dirty working tree warnings applied consistently across commands

### Modified Capabilities
- `branch-creation`: Improved error message when branch already exists (suggest `st attach`)
- `restack`: `st continue` validates restack-state.json exists; improved conflict error messages with branch name

## Impact

- **Code**: New `cmd/st/detach.go`, modifications to `cmd/st/main.go` (HEAD guard), `cmd/st/new.go`, `cmd/st/append.go`, `cmd/st/continue.go`, `cmd/st/restack.go`, `cmd/st/insert.go`, `cmd/st/attach.go`
- **Tests**: New tests for detach, HEAD guard, improved error messages, dirty tree warnings
- **Dependencies**: None
- **Risk**: Low — additive changes (new command, guards) plus error message improvements. No changes to core restack/backup engine.
