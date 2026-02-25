# Staccato - Git Stack Management CLI

A deterministic, offline-first Git stack management tool inspired by Graphite and Git Town.

## Overview

`st` provides branch-level stacking with:

- **Deterministic restacking**: Rebase branches in topological order, stopping on first conflict
- **Automatic backups**: Creates backups before any destructive operation
- **Lazy attachment**: Retrofit existing manually-stacked branches
- **Offline-first**: No automatic push, remote sync is explicit

## Installation

```bash
# Build from source
go build -o st ./cmd/st/

# Symlink to your PATH
ln -sf $(pwd)/st ~/.local/bin/st
```

## Quick Start

```bash
# Initialize stack by creating your first feature branch
st new feature-1

# Create child branches
st append feature-2
st append feature-3

# View your stack
st log

# Insert a branch before current
st insert feature-1-5

# Restack everything
st restack

# Fetch, detect merged branches, restack & push
st sync

# Pull only (fetch + restack, no push)
st sync --down
```

## Commands

| Command | Description | Flags |
| --- | --- | --- |
| `st new <branch>` | Create a new branch from root/trunk | |
| `st append <branch>` | Create a child branch from current | |
| `st insert <branch>` | Insert a branch before current, restack downstream | |
| `st restack` | Restack stack in topological order | `--to-current` |
| `st continue` | Resume restack after conflict resolution | |
| `st attach [branch]` | Interactively adopt/relocate a branch in the stack | `--auto`, `--parent <branch>` |
| `st switch` | Interactive branch switcher | |
| `st log` | Display stack hierarchy | |
| `st sync` | Fetch, detect merged branches, restack & push | `--dry-run`, `--down` |
| `st backup` | Create a manual backup of all stack branches | |
| `st restore [branch]` | Restore from backup | `--all` |
| `st pr make` | Create a PR for the current branch | |
| `st pr view` | View the PR for the current branch | |
| `st status` | Show PR status for the entire stack | |

### `st attach`

Adopts existing branches into the stack. Always launches an interactive TUI, even if the branch is already tracked (allowing you to relocate it).

- **Recursive attachment**: After selecting a parent, if that parent isn't tracked either, it prompts for *its* parent, and so on up the chain
- **Trunk auto-detection**: Common trunk names (`main`, `master`, `develop`, `trunk`) are automatically set as root when selected
- **`r` keybinding**: Press `r` in the TUI to manually designate any branch as root
- **`--parent <branch>`**: Skip the TUI and specify the parent directly. Works for both new and already-tracked branches. Trunk names are auto-detected as root.
- **`--auto`**: Automatically select the best parent candidate (skip TUI)

### `st sync`

Performs a full sync cycle:

1. Fetch from remote (with prune)
2. Fast-forward trunk to match remote
3. Detect merged branches (regular merge and squash merge)
4. Remove merged branches from the stack (reparenting children)
5. Restack remaining branches onto updated trunk
6. Push remaining branches (skip with `--down`)

Use `--down` to pull and restack without pushing. Use `--dry-run` to see what would be pushed.

### `st restack`

Rebases all branches in your lineage onto their parents. Use `--to-current` to restack only up to the current branch (useful when you're not at the tip of the stack).

### TUI Navigation

The `st attach` and `st switch` commands launch interactive TUIs:

- **Arrow keys**: Navigate up/down
- **/**: Enter search mode
- **Enter**: Select item
- **q/Esc**: Quit
- **r**: Set selected branch as root (`st attach` only)
- **n/N**: Navigate to next/previous search match (`st switch` only)

TUI commands require an interactive terminal (TTY). Use `st attach --auto` or `st attach --parent` for non-interactive usage.

## Features

### Branch-Level Stacking

- Think in branches, not commits
- Each branch stores parent, base SHA, and head SHA
- Metadata persisted in `.git/stack/graph.json`

### Deterministic Restacking

- Topological sort ensures parents rebased before children
- Branch-level rebasing: conflicts occur once per branch
- Git rerere integration for automatic conflict resolution

### Safety First

- Automatic backups before destructive operations
- Restore from backup if restack fails
- Cycle detection prevents invalid stack structures

### Lazy Attachment (Recursive)

- Retrofit existing manually-created branches
- Shows ALL branches as potential parents (tracked or untracked)
- Recursive: prompts for each untracked parent up the chain until reaching a root
- Trunk branches (`main`, `master`, `develop`, `trunk`) are auto-detected as root
- Press `r` to manually set any branch as root

## Configuration

Stack metadata is stored in `.git/stack/graph.json`:

```json
{
  "version": 1,
  "root": "main",
  "branches": {
    "feature-a": {
      "name": "feature-a",
      "parent": "main",
      "base_sha": "abc123",
      "head_sha": "def456"
    }
  }
}
```

## Development

```bash
# Run all tests
go test ./... -v -count=1

# Run E2E tests only
go test ./cmd/st/ -v -count=1

# Run specific test
go test ./cmd/st/ -v -count=1 -run TestAttach

# Build
go build -o st ./cmd/st/
```

All feature development follows TDD (red-green-refactor). Write failing tests first.

## Philosophy

- **Offline-first**: No automatic network operations
- **Explicit is better than implicit**: Push is manual
- **Safety over speed**: Always backup before destructive operations
- **Branch-level thinking**: Users think in features, not commits

## License

MIT License - See LICENSE file for details
