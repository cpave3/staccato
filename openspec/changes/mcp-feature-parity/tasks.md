## 1. Prerequisites & Stack Navigation Commands

- [x] 1.1 Add `RebaseAbort()` method to `pkg/git/Runner` (already exists)
- [x] 1.2 CommitAmend handled inline via `gitRunner.Run("commit", "--amend", ...)`
- [x] 1.3 Add `DeleteBranch(name string)` method to `pkg/git/Runner` (already exists)
- [x] 1.4 Implement `st up` command in `cmd/st/up.go` — check out single child, error on multiple/none
- [x] 1.5 Implement `st down` command in `cmd/st/down.go` — check out parent, error at root
- [x] 1.6 Implement `st top` command in `cmd/st/top.go` — follow single-child links to tip
- [x] 1.7 Implement `st bottom` command in `cmd/st/bottom.go` — follow parent links to first child of root
- [x] 1.8 Register `up`, `down`, `top`, `bottom` commands in `cmd/st/main.go`

## 2. Branch Modify Command

- [x] 2.1 Implement `st modify` command in `cmd/st/modify.go` — amend HEAD + restack downstream
- [x] 2.2 Support `--all` flag to stage all changes before amending
- [x] 2.3 Support `--message` flag to update commit message
- [x] 2.4 Create automatic backups before restacking, clean up on success
- [x] 2.5 Register `modify` command in `cmd/st/main.go`
- [x] 2.6 Write E2E tests for modify (amend, restack, conflict, no-changes error)

## 3. Branch Delete Command

- [x] 3.1 `RemoveBranch` and `ReparentChildren` already exist in `pkg/graph/` (already exists)
- [x] 3.2 Implement `st delete` command in `cmd/st/delete.go` — remove from graph, delete git branch, reparent children
- [x] 3.3 Handle current-branch deletion (checkout parent first)
- [x] 3.4 Add `--force` flag for branches with unpushed commits
- [x] 3.5 Register `delete` command in `cmd/st/main.go`
- [x] 3.6 Write E2E tests for delete (no children, with children, current branch, root error, force flag)

## 4. Branch Move Command

- [x] 4.1 Implement `st move` command in `cmd/st/move.go` — reparent + restack
- [x] 4.2 Add cycle detection (cannot move onto self or descendant)
- [x] 4.3 Register `move` command in `cmd/st/main.go`
- [x] 4.4 Write E2E tests for move (reparent, restack, cycle errors, not-in-stack errors)

## 5. Abort Command

- [x] 5.1 Implement `st abort` command in `cmd/st/abort.go` — git rebase --abort, clear restack state, restore backups
- [x] 5.2 Register `abort` command in `cmd/st/main.go`
- [x] 5.3 Write E2E tests for abort (active rebase, no rebase error, state cleanup)

## 6. MCP Universal Tool (st_run)

- [x] 6.1 Implement `st_run` tool in `pkg/mcp/tools_run.go` — subprocess execution via `os.Executable()`
- [x] 6.2 Block recursive `st mcp` invocation
- [x] 6.3 Register `st_run` tool in `pkg/mcp/server.go`
- [ ] 6.4 Write unit tests for st_run (success, failure, recursive block, empty command)

## 7. MCP Learn Prompt

- [x] 7.1 Write `pkg/mcp/prompts/learn-staccato.md` — comprehensive stacking guide with command reference
- [x] 7.2 Register `learn-staccato` as MCP prompt and resource in `pkg/mcp/prompts.go`
- [x] 7.3 Verify prompt renders correctly via MCP protocol (compiles and embeds successfully)

## 8. Enhanced st_pr Tool

- [x] 8.1 Add `stack` boolean parameter to `st_pr` tool in `pkg/mcp/tools_sync.go`
- [x] 8.2 Implement stack-wide PR info — iterate lineage, push each, return array of PR info objects
- [ ] 8.3 Write tests for stack-wide PR info
