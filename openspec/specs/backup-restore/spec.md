# Backup & Restore

This specification covers the backup and restore system for Staccato, including automatic backups created during restack/insert operations, manual backups created on demand, listing, interactive cleanup, and restoration.

---

## Automatic Backups

### Requirement: Automatic Backup Creation

The system SHALL create automatic backups of branches during restack and insert operations. Automatic backup branch names SHALL follow the format `backup/auto/<branch>/<nanosecond-timestamp>`, where the timestamp is `time.Now().UnixNano()`. The backup SHALL be created by copying the branch ref via `git branch --copy`.

#### Scenario: Create automatic backup of a single branch

- **WHEN** a restack or insert operation begins for a branch
- **THEN** an automatic backup branch SHALL be created at `backup/auto/<branch>/<nanosecond-timestamp>`
- **AND** the backup branch SHALL point to the same commit as the original branch

#### Scenario: Create automatic backups for an entire stack

- **WHEN** a restack or insert operation involves multiple branches in a stack
- **THEN** an automatic backup SHALL be created for each branch in the stack
- **AND** a mapping from original branch name to backup branch name SHALL be returned

### Requirement: Automatic Backup Cleanup on Success

Automatic backups SHALL be cleaned up after a successful restack or insert operation. The `CleanupStackBackups` method SHALL delete all automatic backups for the given set of branches.

#### Scenario: Successful restack cleans up automatic backups

- **WHEN** a restack operation completes successfully
- **THEN** all automatic backup branches created for that operation SHALL be deleted

#### Scenario: Failed restack preserves automatic backups

- **WHEN** a restack operation fails (e.g., due to a merge conflict)
- **THEN** the automatic backup branches SHALL be preserved for later restoration

### Requirement: Automatic Backup Retention Policy

The `CleanupOldBackups` method SHALL retain only the specified number of most recent automatic backups for a given branch, deleting the oldest backups first. Backups SHALL be sorted by timestamp in descending order (newest first).

#### Scenario: Cleanup retains specified number of backups

- **WHEN** `CleanupOldBackups` is called with `keep=2` and 5 backups exist for a branch
- **THEN** the 3 oldest backups SHALL be deleted
- **AND** the 2 newest backups SHALL be preserved

#### Scenario: Cleanup is a no-op when backups are within limit

- **WHEN** `CleanupOldBackups` is called with `keep=3` and only 2 backups exist
- **THEN** no backups SHALL be deleted

---

## Manual Backups

### Requirement: Manual Backup Creation

The `st backup` command SHALL create a manual snapshot of every branch in the stack, excluding the root branch. Manual backup branch names SHALL follow the format `backup/manual/<YYYY-MM-DD_HH-MM-SS>/<branch>`. All branches in a single manual backup operation SHALL share the same timestamp.

#### Scenario: Create manual backup of stack

- **WHEN** the user runs `st backup`
- **THEN** a backup branch SHALL be created for each non-root branch in the stack
- **AND** the backup branch names SHALL use the format `backup/manual/<YYYY-MM-DD_HH-MM-SS>/<branch>`
- **AND** an informational message SHALL be printed with the timestamp and number of branches backed up

#### Scenario: Manual backup with empty stack

- **WHEN** the user runs `st backup` and there are no non-root branches in the stack
- **THEN** the command SHALL return an error with the message "no branches in the stack to backup"

### Requirement: Manual Backup Persistence

Manual backups SHALL persist until explicitly deleted by the user. They SHALL NOT be automatically cleaned up by restack or insert operations.

#### Scenario: Manual backups survive restack

- **WHEN** manual backups exist and a restack operation completes
- **THEN** the manual backups SHALL still exist

---

## Backup Listing

### Requirement: List All Backups

The `st backup list` command SHALL display all backup branches (both automatic and manual), sorted newest-first. Each entry SHALL display the backup kind (auto or manual), the timestamp formatted as `YYYY-MM-DD HH:MM:SS`, and the source branch name.

#### Scenario: List backups when backups exist

- **WHEN** the user runs `st backup list` and backups exist
- **THEN** the output SHALL display the total count of backups
- **AND** each backup SHALL be displayed with its kind tag (`[auto]` or `[manual]`), timestamp, and source branch name
- **AND** backups SHALL be sorted from newest to oldest

#### Scenario: List backups when no backups exist

- **WHEN** the user runs `st backup list` and no backups exist
- **THEN** the output SHALL display "No backups found"

### Requirement: List Automatic Backups for a Branch

