## Why

Staccato commands produce meaningful lifecycle events (PRs created, branches restacked, syncs completed) but there's no way to trigger custom automations off them. Users need extensibility — e.g., posting to Slack when a PR is created, running linters after a restack, or logging sync results — without modifying staccato itself. A hook system modelled after Claude Code's approach (executable scripts in hook directories) provides this extensibility with minimal complexity.

## What Changes

- Add a hook execution engine that discovers and runs executable scripts from hook directories
- Define lifecycle events across all major staccato operations that can trigger hooks
- Pass structured JSON context to hook scripts via stdin
- Support two hook directory levels, both active simultaneously:
  - **Global**: `~/.config/staccato/hooks/<event>/` — user-wide hooks applied to all repos
  - **Project**: `.staccato/hooks/<event>/` — repo-specific hooks, committable and shareable
- When an event fires, hooks from **both** directories are discovered and executed (global first, then project)
- All executable files within an event directory are run when that event fires
- Hook exit codes control flow: exit 0 = success, exit 2 = block/abort, other = warn and continue

### Hook Events

The following lifecycle events will trigger hooks:

| Event | Fires when | Example use case |
|-------|-----------|-----------------|
| `post-pr-create` | After `st pr make` creates a PR | Slack notification, open in browser |
| `post-branch-create` | After `st new`, `st append`, `st insert` | Set up branch config, create draft PR |
| `post-branch-delete` | After `st delete` or `st delete-stack` removes branches | Clean up remote branches, notify team |
| `post-restack` | After `st restack` completes successfully | Run tests, push branches |
| `post-restack-conflict` | When restack stops on a conflict | Notify, log conflict details |
| `post-sync` | After `st sync` completes | Log sync results, notify of merged branches |
| `post-attach` | After `st attach` adds a branch to the stack | Auto-restack, create PR |
| `pre-sync` | Before `st sync` begins | Stash changes, run pre-flight checks |
| `pre-restack` | Before `st restack` begins | Validate working tree, stash changes |

## Capabilities

### New Capabilities
- `hooks`: Hook discovery, execution engine, event lifecycle, and configuration

### Modified Capabilities

_(none — this is a new subsystem that integrates into existing commands without changing their spec-level behaviour)_

## Impact

- **Code**: Every command that produces a hookable event needs a call to the hook runner after its operation completes. This is a small addition at the call site (1-2 lines per command).
- **New package**: `pkg/hooks/` — hook discovery, execution, and event definitions.
- **CLI commands**: Modified to fire events: `pr.go`, `new.go`, `append.go`, `insert.go`, `delete.go`, `delete_stack.go`, `restack.go`, `sync.go`, `attach.go`.
- **MCP tools**: Corresponding MCP tool handlers in `pkg/mcp/` should also fire hooks when they perform the same operations.
- **Dependencies**: No new external dependencies — uses `os/exec` and filesystem discovery.
- **File system**: Establishes `.staccato/hooks/` convention for project hooks and `~/.config/staccato/hooks/` for global hooks.
