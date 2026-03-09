# Branch Creation

This specification covers the three branch creation commands in st: `new`, `append`, and `insert`.

---

## st new

### Requirement: Create Branch from Root
The `st new <branch>` command SHALL create a new git branch from the current stack root branch and add it to the stack graph with the root as its parent.

#### Scenario: Basic branch creation
- **WHEN** the user runs `st new feature-1`
- **THEN** a git branch named `feature-1` SHALL be created from the root branch
- **AND** the working tree SHALL be checked out to `feature-1`
- **AND** the branch SHALL be added to the graph with the root as its parent
- **AND** the branch's BaseSHA SHALL be set to the root branch's current commit SHA
- **AND** the branch's HeadSHA SHALL be set to the new branch's current commit SHA
- **AND** the graph SHALL be persisted to storage

#### Scenario: Multiple branches from root
- **WHEN** the user runs `st new a` followed by `st new b` (after checking out root between them)
- **THEN** both `a` and `b` SHALL exist in the graph with the root as their parent

### Requirement: Exactly One Argument Required
The `st new` command SHALL require exactly one positional argument specifying the branch name.

#### Scenario: No argument provided
- **WHEN** the user runs `st new` with no arguments
- **THEN** the command SHALL fail with a usage error

#### Scenario: Too many arguments provided
- **WHEN** the user runs `st new branch1 branch2`
- **THEN** the command SHALL fail with a usage error

### Requirement: Fail if Branch Already Exists
The `st new` command SHALL fail if a git branch with the given name already exists.

#### Scenario: Duplicate branch name
- **WHEN** the user runs `st new feature-1` and a branch named `feature-1` already exists
- **THEN** the command SHALL return an error indicating branch creation failed
- **AND** the graph SHALL NOT be modified

### Requirement: Staleness Check
The `st new` command SHALL check for graph staleness before proceeding.

#### Scenario: Staleness check runs before creation
- **WHEN** the user runs `st new <branch>`
- **THEN** the command SHALL invoke the staleness check against the current graph and git state before creating the branch

---

## st append

### Requirement: Create Child Branch from Current Branch
The `st append <branch>` command SHALL create a new git branch from the current branch and add it as a child of the current branch in the stack graph.

#### Scenario: Append child to tracked branch
- **WHEN** the current branch is `f1` (which is tracked in the graph)
- **AND** the user runs `st append f2`
- **THEN** a git branch named `f2` SHALL be created from `f1`
- **AND** the working tree SHALL be checked out to `f2`
- **AND** `f2` SHALL be added to the graph with `f1` as its parent
- **AND** the branch's BaseSHA SHALL be set to `f1`'s current commit SHA
- **AND** the branch's HeadSHA SHALL be set to the new branch's current commit SHA
- **AND** the graph SHALL be persisted to storage

### Requirement: Append from Root Branch Permitted
The `st append` command SHALL allow appending from the root branch without requiring explicit attachment.

#### Scenario: Append from root
- **WHEN** the current branch is the root branch (e.g., `main`)
- **AND** the user runs `st append child`
- **THEN** the branch `child` SHALL be created with the root as its parent
- **AND** no "not in the stack" error SHALL occur

### Requirement: Current Branch Must Be in Stack
The `st append` command SHALL fail if the current branch is neither the root nor a tracked branch in the graph.

#### Scenario: Append from untracked branch
- **WHEN** the current branch is `untracked` which is not in the graph and is not the root
- **AND** the user runs `st append child`
- **THEN** the command SHALL return an error containing "not in the stack"
- **AND** the error message SHALL suggest running `st attach` first

### Requirement: Exactly One Argument Required
The `st append` command SHALL require exactly one positional argument specifying the branch name.

#### Scenario: No argument provided
- **WHEN** the user runs `st append` with no arguments
- **THEN** the command SHALL fail with a usage error

#### Scenario: Too many arguments provided
- **WHEN** the user runs `st append branch1 branch2`
- **THEN** the command SHALL fail with a usage error

### Requirement: Fail if Branch Already Exists
The `st append` command SHALL fail if a git branch with the given name already exists.

#### Scenario: Duplicate branch name
- **WHEN** the user runs `st append feature-1` and a branch named `feature-1` already exists
- **THEN** the command SHALL return an error indicating branch creation failed
- **AND** the graph SHALL NOT be modified

### Requirement: Staleness Check
The `st append` command SHALL check for graph staleness before proceeding.

#### Scenario: Staleness check runs before creation
- **WHEN** the user runs `st append <branch>`
- **THEN** the command SHALL invoke the staleness check against the current graph and git state before creating the branch

---

## st insert

