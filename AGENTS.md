# AGENTS.md

## Project Overview

**st (Staccato)** is a deterministic, offline-first Git stack management CLI written in Go. It provides branch-level stacking with automatic backups, lazy attachment, and interactive TUI for branch selection.

## Architecture

### Project Structure
```
st/
├── cmd/st/
│   ├── main.go           # Entry point, getContext/saveContext, init
│   ├── graph.go           # graph share/local/which subcommands
│   ├── sync.go            # sync command (fetch, merge detect, restack, push)
│   ├── attach.go          # attach command + attachTUI model
│   ├── switch.go          # switchTUI model + switch command
│   ├── new.go, append.go, insert.go, restack.go, continue.go
│   ├── backup.go, restore.go, log.go, status.go, pr.go
│   ├── split.go           # split describe/apply (thin wrapper over pkg/split)
│   ├── exec.go            # exec command (thin wrapper over pkg/stackexec)
│   ├── graph_transfer.go  # graph export/import (thin wrapper over pkg/transfer)
│   ├── version.go, update.go  # st version, st update (pkg/version, pkg/selfupdate)
│   ├── help.go            # help pages, 'st help' agent primer, usage errors
│   └── *_test.go          # E2E + TUI model tests by command/behavior
├── internal/testutil/
│   └── git.go             # Test helpers (GitRepo, isolated repos)
├── pkg/
│   ├── graph/             # Stack graph model & JSON persistence
│   ├── git/               # Git operations wrapper (subprocess)
│   ├── backup/            # Backup/restore system (auto + manual)
│   ├── restack/           # Topological sort & restack engine
│   ├── attach/            # Lazy attachment logic
│   ├── split/             # Split one branch into stacked branches from a plan
│   ├── stackexec/         # Run a command on every branch of a lineage
│   ├── transfer/          # Graph export document and per-branch import merge
│   ├── version/           # The binary's version (stamped at release build)
│   ├── selfupdate/        # Find the newest GitHub release and install it
│   ├── forge/             # Forge detection (GitHub, etc.)
│   ├── hooks/             # Hook discovery & execution engine
│   └── output/            # CLI formatting utilities
├── scripts/next-version.sh  # Next release tag (used by the release workflow)
├── .github/workflows/release.yml  # Manual release: tag, test, build, publish
├── .golangci.yml          # Linter config (govet, staticcheck, unused, ineffassign)
├── Taskfile.yml           # Task runner (task build/test/lint/check)
├── go.mod
└── st                     # Compiled binary
```

### Key Components

#### pkg/graph
- `Graph` struct: Root branch + `map[string]*Branch` of tracked branches
- `Branch` struct: `Name`, `Parent`, `BaseSHA`, `HeadSHA`
- Two storage modes (branching logic lives in `cmd/st/main.go`):
  - **Local**: JSON file at `.git/stack/graph.json` (default)
  - **Shared**: Blob stored at `refs/staccato/graph` git ref (opt-in via `st graph share`)
- Constants: `DefaultGraphPath`, `SharedGraphRef`
- `GetChildren(parent)` returns `[]*Branch`
- `ValidateNoCycle()` prevents circular dependencies

#### pkg/git
- `Runner` struct: Wraps git subprocess calls via `exec.Command`
- All output is trimmed via `strings.TrimSpace`
- Key methods: `CreateAndCheckoutBranch`, `Rebase`, `RebaseContinue`, `IsRebaseInProgress`, `GetMergeBase`, `CopyBranch`, `HasRemote`, `Push`
- Ref helpers: `RefExists`, `ReadBlobRef`, `WriteBlobRef`, `DeleteRef`, `PushRef`, `FetchRef` — used for shared graph storage
- Refspec helpers: `AddFetchRefspec`, `RemoveFetchRefspec`, `HasFetchRefspec` — configure auto-fetch of `refs/staccato/*`
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

#### pkg/split
- `cmd/st/split.go` is a thin wrapper over it
- `Describe(sc, branch)`: inventory of a branch (commits, changed files, churn) as a `Description`
- `ParsePlan(data)` / `Plan.Validate()`: strict JSON decoding (unknown keys rejected) and shape checks
- `Apply(sc, plan, opts)`: builds every slice, then moves all branches in one `git update-ref --stdin` transaction
- Slices are built with plumbing in a scratch index (`GIT_INDEX_FILE`, `read-tree`, `update-index --index-info`, `write-tree`, `commit-tree`); the working tree and real index are never touched
- **Invariant**: the top slice's tree equals the original branch's tree. Everything else relies on it: because content is identical, descendants are re-parented with `commit-tree` (same tree, new parent) instead of going through the restack engine, so no checkout happens and no conflict is possible
- `--exec` verification (`verify.go`) is the only part that checks anything out; it restores the starting checkout on failure and on SIGINT/SIGTERM

