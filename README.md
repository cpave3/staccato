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

Download the binary for your platform from the newest release
(`st-linux-amd64`, `st-linux-arm64` or `st-darwin-arm64`):

```bash
curl -fsSLo ~/.local/bin/st https://github.com/cpave3/staccato/releases/latest/download/st-linux-amd64
chmod +x ~/.local/bin/st
st version
```

`st update` installs the newest release over it later (`st update --check` only
reports whether there is one), keeping the previous binary as `st.prev`. Each
release lists the binaries' SHA-256 sums in `SHA256SUMS`.

On macOS the binary is not notarized. Downloaded with `curl` it runs as is; if
you downloaded it with a browser, clear the quarantine flag first:
`xattr -d com.apple.quarantine ~/.local/bin/st`.

Or build from source:

```bash
go build -o st ./cmd/st/
ln -sf $(pwd)/st ~/.local/bin/st
```

A build from source reports its version as `dev` and does not update itself.

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

Every command documents itself: `st --help` lists them all with examples, `st <command> --help` gives a command's usage, arguments, options and examples, and `st help` prints a primer on the concepts and main workflows. The table below is a summary.

| Command               | Description                                        | Flags                         |
| --------------------- | -------------------------------------------------- | ----------------------------- |
| `st new <branch>`     | Create a new branch from root/trunk                |                               |
| `st append <branch>`  | Create a child branch from current                 |                               |
| `st insert <branch>`  | Insert a branch before current, restack downstream |                               |
| `st restack`          | Restack stack in topological order                 | `--to-current`                |
| `st continue`         | Resume restack after conflict resolution           |                               |
| `st attach [branch]`  | Interactively adopt/relocate a branch in the stack | `--auto`, `--parent <branch>` |
| `st switch`           | Interactive branch switcher                        |                               |
| `st log`              | Display stack hierarchy                            | `--json`                      |
| `st sync`             | Fetch, detect merged branches, restack & push      | `--dry-run`, `--down`         |
| `st backup`           | Create a manual backup of all stack branches       |                               |
| `st restore [branch]` | Restore from backup                                | `--all`                       |
| `st pr make`          | Create a PR for the current branch                 |                               |
| `st pr view`          | View the PR for the current branch                 |                               |
| `st reviews`          | Collect stack PR review feedback                   | `--current`, `--to-current`, `--out <path>` |
| `st status`           | Show PR status for the entire stack                | `--json`                      |
| `st graph share`      | Share the graph via a pushable/fetchable git ref   |                               |
| `st graph local`      | Move the graph back to local-only storage          |                               |
| `st graph which`      | Show current graph storage mode                    |                               |
| `st graph repair`     | Reconcile current stack with local branch history  | `--dry-run`                   |
| `st split describe [branch]` | Print a branch's commits and changed files as JSON | |
| `st split apply <plan>` | Split a branch into stacked branches from a JSON plan | `--dry-run`, `--exec <cmd>`, `--json` |
| `st exec <command>`   | Run a command on every branch of the current stack and report which pass | `--fail-fast`, `--json` |

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

If a merged branch is still checked out in another worktree, sync removes it from the stack but leaves the local branch in place, prints a warning, and succeeds. Later syncs retry the local deletion until the worktree releases the branch.

Use `--down` to pull and restack without pushing. Use `--dry-run` to see what would be pushed.

A branch the remote refuses to take is named with git's reason after the other branches are pushed, and `st sync` exits 1. `st sync --json` prints the result as JSON, with refused pushes under `failed_pushes`.

### `st restack`

