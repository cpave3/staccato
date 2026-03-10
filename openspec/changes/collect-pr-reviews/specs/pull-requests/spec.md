## ADDED Requirements

### Requirement: Reviews command
The `st reviews` command SHALL collect PR review feedback from GitHub for branches in the stack and output a unified markdown document.

#### Scenario: Running st reviews with default scope
- **WHEN** the user runs `st reviews` in a repository with a stack
- **THEN** the command SHALL collect reviews from all stack branches that have open PRs
- **AND** SHALL output the unified feedback markdown to stdout

#### Scenario: Running st reviews with --current flag
- **WHEN** the user runs `st reviews --current`
- **THEN** the command SHALL collect reviews only from the current branch's PR

#### Scenario: Running st reviews with --to-current flag
- **WHEN** the user runs `st reviews --to-current`
- **THEN** the command SHALL collect reviews from the current branch and all ancestor branches' PRs

#### Scenario: Running st reviews with --out flag
- **WHEN** the user runs `st reviews --out path/to/output.md`
- **THEN** the command SHALL write the feedback document to the specified path
- **AND** SHALL print a confirmation message to stderr

#### Scenario: No PRs found for branches in scope
- **WHEN** the user runs `st reviews` and no branches in scope have open PRs
- **THEN** the command SHALL print a message indicating no PRs were found

#### Scenario: Forge not available
- **WHEN** the user runs `st reviews` and forge detection fails (e.g., no GitHub remote)
- **THEN** the command SHALL return an error indicating the forge is not available
