## Context

Staccato is a Go CLI for managing stacked branches. Commands like `st pr make`, `st sync`, `st restack`, and `st new` produce meaningful lifecycle events but offer no extension point for automation. The project uses Cobra for CLI, a shared `StaccatoContext` for state, and has both CLI commands (`cmd/st/`) and MCP tool handlers (`pkg/mcp/`) that perform the same operations.

Claude Code's hook system serves as the reference model: executable scripts organised in directories by event name, receiving JSON context on stdin, with exit codes controlling flow.

## Goals / Non-Goals

**Goals:**
- Provide a hook execution engine in `pkg/hooks/` that discovers and runs scripts
- Fire hooks from both CLI commands and MCP tool handlers
- Support two directory levels simultaneously: global (`~/.config/staccato/hooks/`) and project (`.staccato/hooks/`)
- Pass structured, event-specific JSON context to hooks via stdin
- Use exit codes for flow control (0 = ok, 2 = block, other = warn)
- Keep the integration surface minimal — 1-2 lines per call site

**Non-Goals:**
- Configuration file format (no JSON/YAML config for hooks — just directories with executables)
- Hook ordering/priority within a directory (filesystem order is fine)
- Async/background hook execution (all hooks run synchronously in v1)
- HTTP or "prompt" hook types (command-only in v1)
- Hook management CLI commands (users create/remove scripts directly)

## Decisions

### 1. Directory-based discovery over config-file-based

Hooks are discovered by scanning `<hook-dir>/<event-name>/` for executable files. No configuration file is needed.

**Why over config file:** Simpler mental model, no schema to maintain, easy to add/remove hooks (just add/delete a file), works with symlinks. Matches the unix `/etc/cron.d/` and `/etc/profile.d/` pattern. Claude Code uses a JSON config, but that's because it needs matchers, async flags, and multiple hook types — we don't need that complexity yet.

**Alternatives considered:**
- JSON config like Claude Code: More flexible but over-engineered for v1 where we only support command hooks
- Git hooks style (single file per event): Limits to one hook per event, requires wrapper scripts for multiple actions

### 2. Two directory levels: global + project

```
~/.config/staccato/hooks/<event>/   ← global (user-wide)
<repo>/.staccato/hooks/<event>/     ← project (repo-specific)
```

Global hooks run first, then project hooks. Both levels are always scanned.

**Why:** Users want personal automations (e.g., desktop notifications) that apply everywhere, plus project-specific hooks (e.g., CI triggers) that ship with the repo. Two levels cover both cases without configuration.

**Why `~/.config/staccato/`:** Follows XDG Base Directory spec. Consistent with where other tools store user config on Linux.

**Why `.staccato/` in repo root:** Clear namespace, not hidden inside `.git/` (so it's committable), mirrors `.github/` convention.

### 3. JSON on stdin for context passing

Each event defines a JSON payload passed to hooks via stdin. Environment variables are also set for common fields (`ST_REPO_PATH`, `ST_EVENT`, `ST_BRANCH`).

**Why stdin over args:** Structured data, no shell escaping issues, extensible without breaking existing hooks. Env vars provide a convenience layer for simple scripts that don't want to parse JSON.

**Why not stdout for results:** In v1, hooks communicate via exit code only. Stdout/stderr are passed through to the user's terminal. This keeps hook scripts simple — a hook is just a script that does something and exits.

### 4. Exit code semantics

| Exit | Meaning | Behaviour |
|------|---------|-----------|
| 0 | Success | Continue normally |
| 2 | Block | Abort the operation (pre-hooks only). Post-hooks: treated as error, print warning |
| Other | Error | Print stderr as warning, continue |

**Why exit 2 for blocking:** Matches Claude Code's convention. Exit 1 is too common (many tools exit 1 on minor issues), so using 2 as the explicit "block" signal reduces false positives.

**Pre vs post distinction:** Only `pre-*` hooks can block operations. `post-*` hooks fire after the operation has already completed, so blocking doesn't make sense — exit 2 from a post-hook prints a warning instead.

### 5. Hook runner as a standalone package

```go
// pkg/hooks/hooks.go

type Event string
const (
    PostPRCreate       Event = "post-pr-create"
    PostBranchCreate   Event = "post-branch-create"
    // ...
)

type Context struct {
    Event    Event             `json:"event"`
    RepoPath string           `json:"repo_path"`
    Branch   string           `json:"branch,omitempty"`
    Data     map[string]any   `json:"data,omitempty"`
}

type Runner struct {
    repoPath string
}

func NewRunner(repoPath string) *Runner
func (r *Runner) Fire(ctx Context) error  // returns error only if pre-hook blocks
```

**Why a `Runner` type:** Captures `repoPath` once, reused across the command lifecycle. Testable — can inject a mock or point at a temp directory.

**Why `Fire()` returns error for blocking:** Callers use `if err := hooks.Fire(...); err != nil { return err }` for pre-hooks. Post-hooks always return nil (warnings are printed, not returned).

### 6. Integration pattern in commands

```go
// In pr.go, after PR creation succeeds:
hookRunner := hooks.NewRunner(repoPath)
hookRunner.Fire(hooks.Context{
    Event:    hooks.PostPRCreate,
    RepoPath: repoPath,
    Branch:   currentBranch,
    Data: map[string]any{
        "base":   branchInfo.Parent,
        "web":    web,
    },
})
```

For MCP tools, the same pattern applies — fire hooks after the operation in the tool handler closure.

### 7. Timeout handling

Each hook script gets a 30-second timeout by default. If a script hangs, it's killed and treated as a non-blocking error (warning printed).

**Why 30s:** Long enough for network calls (Slack webhooks, API calls), short enough that a stuck script doesn't block the user indefinitely.

## Risks / Trade-offs

- **[Slow hooks block CLI]** → 30s timeout per script. Future work could add async execution if users need fire-and-forget hooks.
- **[No hook ordering guarantees]** → Within a directory, execution order depends on `os.ReadDir` (alphabetical). Users can prefix filenames (`01-`, `02-`) if ordering matters.
- **[Hook errors are noisy]** → Non-blocking errors print stderr as a warning. Verbose mode (`--verbose`) could gate this in future, but for v1 it's visible always — better to know a hook failed.
- **[MCP and CLI divergence]** → Both call `hooks.Fire()` but from different code paths. If a command adds a new hookable event, the MCP tool handler must also be updated. Mitigation: document this as a development convention.