The `ListBackups` method SHALL return all automatic backups for a specific branch, sorted newest-first. It SHALL match both the current format (`backup/auto/<branch>/<timestamp>`) and the legacy format (`backup/<branch>/<timestamp>`), while excluding branches under the `auto/` or `manual/` subpaths from legacy matching.

#### Scenario: List auto backups for a specific branch

- **WHEN** `ListBackups` is called for a branch with multiple automatic backups
- **THEN** all matching backup branch names SHALL be returned
- **AND** results SHALL be sorted by timestamp descending (newest first)

#### Scenario: Legacy backup format is recognized

- **WHEN** backups exist in the legacy format `backup/<branch>/<timestamp>`
- **THEN** `ListBackups` SHALL include them in the results
- **AND** branches under `backup/auto/` or `backup/manual/` SHALL NOT be matched by the legacy pattern

---

## Backup Parsing

### Requirement: Backup Branch Name Parsing

The system SHALL recognize four backup branch name formats and parse them into structured `BackupInfo` records containing the branch ref, source branch name, kind (auto or manual), and timestamp.

#### Scenario: Parse new auto format

- **WHEN** a branch name matches `backup/auto/<branch>/<nanosecond-timestamp>`
- **THEN** it SHALL be parsed as an automatic backup with the correct source branch and timestamp

#### Scenario: Parse new manual format

- **WHEN** a branch name matches `backup/manual/<YYYY-MM-DD_HH-MM-SS>/<branch>`
- **THEN** it SHALL be parsed as a manual backup with the correct source branch and timestamp

#### Scenario: Parse legacy auto format

- **WHEN** a branch name matches `backup/<branch>/<nanosecond-timestamp>` and the branch segment is not `auto` or `manual`
- **THEN** it SHALL be parsed as an automatic backup with the correct source branch and timestamp

#### Scenario: Parse legacy manual format

- **WHEN** a branch name matches `backups/<YYYY-MM-DD_HH-MM-SS>/<branch>`
- **THEN** it SHALL be parsed as a manual backup with the correct source branch and timestamp

#### Scenario: Unrecognized branch name

- **WHEN** a branch name does not match any known backup format
- **THEN** parsing SHALL return false (not recognized)

---

## Restore

### Requirement: Restore Single Branch from Automatic Backup

The `st restore [branch]` command SHALL restore a branch from its most recent automatic backup. If no branch name is provided, the current branch SHALL be used. Restoration SHALL delete the original branch, copy the backup to the original branch name, check out the restored branch, and then delete the backup ref.

#### Scenario: Restore a named branch

- **WHEN** the user runs `st restore <branch>` and an automatic backup exists for that branch
- **THEN** the original branch SHALL be replaced with the backup contents
- **AND** the restored branch SHALL be checked out
- **AND** the backup branch SHALL be deleted after restoration

#### Scenario: Restore current branch (no argument)

- **WHEN** the user runs `st restore` without a branch argument
- **THEN** the current branch SHALL be used as the target for restoration

#### Scenario: Restore when currently on the target branch

- **WHEN** the user is on the branch being restored
- **THEN** the system SHALL first check out the backup branch temporarily
- **AND** then delete the original, copy the backup, and check out the restored branch

#### Scenario: Restore with no backups available

- **WHEN** the user runs `st restore <branch>` and no automatic backups exist for that branch
- **THEN** the command SHALL return an error with the message "no backups found for branch '<branch>'"

### Requirement: Restore All Branches in Stack

The `st restore --all` command SHALL restore all branches in the current stack that have automatic backups. It SHALL abort any in-progress rebase before restoring, update the graph state after restoration, save the context, and clear restack state.

#### Scenario: Restore all branches after failed restack

- **WHEN** the user runs `st restore --all` and automatic backups exist for stack branches
- **THEN** each branch with a backup SHALL be restored from its most recent automatic backup
- **AND** the graph SHALL be updated with the new HEAD and base SHAs for each restored branch
- **AND** the restack state file SHALL be cleared

#### Scenario: Restore all aborts in-progress rebase

- **WHEN** the user runs `st restore --all` and a rebase is in progress
- **THEN** the in-progress rebase SHALL be aborted before restoration begins

#### Scenario: Restore all with rebase abort failure

- **WHEN** the user runs `st restore --all` and the rebase abort fails
- **THEN** the command SHALL return an error with message "failed to abort rebase: <details>"

#### Scenario: Restore all skips root branch in graph update

- **WHEN** `st restore --all` updates the graph state
- **THEN** the root branch SHALL be skipped during graph SHA updates

#### Scenario: Partial restore failure

