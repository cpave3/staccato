# AGENTS.md

## Project Overview

**st (Staccato)** is a deterministic, offline-first Git stack management CLI written in Go. It provides branch-level stacking with automatic backups, lazy attachment, and interactive TUI for branch selection.

## Architecture

### Project Structure
```
st/
├── cmd/st/
│   ├── main.go           # All commands + attachTUI model
│   ├── switch.go          # switchTUI model + switch command
│   └── main_test.go       # Comprehensive E2E + TUI model tests
├── internal/testutil/
│   └── git.go             # Test helpers (GitRepo, isolated repos)
├── pkg/
│   ├── graph/             # Stack graph model & JSON persistence
│   ├── git/               # Git operations wrapper (subprocess)
│   ├── backup/            # Backup/restore system (auto + manual)
│   ├── restack/           # Topological sort & restack engine
│   ├── attach/            # Lazy attachment logic
│   └── output/            # CLI formatting utilities
├── go.mod
└── st                     # Compiled binary
```

### Key Components

#### pkg/graph
- `Graph` struct: Root branch + `map[string]*Branch` of tracked branches
- `Branch` struct: `Name`, `Parent`, `BaseSHA`, `HeadSHA`
- JSON persistence to `.git/stack/graph.json`
- `GetChildren(parent)` returns `[]*Branch`
- `ValidateNoCycle()` prevents circular dependencies

#### pkg/git
- `Runner` struct: Wraps git subprocess calls via `exec.Command`
- All output is trimmed via `strings.TrimSpace`
- Key methods: `CreateAndCheckoutBranch`, `Rebase`, `RebaseContinue`, `IsRebaseInProgress`, `GetMergeBase`, `CopyBranch`, `HasRemote`, `Push`
- `GetAllBranches()` uses `--format=%(refname:short)`

#### pkg/backup
- Two backup schemes:
  - **Automatic** (`backup/<branch>/<nanosecond-timestamp>`): Created during restack/insert, auto-cleaned on success
  - **Manual** (`backups/<YYYY-MM-DD_HH-MM-SS>/<branch>`): Created via `st backup`, persists until deleted
- `ListBackups(branch)` looks for `backup/<branch>/` prefix (automatic only)
- `RestoreBackup` deletes original, copies backup, checks out restored branch, deletes backup ref

#### pkg/restack
- `TopologicalSort(g, root)`: Parents before children, cycle detection
- `GetStackBranches(g, start)`: DFS all descendants including start
- `GetDownstreamBranches(g, start)`: Descendants excluding start
- `GetLineage(g, branch)`: Ancestors + descendants chain
- `GetAncestors(g, branch)`: Root → branch (no descendants)
- `IsBranchAtTip(g, branch)`: No children = true
- `Restack()` / `RestackLineage()`: Create backups → enable rerere → topo-sort → rebase each onto parent → stop on conflict
- `Continue()`: Resume rebase → update SHAs → continue restack

#### pkg/attach
- `SuggestParents(g, branch)`: Scores candidates by merge-base recency
- `AutoAttach(g, branch, true)`: Uses first (best) candidate
- `AttachBranch(g, branch, parent)`: Gets merge-base as BaseSHA, HEAD as HeadSHA
- `IsBranchInGraph()` / `FindRoot()`: Graph traversal helpers
- `GetUnattachedBranches()`: Branches not in graph

#### pkg/output
- `Printer` struct with `verbose` flag
- `Info()` only prints when verbose is true — **tests need `-v` flag** for verbose output
- Icons: `✔` success, `⚠` warning, `✘` error, `●` current branch, `○` other branch

### TUI Models (Bubble Tea)

Both `attachTUI` and `switchTUI` in `cmd/st/`:
- Use `list.Model` from `charmbracelet/bubbles` for navigation
- **Index-based selection**: `list.Index()` + `candidates[idx].name` (NOT type assertion from list items)
- Search mode activated by `/`, filtered by `updateMatches()`
- `selected` field holds result; `quitting` indicates exit
- `Enter` selects, `q`/`Esc` quits without selection

