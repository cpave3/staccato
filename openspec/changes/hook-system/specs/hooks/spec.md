## ADDED Requirements

### Requirement: Hook discovery from directories
The system SHALL discover hook scripts by scanning event-named subdirectories within hook directories. An executable file located at `<hook-dir>/<event-name>/<script>` SHALL be registered as a hook for that event. Non-executable files SHALL be ignored.

#### Scenario: Executable script in event directory
- **WHEN** a file at `.staccato/hooks/post-pr-create/notify.sh` exists and is executable
- **THEN** the system SHALL execute `notify.sh` when the `post-pr-create` event fires

#### Scenario: Non-executable file is ignored
- **WHEN** a file at `.staccato/hooks/post-pr-create/README.md` exists but is not executable
- **THEN** the system SHALL NOT attempt to execute it when the `post-pr-create` event fires

#### Scenario: Missing event directory
- **WHEN** no directory exists at `<hook-dir>/post-pr-create/`
- **THEN** the system SHALL treat this as zero hooks for that event (no error)

### Requirement: Two-level hook directory resolution
The system SHALL scan two hook directories for every event, in order:
1. Global: `~/.config/staccato/hooks/`
2. Project: `<repo-root>/.staccato/hooks/`

Hooks from both directories SHALL be collected and executed. Global hooks SHALL execute before project hooks.

#### Scenario: Both global and project hooks exist
- **WHEN** global directory contains `.config/staccato/hooks/post-sync/global.sh` AND project directory contains `.staccato/hooks/post-sync/project.sh`
- **THEN** the system SHALL execute `global.sh` first, then `project.sh`

#### Scenario: Only project hooks exist
- **WHEN** no global hook directory exists for an event but project hooks do
- **THEN** the system SHALL execute only the project hooks without error

#### Scenario: Neither directory exists
- **WHEN** neither global nor project hook directories exist
- **THEN** the system SHALL proceed normally with no hooks executed and no error

### Requirement: JSON context on stdin
The system SHALL pass a JSON payload to each hook script via stdin. The payload SHALL contain at minimum:
- `event`: the event name (string)
- `repo_path`: absolute path to the repository root (string)
- `branch`: the current or relevant branch name (string, when applicable)
- `data`: event-specific key-value data (object)

#### Scenario: Hook receives JSON context
- **WHEN** a `post-pr-create` hook fires for branch `feature-1` with base `main`
- **THEN** stdin SHALL contain JSON with `event` = `"post-pr-create"`, `branch` = `"feature-1"`, and `data.base` = `"main"`

### Requirement: Environment variables for common fields
The system SHALL set environment variables for common context fields when executing hook scripts:
- `ST_EVENT`: the event name
- `ST_REPO_PATH`: absolute path to the repository root
- `ST_BRANCH`: the current or relevant branch name (when applicable)

#### Scenario: Simple script uses environment variables
- **WHEN** a hook script reads `$ST_BRANCH` during a `post-branch-create` event for branch `feature-x`
- **THEN** the value SHALL be `"feature-x"`

### Requirement: Exit code flow control
The system SHALL interpret hook exit codes as follows:
- Exit 0: success, continue normally
- Exit 2: block the operation (pre-hooks only; post-hooks treat this as a warning)
- Any other exit code: non-blocking error, print stderr as a warning, continue

#### Scenario: Pre-hook blocks operation
- **WHEN** a `pre-sync` hook script exits with code 2
- **THEN** the `st sync` operation SHALL be aborted and the hook's stderr SHALL be shown to the user

#### Scenario: Post-hook exit 2 treated as warning
- **WHEN** a `post-pr-create` hook script exits with code 2
- **THEN** the system SHALL print stderr as a warning but SHALL NOT undo the PR creation

#### Scenario: Hook exits with non-zero non-2 code
- **WHEN** a hook script exits with code 1
- **THEN** the system SHALL print stderr as a warning and continue with the operation

#### Scenario: Hook exits successfully
- **WHEN** a hook script exits with code 0
- **THEN** the system SHALL continue normally without any warning

### Requirement: Hook timeout
The system SHALL enforce a 30-second timeout on each hook script execution. If a script exceeds the timeout, the system SHALL kill the process and treat it as a non-blocking error.

#### Scenario: Hook exceeds timeout
- **WHEN** a hook script runs for more than 30 seconds
- **THEN** the system SHALL kill the process and print a timeout warning

#### Scenario: Hook completes within timeout
- **WHEN** a hook script completes in 5 seconds
- **THEN** the system SHALL process the exit code normally

### Requirement: Post-PR-create event
The system SHALL fire a `post-pr-create` event after `st pr make` successfully creates a pull request. The event data SHALL include `base` (target branch) and `web` (whether `--web` flag was used).

#### Scenario: PR created via CLI
- **WHEN** a user runs `st pr make` and the PR is created successfully
- **THEN** the `post-pr-create` event SHALL fire with `branch` set to the current branch and `data.base` set to the parent branch

#### Scenario: PR creation fails
- **WHEN** `st pr make` fails (e.g., push error)
- **THEN** the `post-pr-create` event SHALL NOT fire

