## MODIFIED Requirements

### Requirement: Create Branch from Root
The `st new <branch>` command SHALL create a new git branch from the current stack root branch and add it to the stack graph with the root as its parent. The branch SHALL always be created from the root branch's HEAD commit, regardless of which branch is currently checked out.

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

#### Scenario: New branch from non-root branch
- **WHEN** the user is currently on branch `foo` which has commits not on the root branch
- **AND** the user runs `st new bar`
- **THEN** branch `bar` SHALL be created from the root branch's HEAD commit
- **AND** `bar` SHALL NOT contain any commits from `foo` that are not on the root branch
- **AND** the working tree SHALL be checked out to `bar`
