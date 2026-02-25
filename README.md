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

# Push to remote
st sync
```

## Commands

| Command               | Description                                                    |
| --------------------- | -------------------------------------------------------------- |
| `st new <branch>`     | Create a new branch from root/main                             |
| `st append <branch>`  | Create a child branch from current                             |
| `st insert <branch>`  | Insert a branch before current, restack downstream             |
| `st restack`          | Restack entire stack in topological order                      |
| `st continue`         | Resume restack after conflict resolution                       |
| `st attach [--auto]`  | Adopt an existing branch into the stack (opens TUI by default) |
| `st switch`           | Interactive branch switcher with vim-like navigation           |
| `st restore [branch]` | Restore from backup                                            |
| `st sync [--dry-run]` | Push branches to remote                                        |
| `st log`              | Display stack hierarchy                                        |

### TUI Commands

The `st attach` and `st switch` commands launch interactive TUIs with vim-like navigation:

- **Arrow keys**: Navigate up/down
- **/**: Enter search mode
- **Enter**: Select item (or exit search and jump to first match)
- **n/N**: Navigate to next/previous match (after exiting search)
- **q/Esc**: Quit

**Note:** TUI commands require an interactive terminal (TTY) and won't work in non-interactive environments (CI/CD, scripts). Use `st attach --auto` for non-interactive usage.

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
- **Recursive attachment**: After selecting m3's parent (m2), it prompts for m2's parent, and so on until reaching root
- Never rewrites history during attachment
- Interactive TUI mode for visual parent selection

### Interactive TUIs

Both `st switch` and `st attach-tui` provide vim-like navigation:

- **Arrow keys**: Navigate up/down
- **/**: Enter search mode
- **Enter**: Exit search mode and jump to first match
- **n/N**: Navigate to next/previous match (after exiting search)
- **Enter**: Select current item
- **q/Esc**: Quit

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
# Run tests
go test ./...

# Run specific package tests
go test ./pkg/graph/...
go test ./pkg/restack/...

# Build
go build -o st ./cmd/st/
```

## Philosophy

- **Offline-first**: No automatic network operations
- **Explicit is better than implicit**: Push is manual
- **Safety over speed**: Always backup before destructive operations
- **Branch-level thinking**: Users think in features, not commits

## License

MIT License - See LICENSE file for details
