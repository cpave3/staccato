---
name: staccato
description: |
  Manage git stacks using Staccato (st) — create stacked branches, restack after changes, sync with remote, and create stacked PRs. Use when working in a repo with stacked branches, when the user mentions stacks/stacking, when you need to split work into incremental PRs, when you see `.git/stack/` or `refs/staccato/` in a repo, or when MCP tools prefixed with `st_` are available. Also use when the user says "st", "staccato", "stack", "restack", or "stacked PRs".
---

# Staccato — Git Stack Management

Staccato (`st`) manages dependent branch chains ("stacks") so each PR is small and focused. Branches form a tree rooted at trunk (main/master), and restacking rebases each branch onto its parent automatically.

## Two Interfaces

### 1. MCP Tools (preferred when available)

When the staccato MCP server is connected, use these tools directly:

**Read-only (structured JSON output):**
- `st_log` — stack tree structure
- `st_status` — stack tree with PR status (number, state, checks, reviews)
- `st_reviews` — stack PR review feedback; filters out non-allowlisted `[bot]` authors
- `st_current` — current branch name, parent, whether in stack
- `st_git_log` — git log (params: `range`, `limit`, `stat`)
- `st_git_diff` — diff output (params: `staged`, `paths`)
- `st_git_diff_stat` — diff stat against a ref
- `st_git_status` — working tree status (porcelain)

**Branch creation:**
- `st_new` — create branch from trunk (`branch_name`)
- `st_append` — create child of current branch (`branch_name`)
- `st_insert` — insert before current, reparent downstream (`branch_name`)

**Splitting a branch:**
- `st_split_describe` — inventory of a branch as JSON: commits, changed files, which commits touch which files (`branch`)
- `st_split_apply` — split a branch into stacked branches from a plan (`plan`: object, `dry_run`: bool)

**Stack operations:**
- `st_restack` — rebase lineage onto parents (`to_current`: bool)
- `st_continue` — resume after conflict resolution
- `st_attach` — add existing branch to stack (`branch_name`, `parent`)
- `st_sync` — fetch + detect merges + restack + push (`dry_run`, `down_only`)
- `st_pr` — push and get PR creation info (`stack`: push whole lineage)

**Git operations:**
- `st_git_add` — stage files (`paths`: string array)
- `st_git_commit` — commit (`message`)
- `st_git_checkout` — checkout branch (`branch`)
- `st_git_cherry_pick` — cherry-pick commits (`commits`: string array)
- `st_git_reset` — reset HEAD (`mode`: soft/mixed/hard, `ref`)

**Escape hatch:**
- `st_run` — run any `st` subcommand as a string (e.g. `st_run(command: "up")`, `st_run(command: "delete feature-x --force")`)

### 2. CLI (when MCP is not available)

Run `st` commands via shell:
```
st new <name>          st append <name>       st insert <name>
st up / down / top / bottom                   st switch (TUI)
st restack [--to-current]                     st continue / st abort
st modify [--all] [-m "msg"]                  st move --onto <parent>
st attach <branch> --parent <parent>          st detach <branch>
st delete <branch> [-f]                       st log / st status
st sync [--dry-run] [--down]                  st pr make [--web]
st backup                                     st restore [branch] [--all]
st reviews [--current] [--to-current] [--out path]
st graph share / local / which
st split describe [branch]
st split apply <plan.json | -> [--dry-run] [--exec "cmd"] [--json]
st exec [--fail-fast] [--json] "<command>"
```

## Workflows

### Start a new stack
```
st new feature/api           # branch from trunk
# ... write code, commit ...
st append feature/frontend   # stack on top of current
# ... write code, commit ...
st append feature/tests      # keep stacking
```

### Add commits to a mid-stack branch
```
st down                      # or: st_git_checkout / st_run(command: "down")
# ... make changes, commit ...
st restack                   # rebase everything downstream
```

### Amend a mid-stack branch (modify)
```
# make changes on the target branch
st modify --all              # amends HEAD, auto-restacks downstream
# or with MCP: st_run(command: "modify --all -m 'updated msg'")
```

### Handle restack conflicts
When `st_restack` or `st restack` reports a conflict:
1. Check which files conflict: `st_git_status` or `git status`
2. Edit the conflicting files to resolve
3. Stage resolved files: `st_git_add(paths: ["file.go"])` or `git add file.go`
4. Continue: `st_continue` or `st continue`
5. If stuck: `st_run(command: "abort")` or `st abort` to cancel and restore backups

### Sync and push
```
st_sync                      # fetch, detect merged PRs, restack, push all
st_sync(dry_run: true)       # preview what would happen
st_sync(down_only: true)     # pull only, don't push
```

### Create stacked PRs
```
st_pr(stack: true)           # pushes all branches in lineage, returns base/head pairs
# Then create PRs using gh or the forge — each PR targets its parent branch as base
```

