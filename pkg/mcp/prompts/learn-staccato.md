# Staccato (st) — Git Stack Management

## What is Stacking?

Stacking is a workflow where you build a sequence of dependent pull requests (PRs), each based on the one before it. Instead of one massive PR, you create small, focused changes that reviewers can understand quickly.

```
main ← feature/api ← feature/frontend ← feature/docs
```

Each branch in the stack builds on its parent. When a parent changes (e.g., after code review), all children are automatically rebased.

## Core Concepts

- **Stack**: A sequence of branches, each building on its parent
- **Root/Trunk**: The base branch (e.g., `main`) that stacks are built on
- **Upstream/Downstack**: Branches below the current one (toward root)
- **Downstream/Upstack**: Branches above the current one (away from root)
- **Restack**: Rebase each branch onto its parent to incorporate changes
- **Graph**: Staccato's metadata tracking branch relationships

## Commands

### Creating Branches
| Command | Description |
|---------|-------------|
| `st new <name>` | Create a new branch from root/trunk |
| `st append <name>` | Create a child of the current branch |
| `st insert <name>` | Insert a branch before current, reparent downstream |

### Navigation
| Command | Description |
|---------|-------------|
| `st up` | Check out the child branch (one level up the stack) |
| `st down` | Check out the parent branch (one level down) |
| `st top` | Jump to the tip of the current lineage |
| `st bottom` | Jump to the first branch above root |
| `st switch` | Interactive branch selector (TUI) |

### Stack Operations
| Command | Description |
|---------|-------------|
| `st restack` | Rebase all branches in the lineage onto their parents |
| `st continue` | Resume restack after resolving a conflict |
| `st abort` | Cancel an in-progress rebase and clear state |
| `st modify` | Amend current commit and auto-restack downstream |
| `st move --onto <target>` | Reparent current branch onto a different parent |

### Branch Management
| Command | Description |
|---------|-------------|
| `st attach <branch> --parent <parent>` | Add an existing branch to the stack |
| `st delete <branch>` | Remove a branch, reparent its children |
| `st log` | Display the stack tree |
| `st status` | Display stack tree with PR status |

### Splitting a Branch
| Command | Description |
|---------|-------------|
| `st split describe [branch]` | Print a branch's commits, changed files and churn as JSON |
| `st split apply <plan.json>` | Split a branch into stacked branches from a JSON plan (`--dry-run`, `--exec "<cmd>"`, `--json`) |

### Sync & PRs
| Command | Description |
|---------|-------------|
| `st sync` | Fetch, detect merges, restack, push |
| `st pr` | Push and return PR creation info |
| `st backup` | Create manual backup of all stack branches |
| `st restore` | Restore from automatic backup |

### Graph Storage
| Command | Description |
|---------|-------------|
| `st graph share` | Move graph to shared git ref (for team use) |
| `st graph local` | Move graph back to local file |
| `st graph which` | Show current storage mode |

## Common Workflows

### Start a new stack
```bash
st new feature/api        # Create from trunk
# ... write code ...
git add . && git commit -m "Add API endpoints"
st append feature/frontend  # Stack on top
# ... write code ...
git add . && git commit -m "Add frontend"
```

### Modify a mid-stack branch
```bash
st down                    # Navigate to the branch to modify
# ... make changes ...
st modify --all            # Amend and auto-restack downstream
```

### Split a large branch into a stack
You decide where the boundaries go. Staccato builds the slices and moves the branches in one step, so do not reset, stage, or cherry-pick by hand.

```bash
st split describe                                  # JSON inventory; keep "head"
# write plan.json (below)
st split apply plan.json --dry-run                 # check the plan; nothing moves
st split apply plan.json --exec "<build command>"  # apply only if every slice builds
```

A plan lists the slices from the bottom of the stack to the top:

```json
{
  "branch": "feature-big",
  "head": "<head from st split describe>",
  "slices": [
    {"name": "feature-big-core", "message": "feat: order model", "paths": ["pkg/core/"]},
    {"name": "feature-big-api", "commits": ["1a2b3c4d5e6f"]},
    {"name": "feature-big", "message": "feat: order list page", "rest": true}
  ]
}
```

- Each slice takes its content from exactly one of `paths`, `commits`, or `"rest": true`.
- `paths` are pathspecs relative to the repository root. Every changed file they match gets its **final** content in one new commit. Needs a `message`.
- `commits` are existing commits of the branch, kept with their message and author. No `message`.
- `"rest": true` takes everything the earlier slices left out. Last slice only. Needs a `message`.
- The top slice must hold exactly what the branch held before. A plan that leaves a file out is refused and nothing changes; ending with `rest` guarantees completeness.
- Name the top slice after the original branch so its PR and child branches stay attached. To delete the original name instead, add `"drop_original": true`.
- Cut at `commits` only when each commit belongs wholly to one slice. Split by `paths` when a commit touches files that belong in different slices, or when `churn` in the `describe` output is much larger than `totals` (the commits rework each other).
- The working tree is not touched, so uncommitted changes are fine. There is no undo command: when a split rewrites commits, the originals are kept on the `backup/manual/...` branch named in the output.
- For a file whose changes belong to two slices, give the earlier slice `"overrides": {"README.md": "/path/to/intermediate-content"}` and list `README.md` in a later slice's `paths`.
- Child branches are moved onto the top slice automatically. Any listed under `needs_restack` in the result were already behind: run `st restack` on them.

Use the `split-monolithic-pr` prompt for the full guide, including how to choose the boundaries.

### Handle conflicts during restack
```bash
st restack                 # May hit conflicts
# ... resolve conflicts in editor ...
git add <resolved-files>
st continue                # Resume restacking
# OR
st abort                   # Cancel and restore originals
```

### Sync with remote
```bash
st sync                    # Fetch + detect merges + restack + push
st sync --down-only        # Only pull, don't push
```

## MCP Tools

When using Staccato via MCP, prefer these structured tools for queries:
- `st_log` — returns JSON tree structure
- `st_status` — returns JSON tree with PR status
- `st_current` — returns JSON with current branch info

To split a branch, use the dedicated tools:
- `st_split_describe` — JSON inventory of a branch (`branch`)
- `st_split_apply` — apply a split plan (`plan`: the plan object, which must name its `branch`; `dry_run`)

For all other operations, use `st_run` with any command string:
- `st_run("up")`, `st_run("modify --all")`, `st_run("delete feature-x")`

`--exec` is not available over MCP, through either `st_split_apply` or `st_run`. Run `st split apply --exec` from a shell.

## Key Principles

1. **Offline-first**: All operations work without network access (except sync/push)
2. **Deterministic**: Restacking always produces the same result for the same inputs
3. **Automatic backups**: Destructive operations create backups that can be restored
4. **Lazy attachment**: Existing branches can be retrofitted into a stack at any time
