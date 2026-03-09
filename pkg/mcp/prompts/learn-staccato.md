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

### Split a large change into a stack
Use the `split-monolithic-pr` prompt for guided help splitting a large PR into focused stacked branches.

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

For all other operations, use `st_run` with any command string:
- `st_run("up")`, `st_run("modify --all")`, `st_run("delete feature-x")`

## Key Principles

1. **Offline-first**: All operations work without network access (except sync/push)
2. **Deterministic**: Restacking always produces the same result for the same inputs
3. **Automatic backups**: Destructive operations create backups that can be restored
4. **Lazy attachment**: Existing branches can be retrofitted into a stack at any time
