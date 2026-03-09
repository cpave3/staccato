## ADDED Requirements

### Requirement: Modify Current Branch (st modify)
The `st modify` command SHALL amend the HEAD commit of the current branch and restack all downstream branches. The current branch MUST be in the stack. The command SHALL create automatic backups before restacking. If `--all` is provided, all changes SHALL be staged before amending. If `--message` is provided, the commit message SHALL be updated.

#### Scenario: Amend with staged changes
- **WHEN** `st modify` is run with staged changes on a tracked branch
- **THEN** the HEAD commit SHALL be amended with the staged changes
- **AND** all downstream branches SHALL be restacked
- **AND** automatic backups SHALL be created before restacking
- **AND** backups SHALL be cleaned up on success

#### Scenario: Amend with --all flag
- **WHEN** `st modify --all` is run with unstaged changes
- **THEN** all changes SHALL be staged before amending
- **AND** the HEAD commit SHALL be amended
- **AND** downstream branches SHALL be restacked

#### Scenario: Update commit message
- **WHEN** `st modify --message "new message"` is run
- **THEN** the HEAD commit message SHALL be updated to the provided message
- **AND** downstream branches SHALL be restacked

#### Scenario: No downstream branches
- **WHEN** `st modify` is run on a tip branch (no children)
- **THEN** the HEAD commit SHALL be amended
- **AND** no restacking SHALL occur (nothing downstream)

#### Scenario: Restack conflict during modify
- **WHEN** `st modify` amends a commit and downstream restacking encounters a conflict
- **THEN** the command SHALL print the conflict location
- **AND** the command SHALL instruct the user to resolve and run `st continue`

#### Scenario: Not in stack
- **WHEN** `st modify` is run on a branch not in the stack
- **THEN** the command SHALL exit with an error indicating the branch is not in the stack

#### Scenario: No changes to amend
- **WHEN** `st modify` is run with no staged changes and no `--all` flag and no `--message` flag
- **THEN** the command SHALL exit with an error indicating nothing to modify
