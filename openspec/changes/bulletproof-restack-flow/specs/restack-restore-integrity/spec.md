## ADDED Requirements

### Requirement: Restore All Returns Exact Pre-Restack State

The `st restore --all` command SHALL return the repository to the exact state it was in before the restack began. This includes branch HEAD SHAs matching pre-restack values, graph SHAs being updated to match restored branches, and the restack state file being cleared.

#### Scenario: Restore after conflict returns to pre-restack state

- **WHEN** a restack encounters a conflict at branch s1
- **AND** the user runs `st restore --all`
- **THEN** all branches SHALL have the same HEAD SHA as before the restack
- **AND** the graph SHALL have BaseSHA and HeadSHA values matching the restored branch state
- **AND** the restack state file SHALL be cleared
- **AND** no rebase SHALL be in progress

#### Scenario: Restore after partial continue returns to pre-restack state

- **WHEN** a restack encounters a conflict at s1, the user resolves and continues, then encounters a conflict at s2
- **AND** the user runs `st restore --all`
- **THEN** all branches SHALL have the same HEAD SHA as before the original restack began
- **AND** the graph SHAs SHALL be consistent with the restored branches
- **AND** branches that were successfully rebased during continue SHALL be reverted to their pre-restack state

#### Scenario: Restore preserves file contents

- **WHEN** a restack is in progress with a conflict
- **AND** the user runs `st restore --all`
- **THEN** the file contents on each branch SHALL match what they were before the restack
- **AND** no rebase artifacts (conflict markers, .git/rebase-merge) SHALL remain

### Requirement: Restore All Handles In-Progress Rebase

When `st restore --all` is run during an in-progress rebase (conflict state), it SHALL abort the rebase before restoring branches. The abort SHALL succeed even if the working tree has uncommitted conflict resolution changes.

#### Scenario: Restore aborts active rebase before restoring

- **WHEN** a restack has stopped due to a conflict (rebase in progress)
- **AND** the user runs `st restore --all` without resolving the conflict
- **THEN** the in-progress rebase SHALL be aborted
- **AND** all branches SHALL be restored from their backups
- **AND** the user SHALL be on a valid branch (not detached HEAD)

### Requirement: Restore Updates Graph Children BaseSHA

After restoring branches, the graph SHALL have consistent parent-child SHA relationships. Each branch's BaseSHA SHALL correspond to the actual merge-base between the branch and its parent after restoration.

#### Scenario: Children BaseSHA consistent after restore

- **WHEN** `st restore --all` completes
- **THEN** for each non-root branch in the graph, the recorded BaseSHA SHALL match the actual merge-base between the branch and its parent branch
- **AND** the recorded HeadSHA SHALL match the actual HEAD of the branch

### Requirement: Single Branch Restore Preserves Stack Integrity

When restoring a single branch (not `--all`), the graph SHALL be updated for that branch, and the stack SHALL remain in a usable state for subsequent operations.

#### Scenario: Restore single branch updates graph

- **WHEN** the user runs `st restore <branch>` for a branch that has an automatic backup
- **THEN** the branch SHALL be restored from the backup
- **AND** the graph's HeadSHA for that branch SHALL be updated to match the restored branch's HEAD
- **AND** downstream branches SHALL remain in the graph (they may need restacking)

### Requirement: Backup Integrity During Multi-Continue Cycle

Automatic backups created at the start of a restack SHALL survive through multiple conflict → continue cycles. The backups SHALL always reflect the pre-restack state, not any intermediate rebased state.

#### Scenario: Backups survive multiple continue cycles

- **WHEN** a restack creates backups and then encounters multiple conflicts requiring multiple `st continue` invocations
- **THEN** the original automatic backups SHALL still exist after each continue
- **AND** restoring from these backups SHALL return to the pre-restack state (not an intermediate state)

#### Scenario: Backups cleaned up only on final success

- **WHEN** a restack with multiple conflicts is fully resolved via `st continue`
- **AND** all branches are successfully rebased
- **THEN** the automatic backups SHALL be cleaned up
- **AND** no orphaned backup branches SHALL remain
