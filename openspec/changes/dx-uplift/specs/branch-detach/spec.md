## ADDED Requirements

### Requirement: Detach Branch from Stack Graph

The `st detach <branch>` command SHALL remove a branch from the stack graph while keeping the git branch intact. If no branch name is provided, the current branch SHALL be used. Children of the detached branch SHALL be reparented to the detached branch's parent.

#### Scenario: Detach a branch with no children

- **WHEN** the user runs `st detach feature-1` and `feature-1` is in the graph with parent `main`
- **THEN** `feature-1` SHALL be removed from the graph
- **AND** the git branch `feature-1` SHALL still exist
- **AND** the graph SHALL be persisted
- **AND** a message SHALL be printed: "Detached 'feature-1' from stack"

#### Scenario: Detach a branch with children

- **WHEN** the user runs `st detach feature-1` and `feature-1` has children `feature-2` and `feature-3`
- **AND** `feature-1`'s parent is `main`
- **THEN** `feature-1` SHALL be removed from the graph
- **AND** `feature-2` and `feature-3` SHALL be reparented to `main`
- **AND** a message SHALL be printed indicating children were reparented
- **AND** a message SHALL suggest running `st restack` to update the reparented branches

#### Scenario: Detach current branch (no argument)

- **WHEN** the user runs `st detach` without arguments while on branch `feature-1`
- **THEN** `feature-1` SHALL be detached from the graph (same behavior as explicit argument)

#### Scenario: Detach branch not in graph

- **WHEN** the user runs `st detach unknown-branch` and `unknown-branch` is not in the graph
- **THEN** the command SHALL return an error: "branch 'unknown-branch' is not in the stack"

#### Scenario: Detach the root branch

- **WHEN** the user runs `st detach main` and `main` is the root branch
- **THEN** the command SHALL return an error: "cannot detach the root branch 'main'"

### Requirement: Exactly Zero or One Arguments

The `st detach` command SHALL accept zero or one positional arguments.

#### Scenario: No argument uses current branch

- **WHEN** the user runs `st detach` with no arguments
- **THEN** the current branch SHALL be used as the target

#### Scenario: Too many arguments

- **WHEN** the user runs `st detach branch1 branch2`
- **THEN** the command SHALL fail with a usage error
