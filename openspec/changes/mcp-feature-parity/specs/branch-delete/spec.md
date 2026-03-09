## ADDED Requirements

### Requirement: Delete Branch from Stack (st delete)
The `st delete <branch>` command SHALL remove a branch from the stack graph and delete the git branch. Children of the deleted branch SHALL be reparented to the deleted branch's parent. If the deleted branch is the currently checked-out branch, the parent SHALL be checked out first. The branch MUST be in the stack (not the root). Deleting the root SHALL be an error.

#### Scenario: Delete branch with no children
- **WHEN** `st delete feature-x` is run and `feature-x` has no children
- **THEN** `feature-x` SHALL be removed from the graph
- **AND** the git branch `feature-x` SHALL be deleted
- **AND** the graph SHALL be saved

#### Scenario: Delete branch with children (reparenting)
- **WHEN** `st delete feature-x` is run and `feature-x` has children `[child-a, child-b]`
- **THEN** `child-a` and `child-b` SHALL have their parent updated to `feature-x`'s parent
- **AND** `feature-x` SHALL be removed from the graph
- **AND** the git branch `feature-x` SHALL be deleted
- **AND** the graph SHALL be saved

#### Scenario: Delete current branch
- **WHEN** `st delete feature-x` is run while `feature-x` is checked out
- **THEN** the parent branch SHALL be checked out first
- **AND** then `feature-x` SHALL be deleted from graph and git

#### Scenario: Delete root branch
- **WHEN** `st delete main` is run where `main` is the root
- **THEN** the command SHALL exit with an error indicating the root cannot be deleted

#### Scenario: Delete branch not in stack
- **WHEN** `st delete unknown-branch` is run and the branch is not in the stack
- **THEN** the command SHALL exit with an error indicating the branch is not in the stack

#### Scenario: Force flag for unpushed branches
- **WHEN** `st delete feature-x` is run and `feature-x` has commits not on the remote
- **THEN** the command SHALL warn about unpushed commits
- **AND** require `--force` flag to proceed
