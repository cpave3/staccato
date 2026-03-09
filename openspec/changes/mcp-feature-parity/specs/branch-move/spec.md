## ADDED Requirements

### Requirement: Move Branch to New Parent (st move)
The `st move --onto <target>` command SHALL reparent the current branch onto a different parent in the stack. The target MUST be in the stack (either root or a tracked branch). The current branch MUST be in the stack. After reparenting, the current branch and its descendants SHALL be restacked. Automatic backups SHALL be created before restacking.

#### Scenario: Move branch to new parent
- **WHEN** `st move --onto new-parent` is run while on `feature-x`
- **THEN** `feature-x`'s parent SHALL be updated to `new-parent` in the graph
- **AND** `feature-x` SHALL be rebased onto `new-parent`
- **AND** descendants of `feature-x` SHALL be restacked
- **AND** the graph SHALL be saved

#### Scenario: Move with restack conflict
- **WHEN** `st move --onto new-parent` triggers a rebase conflict
- **THEN** the command SHALL print the conflict location
- **AND** instruct the user to resolve and run `st continue`

#### Scenario: Target not in stack
- **WHEN** `st move --onto unknown` is run and `unknown` is not in the stack
- **THEN** the command SHALL exit with an error indicating the target is not in the stack

#### Scenario: Current branch not in stack
- **WHEN** `st move --onto main` is run while on an untracked branch
- **THEN** the command SHALL exit with an error

#### Scenario: Move onto self
- **WHEN** `st move --onto feature-x` is run while on `feature-x`
- **THEN** the command SHALL exit with an error indicating cannot move onto self

#### Scenario: Move onto own descendant
- **WHEN** `st move --onto child-of-feature-x` is run while on `feature-x`
- **THEN** the command SHALL exit with an error indicating cannot move onto a descendant (would create a cycle)
