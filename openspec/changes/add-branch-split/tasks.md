## 1. Git plumbing

- [x] 1.1 Add `Runner.Output(env, stdin, args...)` returning untrimmed stdout with stderr reported only on failure, and `Runner.RunWith` as its trimmed form
- [x] 1.2 Add `Runner.CherryPickTree(onto, commit)` returning a `Pick{Tree, Conflicted, Paths}` using `git merge-tree --write-tree`
- [x] 1.3 Unit tests for both in `pkg/git/git_test.go`

## 2. Describe

- [x] 2.1 Create `pkg/split` with `Describe(sc, branch)` returning commits, files, totals, churn, rename links, children and worktree
- [x] 2.2 Add `st split describe [branch]` printing the description as JSON
- [x] 2.3 E2E tests for commits, files, totals, churn, renames, named branch and untracked branch

## 3. Plan

- [x] 3.1 `Plan` and `Slice` types with strict JSON decoding (unknown fields rejected, line and column on syntax errors)
- [x] 3.2 `Plan.Validate()` for shape: slice count, names, exactly one content source, message rules, `rest` last
- [x] 3.3 Table tests in `pkg/split/plan_test.go`

## 4. Building slices

- [x] 4.1 `paths` slices in a scratch index, including deletions and file modes
- [x] 4.2 `rest` slices
- [x] 4.3 `commits` slices: reuse in place, cherry-pick when reordered, refuse on conflict, leave out a commit that adds nothing
- [x] 4.4 `overrides` for regular files
- [x] 4.5 Completeness check with the missing files, unfinished overrides and unassigned commits listed
- [x] 4.6 Errors for an unmatched path, a path that claims nothing, and an empty slice
- [x] 4.7 Sign commits when `commit.gpgsign` is set

## 5. Applying

- [x] 5.1 Pre-flight: head guard, behind parent, rebase in progress, name validity and collisions, nested names, explicit `drop_original`
- [x] 5.2 One `update-ref --stdin` transaction with old values, graph rewrite, inverse transaction if the graph save fails
- [x] 5.3 Re-parent descendants with `commit-tree`; leave out-of-date descendants untouched and report them
- [x] 5.4 HEAD and worktree rules
- [x] 5.5 Manual backup when commits are rewritten
- [x] 5.6 `--dry-run` and `--json`, plan from stdin
- [x] 5.7 Fire `post-branch-create` and `post-branch-delete` hooks

## 6. Verification

- [x] 6.1 `--exec`: run the command in each slice before anything moves; nothing applied on failure
- [x] 6.2 Refuse with uncommitted tracked changes; stash and fail when the command changes tracked files
- [x] 6.3 Restore the starting checkout on failure and on SIGINT/SIGTERM

## 7. MCP

- [x] 7.1 `StaccatoContext.Reload()` so handlers do not act on a stale graph
- [x] 7.2 `st_split_describe` and `st_split_apply` as thin wrappers over `pkg/split`
- [x] 7.3 `st_run` refuses `--exec`
- [x] 7.4 Handler tests in `pkg/mcp/tools_split_test.go`

## 8. Documentation

- [x] 8.1 Rewrite `pkg/mcp/prompts/split-monolithic-pr.md` around the new commands
- [x] 8.2 Add the split workflow to `skill/SKILL.md` and `pkg/mcp/prompts/learn-staccato.md`
- [x] 8.3 Document `st split` in `README.md` and `AGENTS.md`
