# Staccato - Git Stack Management CLI

![Tests](https://github.com/cpave3/staccato/actions/workflows/test.yml/badge.svg)
![Coverage](https://img.shields.io/endpoint?url=https://gist.githubusercontent.com/cpave3/b4ac84df7b53f7fc3281515f3d1add71/raw/staccato-coverage.json)

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

| Command               | Description                                        | Flags                         |
| --------------------- | -------------------------------------------------- | ----------------------------- |
| `st new <branch>`     | Create a new branch from root/trunk                |                               |
| `st append <branch>`  | Create a child branch from current                 |                               |
| `st insert <branch>`  | Insert a branch before current, restack downstream |                               |
| `st restack`          | Restack stack in topological order                 | `--to-current`                |
| `st continue`         | Resume restack after conflict resolution           |                               |
| `st attach [branch]`  | Interactively adopt/relocate a branch in the stack | `--auto`, `--parent <branch>` |
| `st switch`           | Interactive branch switcher                        |                               |
| `st log`              | Display stack hierarchy                            |                               |
| `st sync`             | Fetch, detect merged branches, restack & push      | `--dry-run`, `--down`         |
| `st backup`           | Create a manual backup of all stack branches       |                               |
| `st restore [branch]` | Restore from backup                                | `--all`                       |
| `st pr make`          | Create a PR for the current branch                 |                               |
| `st pr view`          | View the PR for the current branch                 |                               |
| `st reviews`          | Collect stack PR review feedback                   | `--current`, `--to-current`, `--out <path>` |
| `st status`           | Show PR status for the entire stack                |                               |
| `st graph share`      | Share the graph via a pushable/fetchable git ref   |                               |
| `st graph local`      | Move the graph back to local-only storage          |                               |
| `st graph which`      | Show current graph storage mode                    |                               |
| `st mcp`              | Start the MCP server (stdio transport)             |                               |

### `st attach`

Adopts existing branches into the stack. Always launches an interactive TUI, even if the branch is already tracked (allowing you to relocate it).

- **Recursive attachment**: After selecting a parent, if that parent isn't tracked either, it prompts for _its_ parent, and so on up the chain
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

### `st reviews`

Collects inline review comments, review submissions, and general PR comments from GitHub PRs for branches in the current stack scope.

Bot comments are filtered by allowlist: authors whose login ends in `[bot]` are excluded unless the bot is explicitly allowed as a substantive review bot. The current allowlist is `coderabbitai[bot]`, `cubic-dev-ai[bot]`, and `greptile-apps[bot]`.

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
- Metadata persisted locally or shared via git ref

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

## Hooks

Staccato supports a hook system for triggering custom scripts on lifecycle events. Hooks are executable files placed in event-named directories:

```
~/.config/staccato/hooks/<event>/    # Global (all repos)
<repo>/.staccato/hooks/<event>/      # Project (repo-specific, committable)
```

Both directories are scanned when an event fires — global hooks run first, then project hooks.

### Example

```bash
# Get notified when a PR is created
mkdir -p .staccato/hooks/post-pr-create
cat > .staccato/hooks/post-pr-create/notify.sh << 'EOF'
#!/bin/sh
# Hook receives JSON context on stdin with event, branch, data fields
BRANCH=$(echo "$ST_BRANCH" | tr -d '\n')
echo "PR created for $BRANCH"
EOF
chmod +x .staccato/hooks/post-pr-create/notify.sh
```

### Available Events

| Event                  | Fires when                                     |
| ---------------------- | ---------------------------------------------- |
| `post-pr-create`       | After `st pr make` creates a PR                |
| `post-pr-view`         | After `st pr view` displays a PR               |
| `post-branch-create`   | After `st new`, `st append`, `st insert`       |
| `post-branch-delete`   | After `st delete` or `st delete-stack`          |
| `post-restack`         | After `st restack` completes successfully      |
| `post-restack-conflict`| When `st restack` stops on a conflict          |
| `post-sync`            | After `st sync` completes                      |
| `post-attach`          | After `st attach` adds a branch to the stack   |
| `pre-sync`             | Before `st sync` begins (can block)            |
| `pre-restack`          | Before `st restack` begins (can block)         |

### Hook Context

Each hook receives:
- **stdin**: JSON payload with `event`, `repo_path`, `branch`, and event-specific `data`
- **Environment variables**: `ST_EVENT`, `ST_REPO_PATH`, `ST_BRANCH`

### Exit Codes

| Exit Code | Meaning |
| --------- | ------- |
| 0         | Success — continue normally |
| 2         | Block the operation (`pre-*` hooks only) |
| Other     | Warning — print stderr and continue |

Hooks have a 30-second timeout. MCP tools fire the same hooks as their CLI equivalents.

## Configuration

By default, stack metadata is stored locally in `.git/stack/graph.json` (invisible to git, local-only).

### Shared Mode

For teams, the graph can be shared via a git ref (`refs/staccato/graph`):

```bash
st graph share    # Move local graph to a shared git ref
st graph which    # Check current mode: "Shared" or "Local"
st graph local    # Move back to local-only storage
```

In shared mode:

- The graph is stored as a blob at `refs/staccato/graph` — invisible to checkout but pushable/fetchable
- `st sync` automatically pushes the graph ref alongside branches
- A fetch refspec is configured so `git fetch` pulls the graph ref too
- Teammates cloning the repo get the shared graph on their first `st sync`

### Graph Format

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

## MCP Server

Staccato includes an MCP (Model Context Protocol) server, allowing LLMs to programmatically manage stacked branches. This is useful for splitting monolithic PRs into focused stacks.

### Starting the server

```bash
st mcp
```

This runs the MCP server over stdio. You don't typically run this directly — instead, configure your LLM client to launch it.

### Adding to Claude Code

```bash
claude mcp add staccato --scope user -- st mcp
```

### Adding to OpenCode

Add to your `opencode.json`:

```json
{
  "mcpServers": {
    "staccato": {
      "command": "st",
      "args": ["mcp"]
    }
  }
}
```

### Available tools (20)

**Git helpers**: `st_git_log`, `st_git_diff_stat`, `st_git_cherry_pick`, `st_git_checkout`, `st_git_reset`, `st_git_add`, `st_git_commit`, `st_git_status`, `st_git_diff`

**Stack info**: `st_log`, `st_status`, `st_current`

**Branch creation**: `st_new`, `st_append`, `st_insert`

**Management**: `st_attach`, `st_restack`, `st_continue`

**Sync/PR**: `st_sync`, `st_pr`

### Prompt

The server exposes a `split-monolithic-pr` prompt that guides an LLM through analyzing a large PR and splitting it into focused, stacked branches. Accepts `base_branch` and `source_branch` arguments.

## Development

This project uses [Task](https://taskfile.dev) as a task runner. Install it and the other dev tools once:

```bash
go install github.com/go-task/task/v3/cmd/task@latest
task install:tools
```

Common commands:

```bash
task build          # Build the st binary
task test           # Run all tests
task test:verbose   # Run all tests with verbose output
task test:e2e       # Run E2E tests only
task lint           # Run golangci-lint
task check          # Build + test + lint
task install        # Install st to $GOPATH/bin
```

Or run Go commands directly:

```bash
go test ./cmd/st/ -v -count=1 -run TestAttach   # Run a specific test
```

The linter config (`.golangci.yml`) enables `govet`, `staticcheck`, `unused`, and `ineffassign`. The `unused` linter catches dead code (unused functions, fields, variables).

All feature development follows TDD (red-green-refactor). Write failing tests first.

## Philosophy

- **Offline-first**: No automatic network operations
- **Explicit is better than implicit**: Push is manual
- **Safety over speed**: Always backup before destructive operations
- **Branch-level thinking**: Users think in features, not commits

## License

MIT License - See LICENSE file for details