### Requirement: Post-branch-create event
The system SHALL fire a `post-branch-create` event after `st new`, `st append`, or `st insert` successfully creates a branch. The event data SHALL include `parent` (the parent branch).

#### Scenario: Branch created with st new
- **WHEN** a user runs `st new feature-1` and the branch is created
- **THEN** the `post-branch-create` event SHALL fire with `branch` = `"feature-1"` and `data.parent` set to the root branch

#### Scenario: Branch created with st append
- **WHEN** a user runs `st append child-1` while on `feature-1`
- **THEN** the `post-branch-create` event SHALL fire with `branch` = `"child-1"` and `data.parent` = `"feature-1"`

### Requirement: Post-branch-delete event
The system SHALL fire a `post-branch-delete` event after `st delete` or `st delete-stack` removes a branch. The event data SHALL include `parent` (the branch's former parent). For `st delete-stack`, one event SHALL fire per deleted branch.

#### Scenario: Single branch deleted
- **WHEN** a user runs `st delete feature-1`
- **THEN** the `post-branch-delete` event SHALL fire with `branch` = `"feature-1"`

#### Scenario: Stack deleted
- **WHEN** a user runs `st delete-stack` and branches `a`, `b`, `c` are removed
- **THEN** three `post-branch-delete` events SHALL fire, one for each branch

### Requirement: Post-restack event
The system SHALL fire a `post-restack` event after `st restack` completes successfully. The event data SHALL include `restacked_count` (number of branches restacked).

#### Scenario: Restack succeeds
- **WHEN** `st restack` completes without conflicts
- **THEN** the `post-restack` event SHALL fire with `data.restacked_count` set to the number of branches restacked

#### Scenario: Restack has conflicts
- **WHEN** `st restack` stops due to a conflict
- **THEN** the `post-restack` event SHALL NOT fire; instead `post-restack-conflict` SHALL fire

### Requirement: Post-restack-conflict event
The system SHALL fire a `post-restack-conflict` event when `st restack` stops due to a merge conflict. The event data SHALL include `conflict_branch` (the branch where the conflict occurred).

#### Scenario: Conflict during restack
- **WHEN** `st restack` encounters a conflict at branch `feature-2`
- **THEN** the `post-restack-conflict` event SHALL fire with `data.conflict_branch` = `"feature-2"`

### Requirement: Post-sync event
The system SHALL fire a `post-sync` event after `st sync` completes. The event data SHALL include `merged_branches` (list of branches detected as merged), `pushed_branches` (list of branches pushed), and `restacked_count`.

#### Scenario: Sync completes with merged branches
- **WHEN** `st sync` detects branches `a`, `b` as merged and pushes `c`
- **THEN** the `post-sync` event SHALL fire with `data.merged_branches` = `["a", "b"]`, `data.pushed_branches` = `["c"]`

#### Scenario: Dry-run sync
- **WHEN** `st sync --dry-run` completes
- **THEN** the `post-sync` event SHALL NOT fire

### Requirement: Post-attach event
The system SHALL fire a `post-attach` event after `st attach` successfully adds a branch to the stack. The event data SHALL include `parent` (the parent branch in the stack).

#### Scenario: Branch attached
- **WHEN** a user runs `st attach feature-1` and it's added under `main`
- **THEN** the `post-attach` event SHALL fire with `branch` = `"feature-1"` and `data.parent` = `"main"`

### Requirement: Pre-sync event
The system SHALL fire a `pre-sync` event before `st sync` begins its operations. If any pre-sync hook exits with code 2, the sync SHALL be aborted.

#### Scenario: Pre-sync hook allows
- **WHEN** all `pre-sync` hooks exit with code 0
- **THEN** the sync operation SHALL proceed

#### Scenario: Pre-sync hook blocks
- **WHEN** a `pre-sync` hook exits with code 2 with stderr "uncommitted changes"
- **THEN** `st sync` SHALL abort and display "uncommitted changes"

### Requirement: Pre-restack event
The system SHALL fire a `pre-restack` event before `st restack` begins. If any pre-restack hook exits with code 2, the restack SHALL be aborted.

#### Scenario: Pre-restack hook blocks
- **WHEN** a `pre-restack` hook exits with code 2
- **THEN** `st restack` SHALL abort without modifying any branches

### Requirement: Hooks fire from MCP tool handlers
The system SHALL fire the same hook events when operations are performed through MCP tools as when performed through CLI commands. The `st_pr`, `st_new`, `st_append`, `st_insert`, `st_restack`, `st_sync`, `st_attach`, `st_delete_stack` MCP tools SHALL fire their corresponding hook events.

#### Scenario: MCP tool fires hook
- **WHEN** the `st_pr` MCP tool creates a PR
- **THEN** the `post-pr-create` event SHALL fire with the same context as the CLI equivalent

#### Scenario: MCP pre-hook blocks operation
- **WHEN** a `pre-restack` hook exits with code 2 during an `st_restack` MCP tool call
- **THEN** the MCP tool SHALL return an error indicating the operation was blocked by a hook