- **WHEN** `st restore --all` is run and some branches fail to restore
- **THEN** an error message SHALL be printed for each failed branch
- **AND** restoration SHALL continue for remaining branches

---

## Backup Cleanup

### Requirement: Interactive Backup Cleanup

The `st backup clean` command SHALL present an interactive TUI for selecting and deleting backup branches. Backups SHALL be grouped by source branch name (sorted alphabetically), with branch names displayed as section headers.

#### Scenario: Launch cleanup with existing backups

- **WHEN** the user runs `st backup clean` and backups exist
- **THEN** a TUI SHALL be displayed listing all backups grouped by source branch
- **AND** the cursor SHALL initially be positioned on the first non-header item

#### Scenario: Launch cleanup with no backups

- **WHEN** the user runs `st backup clean` and no backups exist
- **THEN** the message "No backups to clean" SHALL be printed
- **AND** no TUI SHALL be launched

### Requirement: Cleanup TUI Navigation

The cleanup TUI SHALL support keyboard navigation with `up`/`k` to move up and `down`/`j` to move down. The cursor SHALL skip over header items (branch name headers are not selectable).

#### Scenario: Navigate past headers

- **WHEN** the user presses `down`/`j` and the next item is a header
- **THEN** the cursor SHALL skip the header and land on the next selectable backup item

### Requirement: Cleanup TUI Selection

The cleanup TUI SHALL support toggling individual items with `space`, selecting all items with `a`, and deselecting all items with `n`. Header items SHALL NOT be selectable.

#### Scenario: Toggle individual backup selection

- **WHEN** the user presses `space` on a backup item
- **THEN** the item's selection state SHALL be toggled

#### Scenario: Select all backups

- **WHEN** the user presses `a`
- **THEN** all non-header backup items SHALL be selected

#### Scenario: Deselect all backups

- **WHEN** the user presses `n`
- **THEN** all selections SHALL be cleared

#### Scenario: Space on a header item

- **WHEN** the user presses `space` while the cursor is on a header item
- **THEN** nothing SHALL happen (headers are not selectable)

### Requirement: Cleanup TUI Confirmation

The cleanup TUI SHALL require confirmation before deleting selected backups. Pressing `enter` with selections SHALL enter confirmation mode. Pressing `y`/`Y` SHALL confirm deletion and quit. Pressing any other key SHALL cancel confirmation and return to selection mode.

#### Scenario: Confirm deletion

- **WHEN** the user presses `enter` with selections and then presses `y`
- **THEN** the selected backups SHALL be deleted
- **AND** a message SHALL display the number of deleted backups

#### Scenario: Cancel confirmation

- **WHEN** the user presses `enter` with selections and then presses any key other than `y`/`Y`
- **THEN** the TUI SHALL return to selection mode without deleting

#### Scenario: Enter with no selections

- **WHEN** the user presses `enter` with no items selected
- **THEN** the TUI SHALL NOT enter confirmation mode

### Requirement: Cleanup TUI Quit

The cleanup TUI SHALL support quitting without deletion by pressing `q` or `esc`.

#### Scenario: Quit without deleting

- **WHEN** the user presses `q` or `esc`
- **THEN** the TUI SHALL exit without deleting any backups

---

## Error Cases

### Requirement: Backup Existence Validation

Restore and delete operations SHALL verify that the backup branch exists before proceeding.

#### Scenario: Restore nonexistent backup

- **WHEN** `RestoreBackup` is called with a backup name that does not exist as a branch
- **THEN** it SHALL return an error with message "backup <name> does not exist"

#### Scenario: Delete nonexistent backup

- **WHEN** `DeleteBackup` is called with a backup name that does not exist
- **THEN** it SHALL return an error with message "backup <name> does not exist"

### Requirement: Restore Rollback on Failure

If the original branch deletion fails during restore, the system SHALL attempt to restore the original state by checking out the original branch.

#### Scenario: Branch deletion fails during restore

- **WHEN** `RestoreBackup` is unable to delete the original branch
- **THEN** the system SHALL attempt to check out the original branch to restore state
- **AND** an error SHALL be returned with the message "failed to delete original branch: <details>"

### Requirement: Partial Stack Backup Failure

If a backup fails for one branch during stack backup creation, the error SHALL be returned immediately with the partial backup map.

#### Scenario: Stack backup fails mid-operation

- **WHEN** `CreateBackupsForStack` fails to back up one branch in the list
- **THEN** the successfully created backups SHALL be returned in the map
- **AND** an error SHALL be returned indicating which branch failed
