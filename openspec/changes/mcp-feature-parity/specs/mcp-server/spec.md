## MODIFIED Requirements

### Requirement: PR Preparation (st_pr)
The `st_pr` tool SHALL push the current branch (if not already on the remote) and return JSON with `head`, `base`, `remote_url`, and `pushed` fields. The current branch MUST be in the stack. The tool accepts optional `stack` (boolean, default false) parameter. When `stack` is true, the tool SHALL push and return PR info for all branches in the current lineage from root to current, not just the current branch.

#### Scenario: Push and return PR info
- **WHEN** `st_pr` is called while on a tracked branch that has not been pushed
- **THEN** the branch SHALL be pushed to the remote
- **AND** the response SHALL include `"pushed": true`, the head branch, the base (parent) branch, and the remote URL

#### Scenario: Branch already pushed
- **WHEN** `st_pr` is called while on a tracked branch that already exists on the remote
- **THEN** the tool SHALL NOT push again
- **AND** the response SHALL include `"pushed": false`

#### Scenario: Branch not in stack
- **WHEN** `st_pr` is called while on a branch not in the stack
- **THEN** the tool SHALL return an error indicating the branch is not in the stack

#### Scenario: Stack-wide PR info
- **WHEN** `st_pr` is called with `stack: true`
- **THEN** the tool SHALL return a JSON array of PR info objects for each branch in the lineage
- **AND** each entry SHALL contain `head`, `base`, `remote_url`, and `pushed` fields
- **AND** branches not yet pushed SHALL be pushed
