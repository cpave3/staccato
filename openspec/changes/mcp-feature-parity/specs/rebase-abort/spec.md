## ADDED Requirements

### Requirement: Abort In-Progress Rebase (st abort)
The `st abort` command SHALL cancel an in-progress rebase operation, clear staccato's restack state, and restore branches from automatic backups if available. If no rebase is in progress, the command SHALL error.

#### Scenario: Abort active rebase
- **WHEN** `st abort` is run while a rebase is in progress
- **THEN** `git rebase --abort` SHALL be executed
- **AND** the restack state file SHALL be cleared
- **AND** if automatic backups exist for branches in the lineage, they SHALL be restored
- **AND** the graph SHALL be saved

#### Scenario: Abort with backup restoration
- **WHEN** `st abort` is run during a restack that created automatic backups
- **THEN** each branch with an automatic backup SHALL be restored to its pre-restack state
- **AND** the backup refs SHALL be cleaned up after restoration

#### Scenario: No rebase in progress
- **WHEN** `st abort` is run with no rebase in progress
- **THEN** the command SHALL exit with an error: `"no rebase in progress — nothing to abort"`

#### Scenario: Abort clears restack state
- **WHEN** `st abort` successfully cancels a rebase
- **THEN** the restack state file at `.git/stack/restack-state.json` SHALL be deleted
- **AND** subsequent calls to `st continue` SHALL error with "no rebase in progress"