#### pkg/stackexec
- `Run(sc, opts)`: checks out each branch of the current lineage in turn (detached, bottom to top), runs a shell command in it, and returns a `Result` with one `BranchResult` per branch. Backs `st exec` (`cmd/st/exec.go`)
- A failing command is data in the `Result`, not an error. `Run` returns an error only when st cannot do its own part, and then returns the partial `Result` with it
- Returns to the starting branch when it ends, including on SIGINT/SIGTERM. Tracked-file changes the command makes are stashed, so they are not carried into the next branch
- Branches are visited parents-first with siblings in name order (`lineage`), because `Graph.GetChildren` iterates a map
- Same mechanism as `--exec` in `pkg/split/verify.go`, but separate code: a fix to process or signal handling in one probably belongs in the other too

#### pkg/transfer
- `cmd/st/graph_transfer.go` is a thin wrapper over it
- `Document` (`format`, `kind`, `root`, ordered `branches`) is an exchange format, not storage: it is not `graph.json`. Raise `Format` only for a change an older st cannot read; `Decode` refuses a newer format and ignores unknown fields
- `Export(sc, opts)`: the current stack (bottom branch up), `--branch`, or `--all`; head is git's tip, which must equal `<remote>/<name>` unless `AllowUnpushed`
- `Plan(sc, doc, opts)`: validates names with `git check-ref-format` before anything reaches git (the document comes from another machine), fetches, checks commits, decides each branch's `Outcome` from git ancestry (never from the graphs' save counters), cascades `skipped` to children of unmerged parents, refuses cycles. Changes nothing but remote-tracking refs
- `Apply(sc, plan)`: `merge --ff-only` for the checked-out branch first (a refusal re-decides with it skipped), then one `update-ref --stdin` transaction with expected old values, then one `sc.Save()` (local or shared)
- Shared mode's `sync.ReconcileGraphs` is separate and unchanged

#### pkg/version and pkg/selfupdate
- `version.stamped` is set by `-ldflags -X github.com/cpave3/staccato/pkg/version.stamped=vX.Y.Z` in `task build:release`. Only a stamped `vX.Y.Z` is a release (`IsRelease`); a `go install …@vX.Y.Z` build reports its module version but is not a release; anything else is `dev`
- `selfupdate` is adapted from Legato's `internal/engine/selfupdate`: keep the two the same shape so a fix carries across. `st update` is the only command that contacts GitHub; tests point it at an `httptest` server with `ST_RELEASES_API`
- E2E tests of `st update` must run a copy of a stamped build in a temp dir, never the shared test binary, which it would replace

#### pkg/hooks
- `Event` type with constants: `PostPRCreate`, `PostPRView`, `PostBranchCreate`, `PostBranchDelete`, `PostRestack`, `PostRestackConflict`, `PostSync`, `PostAttach`, `PreSync`, `PreRestack`
- `Context` struct: `Event`, `RepoPath`, `Branch`, `Data map[string]any` — marshalled as JSON to hook stdin
- `Runner` struct: Created with `NewRunner(repoPath)`, discovers hooks from two directories:
  - Global: `~/.config/staccato/hooks/<event>/`
  - Project: `<repo>/.staccato/hooks/<event>/`
- `Fire(ctx)`: Discovers all executable scripts for the event, runs each with JSON on stdin and env vars (`ST_EVENT`, `ST_REPO_PATH`, `ST_BRANCH`)
- Exit codes: 0=success, 2=block (pre-hooks) or warn (post-hooks), other=warn and continue
- 30-second timeout per script
- Commands fire hooks after their operations

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

`attachTUI` supports recursive attachment: if selected parent isn't tracked, recursively prompts to attach it first. Press `r` to set a branch as root (stops recursion). Trunk branches (`main`, `master`, `develop`, `trunk`) are auto-detected as roots. Solo roots (root with no branches in graph) fall through to TUI instead of showing "already in stack".

`switchTUI` renders tree with depth-based indentation, supports `n/N` for search match navigation.

## Commands

| Command | Description | Key flags |
|---------|-------------|-----------|
| `new <branch>` | Create from root, checkout | — |
| `append <branch>` | Create child of current, checkout | — |
| `insert <branch>` | Insert before current, reparent+restack downstream, checkout new | — |
| `restack` | Rebase lineage onto parents | `--to-current` |
| `continue` | Resume restack after conflict | — |
| `attach [branch]` | Adopt branch into stack (TUI or auto) | `--auto`, `--parent` |
| `restore [branch]` | Restore from automatic backup | `--all` |
| `sync` | Fetch, drop merged, restack, push; refused pushes exit 1 | `--dry-run`, `--down`, `--json` |
| `backup` | Manual snapshot of all stack branches | — |
| `log` | Display stack tree | `--json` |
| `switch` | Interactive branch selector (TUI) | — |
| `split describe [branch]` | JSON inventory of a branch for planning a split | — |
| `split apply <plan.json\|->` | Split a branch into stacked branches from a JSON plan | `--dry-run`, `--exec`, `--json` |
| `exec <command>` | Run a command on every branch of the current lineage, report pass/fail per branch | `--fail-fast`, `--json` |
| `version` | Print the version (`dev` for a build from source) | `--json` |
| `update` | Install the newest release over the running binary | `--check`, `--json` |

