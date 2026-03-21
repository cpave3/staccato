## 1. Hook Engine (`pkg/hooks/`)

- [x] 1.1 Create `pkg/hooks/hooks.go` with `Event` type, all event constants, `Context` struct, and `Runner` type
- [x] 1.2 Implement directory discovery: scan `<hook-dir>/<event>/` for executable files, skip non-executable
- [x] 1.3 Implement two-level resolution: resolve global (`~/.config/staccato/hooks/`) and project (`.staccato/hooks/`) directories, collect hooks from both (global first)
- [x] 1.4 Implement hook execution: run each script with JSON on stdin, set `ST_EVENT`, `ST_REPO_PATH`, `ST_BRANCH` env vars
- [x] 1.5 Implement exit code handling: 0 = success, 2 = block (pre-hooks) or warn (post-hooks), other = warn and continue
- [x] 1.6 Implement 30-second timeout with process kill on expiry
- [x] 1.7 Write unit tests for discovery, execution, exit code handling, and timeout

## 2. CLI Command Integration

- [x] 2.1 Wire `post-pr-create` hook into `cmd/st/pr.go` after successful PR creation (data: `base`, `web`)
- [x] 2.2 Wire `post-branch-create` hook into `cmd/st/new.go`, `cmd/st/append.go`, and `cmd/st/insert.go` (data: `parent`)
- [x] 2.3 Wire `post-branch-delete` hook into `cmd/st/delete.go` and `cmd/st/delete_stack.go` (one event per branch, data: `parent`)
- [x] 2.4 Wire `pre-restack` and `post-restack` hooks into `cmd/st/restack.go` (pre blocks on exit 2, post data: `restacked_count`)
- [x] 2.5 Wire `post-restack-conflict` hook into `cmd/st/restack.go` on conflict (data: `conflict_branch`)
- [x] 2.6 Wire `pre-sync` and `post-sync` hooks into `cmd/st/sync.go` (pre blocks on exit 2, post data: `merged_branches`, `pushed_branches`, `restacked_count`; skip on `--dry-run`)
- [x] 2.7 Wire `post-attach` hook into `cmd/st/attach.go` (data: `parent`)

## 3. MCP Tool Integration

- [x] 3.1 Wire hooks into `pkg/mcp/tools_sync.go` (`st_pr`, `st_sync`) to fire corresponding events
- [x] 3.2 Wire hooks into `pkg/mcp/tools_branch.go` (`st_new`, `st_append`, `st_insert`) to fire `post-branch-create`
- [x] 3.3 Wire hooks into `pkg/mcp/tools_management.go` (`st_restack`, `st_attach`, `st_delete_stack`) to fire corresponding events
- [x] 3.4 Ensure MCP pre-hook blocks return tool errors (not panics)

## 4. Testing & Validation

- [x] 4.1 Add integration test: create executable hook script in temp dir, run command, verify hook was called with correct JSON
- [x] 4.2 Add integration test: pre-hook exit 2 blocks operation
- [x] 4.3 Add integration test: global + project hooks both execute in correct order