### Collect review feedback
```
st_reviews                   # MCP: review feedback for the current stack scope
st reviews --current         # CLI: current branch's PR only
st reviews --to-current      # CLI: ancestors through current branch
```

Review collection filters bot accounts by allowlist. Authors ending in `[bot]` are excluded unless they are `coderabbitai[bot]`, `cubic-dev-ai[bot]`, or `greptile-apps[bot]`.

### Split a large branch into a stack

Use this when one branch is too big to review. You choose the boundaries; staccato builds the slices and moves the branches in one step. Do not reset, stage, or cherry-pick by hand.

1. **Describe.** `st_split_describe(branch: "feature-big")` or `st split describe`. Keep `head` from the output. Cut at `commits` only when each commit belongs wholly to one slice; split by `paths` when a commit touches files that belong in different slices, or when `churn` is much larger than `totals` (the commits rework each other).
2. **Plan.** Write JSON listing the slices from the bottom of the stack to the top:
   ```json
   {
     "branch": "feature-big",
     "head": "<head from describe>",
     "slices": [
       {"name": "feature-big-core", "message": "feat: order model", "paths": ["pkg/core/"]},
       {"name": "feature-big-api", "message": "feat: orders over HTTP", "paths": ["pkg/api/"]},
       {"name": "feature-big", "message": "feat: order list page", "rest": true}
     ]
   }
   ```
   Each slice uses one of: `paths` (pathspecs from the repository root; matched files get their final content), `commits` (existing commits, kept as they are), or `"rest": true` (everything left, last slice only). `paths` and `rest` slices need a `message`.
3. **Dry run.** `st_split_apply(plan: {...}, dry_run: true)` or `st split apply plan.json --dry-run`. Nothing moves; you get each slice's size and a commit id to inspect.
4. **Verify and apply.** From a shell, `st split apply plan.json --exec "<build command>"` checks out each slice, runs the command, and applies only if it passes everywhere. Without a build command: `st_split_apply(plan: {...})`.

Things to know:

- **The top slice must match the original branch exactly.** A plan that leaves a file out is refused and nothing changes. Ending with a `"rest": true` slice guarantees completeness.
- **Name the top slice after the original branch** so its PR and child branches stay attached. To delete the original name instead, add `"drop_original": true`.
- **Each slice must build on its own.** Keep a changed signature with its call sites, and a new type with its first use.
- **A file whose changes belong to two slices:** give the earlier slice `"overrides": {"README.md": "/path/to/intermediate-content"}`, and list `README.md` in a later slice's `paths` to finish it.
- **Child branches move onto the top slice automatically**, with no checkout and no conflicts. Children that were already behind their parent are reported under `needs_restack`.
- **`backup` in the result** names a branch holding the original commits. Leave it and tell the user.
- `--exec` is CLI only; it is not available through `st_split_apply` or `st_run`.

The `split-monolithic-pr` MCP prompt has the full guide, including how to choose boundaries.

### Check which branches of a stack build

`st exec` runs a command on every branch of the current lineage, bottom to top, and reports which pass. Run it from a shell: it is not available through `st_run`.

```
st exec "<build command>"                     # run on every branch, then report
st exec --fail-fast "<build command>"         # stop at the first branch that fails
st exec --json "<build command>" 2>/dev/null  # only the report, as JSON
```

- The report is on standard output, one line per branch (`passed`, `failed`, `skipped`); the command's own output is on standard error. Exit status 1 means the command failed on at least one branch.
- It needs a working tree without uncommitted changes. Each branch is checked out in turn as a detached HEAD, and the starting branch is checked out again at the end.
- It checks branches and does not change them: a commit the command makes is not added to the branch. Fix a failing branch with `st_git_checkout`, a commit, then `st restack`.
- The command sees `ST_EXEC_BRANCH`, `ST_EXEC_PARENT`, `ST_EXEC_SHA`, `ST_EXEC_INDEX`, `ST_EXEC_TOTAL`. `st exec --help` has the full reference.

### Adopt existing branches into a stack
```
st_attach(branch_name: "old-feature", parent: "main")
# or: st attach old-feature --parent main
```

### Navigate the stack
```
st_run(command: "up")        # go to child
st_run(command: "down")      # go to parent
st_run(command: "top")       # go to tip
st_run(command: "bottom")    # go to first branch above root
```

## Important Behaviors

- **Commit before restacking** — staccato warns on dirty trees; stash or commit first
- **Conflicts pause the restack** — resolve, `git add`, then `st continue`. The rest of the lineage restacks automatically after continuing.
- **Automatic backups** — restack/insert create backups; `st restore` recovers if things go wrong
- **Graph storage** — defaults to `.git/stack/graph.json` (local). Use `st graph share` to store in a git ref for team collaboration.
- **Trunk detection** — main, master, develop, trunk are auto-recognized as root branches
- **st_run** is the escape hatch — any CLI subcommand works through it. Use specific MCP tools when structured JSON output is useful (log, status, current).