Rebases all branches in your lineage onto their parents. Use `--to-current` to restack only up to the current branch (useful when you're not at the tip of the stack).

### `st graph repair`

After restoring or manually moving branch refs, run `st graph repair --dry-run` to inspect proposed base/head corrections, then `st graph repair` to apply them. Only the checked-out branch's top-level stack and its descendants are inspected; sibling stacks are unchanged. Check out a tracked branch first, and finish or abort any in-progress rebase. If the original fork point cannot be identified, repair refuses to guess. Restack also stops before rewriting branches when a recorded base is not in the branch's history.

### `st split`

Splits one branch of a stack into several smaller stacked branches for review. It is built for an AI agent to drive: the agent decides where the boundaries go, and staccato does the mechanics.

```bash
st split describe            # JSON: commits, changed files, which commits touch which files
st split apply plan.json --dry-run
st split apply plan.json --exec "task build"
```

A plan lists the slices from the bottom of the stack to the top:

```json
{
  "branch": "feature-big",
  "head": "<head from st split describe>",
  "slices": [
    { "name": "feature-big-core", "message": "feat: order model", "paths": ["pkg/core/"] },
    { "name": "feature-big-api", "commits": ["1a2b3c4d5e6f"] },
    { "name": "feature-big", "message": "feat: order list page", "rest": true }
  ]
}
```

Each slice takes its content from one source:

- **`paths`**: pathspecs relative to the repository root. Every changed file they match is brought to its final content in one new commit.
- **`commits`**: existing commits of the branch, kept with their message and author. Cutting at existing commit boundaries rewrites nothing.
- **`rest`**: everything the earlier slices left out. Last slice only.

A `paths` slice can also carry `"overrides": {"<path>": "<content file>"}` for a file whose changes belong to two slices: it gives the file content that is only partway to final, and a later slice lists the path to finish it.

What `apply` guarantees:

- **Nothing is lost.** The top slice must hold exactly the content the branch had before. A plan that leaves anything out is refused, with the missing files listed.
- **Nothing is half-done.** Every slice is built with git plumbing before any branch moves, and all branches move in one ref transaction. A plan that cannot be carried out changes nothing. The working tree is not touched, so uncommitted changes are fine.
- **Descendants follow.** Branches stacked on top are re-parented onto the top slice without a checkout and without conflicts. A descendant that was already behind its parent is left alone and reported.

`--dry-run` builds the slices and reports their sizes and commit ids without moving anything. `--exec <cmd>` checks out each slice in turn and runs the command, like `git rebase --exec`, and applies the split only if it succeeds in every slice.

Name the top slice after the original branch so its pull request and child branches stay attached; to delete the original name instead, set `"drop_original": true`. When a split rewrites commits, the originals are kept on a `backup/manual/...` branch.

### `st exec`

Runs a command on every branch of the current lineage, from the bottom of the stack to the top, and reports which branches pass. Use it to find out which branches of a stack build or pass their tests.

```bash
st exec "task build"                      # run on every branch, then report
st exec --fail-fast go test ./...         # stop at the first branch that fails
st exec --json "task build" 2>/dev/null   # only the report, as JSON
```

```
Ran on 3 branch(es): task build
  ✔ feature-core  passed  1.2s
  ✘ feature-api   failed  0.8s  exit 1
  ✔ feature-ui    passed  1.4s
2 passed, 1 failed
```

Each branch's tip is checked out in turn as a detached HEAD, like `git rebase --exec`, and your branch is checked out again when the run ends, also after a failure or Ctrl-C. This needs a working tree without uncommitted changes. No branch is moved: the command is for checking branches, not changing them.

- The command's own output goes to standard error and the report to standard output. The exit status is 1 if the command failed on any branch.
- By default the command runs on every branch even after a failure. `--fail-fast` stops at the first one and reports the rest as skipped.
- A single argument is a shell command (`sh -c`); several arguments are the words of one command, so `st exec go test -v ./...` needs no quoting. Flags of `st exec` go before the command.
- The command sees `ST_EXEC_BRANCH`, `ST_EXEC_PARENT`, `ST_EXEC_SHA`, `ST_EXEC_INDEX` and `ST_EXEC_TOTAL`.
- Untracked and ignored files (build output, dependency directories) stay in place between branches. Tracked files the command modifies are saved with `git stash`.

`st exec --help` has the full reference, including the JSON format.

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

Hooks have a 30-second timeout.

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

### Moving a stack to another clone

`st graph export` writes the current stack's graph (or `--all`) as a JSON document, after checking each branch is pushed. `st graph import <file>` in another clone fetches those branches and merges the document into that clone's graph branch by branch: a missing branch is created, one that is behind is fast-forwarded, and one with local commits the document lacks is left alone and reported. `--dry-run` shows what it would do. See `st graph import --help` for every outcome.

```bash
st sync && st graph export --output stack.json    # where the stack was built
st graph import stack.json                        # where you carry on
```

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

## Using st from an AI agent

`st` is driven from a shell, and its help is written to be enough for an agent that has never used it:

```bash
st help                 # primer: concepts, the main workflows, every command
st --help               # every command, with examples
st <command> --help     # usage, arguments, options and examples for one command
```

- `st log --json` and `st status --json` print the stack as a JSON tree; `st split describe`, `st split apply --json`, `st exec --json`, `st sync --json`, `st graph import --json`, `st version --json` and `st update --json` print JSON too.
- Help goes to standard output and exits 0. A command that fails exits 1 with the reason on standard error, followed by its usage if it was invoked wrongly.
- `st switch`, `st backup clean`, and `st attach` without `--parent` or `--auto` are interactive. The help of each names the non-interactive alternative.
- `st skill install` installs a skill that tells an agent when to reach for `st` and to start from `st help`.

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
