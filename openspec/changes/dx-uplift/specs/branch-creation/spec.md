## MODIFIED Requirements

### Requirement: Fail if Branch Already Exists

The `st new` command SHALL fail if a git branch with the given name already exists. The error message SHALL suggest using `st attach` to add the existing branch to the stack.

#### Scenario: Duplicate branch name

- **WHEN** the user runs `st new feature-1` and a branch named `feature-1` already exists
- **THEN** the command SHALL return an error: "branch 'feature-1' already exists — use 'st attach feature-1' to add it to the stack"
- **AND** the graph SHALL NOT be modified

### Requirement: Fail if Branch Already Exists

The `st append` command SHALL fail if a git branch with the given name already exists. The error message SHALL suggest using `st attach` to add the existing branch to the stack.

#### Scenario: Duplicate branch name

- **WHEN** the user runs `st append feature-1` and a branch named `feature-1` already exists
- **THEN** the command SHALL return an error: "branch 'feature-1' already exists — use 'st attach feature-1' to add it to the stack"
- **AND** the graph SHALL NOT be modified