| `graph share` | Move graph to shared git ref | — |
| `graph local` | Move graph back to local file | — |
| `graph which` | Show current storage mode | — |
| `graph export` | Write a stack's graph as a document for another clone | `--branch`, `--all`, `--output`, `--remote`, `--allow-unpushed` |
| `graph import <file\|->` | Merge a graph document into this clone's graph | `--remote`, `--no-fetch`, `--dry-run`, `--json` |

### Help

Help is the interface documentation: there is no other place a user or an agent learns a command. `cmd/st/help.go` renders every page in one layout (summary, usage, description, commands, arguments, options, examples) from the cobra command, so a command only supplies the content:

- `Use`: the command and its operands, e.g. `"attach [branch]"`. `[options]` is appended when the command has flags of its own
- `Short`: one line, at most 59 characters, no full stop
- `Long`: what it does, what it needs, what it leaves behind, and the non-interactive alternative if it opens a TUI
- `Example`: one runnable command per line, most typical first. A group's help previews the first three of each of its commands
- `withArgs(cmd, argDoc{...})`: one entry per operand in `Use`
- Flag descriptions name their value in backticks: ``"Parent `branch`"`` renders as `--parent <branch>`

`st help` prints the agent primer (`agentPrimer`): concepts and cross-command workflows, plus the command list generated from the tree. A workflow that spans commands belongs there, not in one command's help.

Tests in `cmd/st/help_test.go` hold this together: `TestEveryCommandIsDocumented` fails for a command without a summary, description, examples or argument docs, and `TestExamplesUseRealCommandsAndFlags` fails for an example, in a command or the primer, that names a command or flag that does not exist.

Errors: `main` prints a returned error once. `usageError` adds the usage line and a pointer to `--help` when the command was invoked wrongly (bad arguments or flags); a command that failed while doing its job prints only its reason.

### Command data flow
```
getContext() → (Graph, git.Runner, output.Printer, repoPath, error)
  └─ checks SharedGraphRef first, then local file, then creates new
... perform operations ...
saveContext(g, repoPath, gitRunner) → writes to ref (shared) or file (local)
```

**Important**: `saveContext` takes a `gitRunner` parameter — all call sites must pass it. When adding new commands that save, follow this signature.

## Testing

### Test Architecture
- **Test files by area**: `cmd/st/main_test.go` holds shared helpers and `TestMain`; other `cmd/st/*_test.go` files cover commands, workflows, and TUI models.
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
- **Never hardcode branch names like `"main"`** — use the `root` variable from `setupRepoWithStack`. The default branch depends on the user's git config (`main` vs `master`).
- **Never test TUI launch via `runStExpectError`** expecting a TTY failure — it blocks for ~40s waiting for input. Use model-level tests instead (construct the TUI model directly, send `tea.KeyMsg`).
- `loadGraph` helper checks shared ref first, then falls back to local file — mirrors `getContext` behavior.
- **Avoid network calls in tests**: `FetchRef` and `PushRef` hit the network. Test repos using `AddRemote()` get local bare repos, not real remotes. Never trigger speculative fetches in `getContext` — that causes credential prompts against the user's real GitHub remote.

## Build & Run

```bash
# Using task runner (preferred)
task build          # Build binary
task test           # Run all tests
task lint           # Run golangci-lint
task check          # Build + test + lint

# Or directly
go build -o st ./cmd/st/
go test ./... -count=1
golangci-lint run ./...
```

## Releases

- `gh workflow run release -f bump=patch|minor|major` on `main`: the next version is the newest `vX.Y.Z` tag bumped (`scripts/next-version.sh`); the workflow vets and tests, runs `task release:dist VERSION=<v>` (linux/amd64, linux/arm64, darwin/arm64 into `dist/st-<os>-<arch>` plus `SHA256SUMS`), checks the binary reports the version, attests provenance and publishes with `gh release create`. The tag is the version; nothing is committed back
- Asset names carry no version, so `releases/latest/download/st-linux-amd64` is always the newest
- Binaries are built with `CGO_ENABLED=0`; keep it that way

## Dependencies

- `github.com/spf13/cobra` — CLI framework
- `github.com/charmbracelet/bubbletea` — TUI framework
- `github.com/charmbracelet/bubbles` — TUI components (list)
- `github.com/charmbracelet/lipgloss` — Terminal styling

## Development Process

IMPORTANT: All feature development and bug fixes MUST follow TDD (Test-Driven Development), using the tdd skill:

1. **RED** — Write failing tests first. Tests should describe the expected behavior.
2. **GREEN** — Write the minimal implementation to make tests pass.
3. **REFACTOR** — Clean up duplication while keeping tests green.

Run `task check` to build, test, and lint in one step.

## Code Style

- Go standard formatting (`gofmt`)
- Error messages: lowercase, no trailing period
- `fmt.Errorf("context: %w", err)` for wrapped errors
- Constructors: `NewFoo(...)` pattern
- Public API only for cross-package use
- Linting: `golangci-lint` with `govet`, `staticcheck`, `unused`, `ineffassign` enabled. Run `task lint` or `golangci-lint run ./...` — zero findings expected on main.