### Requirement: Insert Branch Before Current Branch
The `st insert <branch>` command SHALL create a new branch that is inserted between the current branch and its parent in the stack graph. The current branch and all its downstream descendants SHALL be reparented and restacked.

#### Scenario: Insert before a tracked branch
- **WHEN** the current branch is `f1` with parent `main` (root)
- **AND** the user runs `st insert pre-f`
- **THEN** a git branch named `pre-f` SHALL be created from `main` (the old parent of `f1`)
- **AND** `pre-f` SHALL be added to the graph with `main` as its parent
- **AND** `f1` SHALL be reparented to have `pre-f` as its parent
- **AND** all downstream branches of `f1` SHALL be restacked
- **AND** the working tree SHALL be checked out to `pre-f`
- **AND** the graph SHALL be persisted to storage

### Requirement: Current Branch Must Be in Stack
The `st insert` command SHALL fail if the current branch is not tracked in the stack graph.

#### Scenario: Insert from untracked branch
- **WHEN** the current branch is `untracked` which is not in the graph
- **AND** the user runs `st insert x`
- **THEN** the command SHALL return an error containing "not in the stack"

### Requirement: Backup Before Restack
The `st insert` command SHALL create automatic backups of all affected branches before performing the restack operation.

#### Scenario: Backups created for affected branches
- **WHEN** the user runs `st insert pre-f` while on branch `f1`
- **AND** `f1` has downstream branches `f2` and `f3`
- **THEN** automatic backups SHALL be created for `f1`, `f2`, and `f3` before restacking
- **AND** the backup refs SHALL use the `backup/<branch>/<timestamp>` naming convention

### Requirement: Cleanup Backups on Success
The `st insert` command SHALL remove automatic backups for all affected branches after a successful restack.

#### Scenario: Backups cleaned up after success
- **WHEN** `st insert` completes successfully with no conflicts
- **THEN** all automatic backups created during the operation SHALL be deleted

### Requirement: Restore Backups on Restack Failure
The `st insert` command SHALL restore all affected branches from backups if the restack operation fails (for non-conflict errors).

#### Scenario: Restack fails and backups are restored
- **WHEN** the restack triggered by `st insert` fails due to a non-conflict error
- **THEN** all affected branches SHALL be restored from their backups
- **AND** an error SHALL be returned to the user

### Requirement: Report Conflicts on Restack Conflict
The `st insert` command SHALL report the conflicting branch and return an error if a rebase conflict occurs during the restack.

#### Scenario: Restack encounters a conflict
- **WHEN** the restack triggered by `st insert` encounters a merge conflict
- **THEN** the command SHALL report which branch has the conflict
- **AND** the command SHALL return an error indicating "conflict during restack"

### Requirement: Reparent Only the Current Branch
The `st insert` command SHALL reparent only the current branch to point to the newly inserted branch. The current branch's children SHALL retain their existing parent (the current branch).

#### Scenario: Graph structure after insert
- **WHEN** the graph is `root -> f1 -> f2`
- **AND** the user runs `st insert pre-f` while on `f1`
- **THEN** the graph SHALL become `root -> pre-f -> f1 -> f2`
- **AND** `pre-f`'s parent SHALL be `root`
- **AND** `f1`'s parent SHALL be `pre-f`
- **AND** `f2`'s parent SHALL remain `f1`

### Requirement: Exactly One Argument Required
The `st insert` command SHALL require exactly one positional argument specifying the branch name.

#### Scenario: No argument provided
- **WHEN** the user runs `st insert` with no arguments
- **THEN** the command SHALL fail with a usage error

#### Scenario: Too many arguments provided
- **WHEN** the user runs `st insert branch1 branch2`
- **THEN** the command SHALL fail with a usage error

### Requirement: Fail if Branch Already Exists
The `st insert` command SHALL fail if a git branch with the given name already exists.

#### Scenario: Duplicate branch name
- **WHEN** the user runs `st insert feature-1` and a branch named `feature-1` already exists
- **THEN** the command SHALL return an error indicating branch creation failed
- **AND** backups SHALL have been created but the restack SHALL NOT proceed

### Requirement: Staleness Check
The `st insert` command SHALL check for graph staleness before proceeding.

#### Scenario: Staleness check runs before insertion
- **WHEN** the user runs `st insert <branch>`
- **THEN** the command SHALL invoke the staleness check against the current graph and git state before performing the insert

### Requirement: Graph Saved Before Restack
The `st insert` command SHALL persist the graph (with the new branch and reparented current branch) before beginning the restack operation.

#### Scenario: Graph is saved prior to restack
- **WHEN** the user runs `st insert pre-f`
- **THEN** the graph SHALL be saved with the new branch and updated parent pointers before the restack begins
- **AND** this ensures the graph is consistent even if the restack is interrupted