`attachTUI` supports recursive attachment: if selected parent isn't tracked, recursively prompts to attach it first.

`switchTUI` renders tree with depth-based indentation, supports `n/N` for search match navigation.

## Commands

| Command | Description | Key flags |
|---------|-------------|-----------|
| `new <branch>` | Create from root, checkout | — |
| `append <branch>` | Create child of current, checkout | — |
| `insert <branch>` | Insert before current, reparent+restack downstream, checkout new | — |
| `restack` | Rebase lineage onto parents | `--to-current` |
| `continue` | Resume restack after conflict | — |
| `attach [branch]` | Adopt branch into stack (TUI or auto) | `--auto` |
| `restore [branch]` | Restore from automatic backup | `--all` |
| `sync` | Push to remote | `--dry-run` |
| `backup` | Manual snapshot of all stack branches | — |
| `log` | Display stack tree | — |
| `switch` | Interactive branch selector (TUI) | — |

### Command data flow
```
getContext() → (Graph, git.Runner, output.Printer, repoPath, error)
  └─ loads/creates graph, finds repo root
... perform operations ...
saveContext(g, repoPath) → saves graph JSON
```

## Testing

### Test Architecture
- **Single test file**: `cmd/st/main_test.go` covers all 11 commands + TUI model tests
- **`TestMain`**: Builds binary once to temp file, used by all test functions
- **No `t.Parallel()`**: Tests use `os.Chdir` which is process-global
- **Helpers**: `setupRepo`, `setupRepoWithStack`, `runSt`, `runStExpectError`, `loadGraph`, `graphContains`

### Test helpers (`internal/testutil/git.go`)
- `NewGitRepo()`: Creates temp dir, `git init`, configures user, sets `HOME` for isolation
- `InitStack()`: Creates initial commit + `.git/stack/` directory
- `HeadSHA()`: Returns trimmed HEAD SHA
- `AddRemote()`: Creates bare repo, adds as `origin`
- `FileExists(filename)`: Checks working tree
- `WriteFile(filename, content)`: Writes without staging
- `Cleanup()`: Removes repo dir + origin dir if set

### Running Tests
```bash
# All E2E tests
go test ./cmd/st/ -v -count=1

# All tests in project
go test ./... -v -count=1

# Specific packages
go test ./pkg/graph/... -v
go test ./pkg/restack/... -v
```

### Test Patterns
- **E2E tests** (per command): Set up repo → run `st` binary → assert on git state and graph JSON
- **TUI model tests**: Construct model directly → send `tea.KeyMsg` → assert on `selected`/`quitting` fields
- **Error tests**: Use `runStExpectError` → assert error message content
- **Backup tests**: Automatic backups use `backup/` prefix; manual use `backups/` prefix. `st restore` only finds automatic backups.

### Important test considerations
- `append` on an untracked branch auto-creates a graph with that branch as root (so it "succeeds"). To test the error case, create a graph with a different root first.
- `sync --dry-run` uses `printer.Info()` which requires verbose mode — pass `-v` flag to binary.
- `insert` cleans up automatic backups on success. To test `restore`, create backup branches manually matching the `backup/<branch>/<timestamp>` format.

## Build & Run

```bash
# Build
go build -o st ./cmd/st/

# Run
./st --help
./st new feature-branch
./st log
```

## Dependencies

- `github.com/spf13/cobra` — CLI framework
- `github.com/charmbracelet/bubbletea` — TUI framework
- `github.com/charmbracelet/bubbles` — TUI components (list)
- `github.com/charmbracelet/lipgloss` — Terminal styling

## Code Style

- Go standard formatting (`gofmt`)
- Error messages: lowercase, no trailing period
- `fmt.Errorf("context: %w", err)` for wrapped errors
- Constructors: `NewFoo(...)` pattern
- Public API only for cross-package use
