## ADDED Requirements

### Requirement: Navigate Up (st up)
The `st up` command SHALL check out the child branch of the current branch. The current branch MUST be in the stack. If the current branch has exactly one child, that child SHALL be checked out. If the current branch has multiple children, the command SHALL error listing the children and suggesting `st switch`. If the current branch has no children (is at the tip), the command SHALL error indicating there is no branch above.

#### Scenario: Single child branch
- **WHEN** `st up` is run while on a branch with exactly one child
- **THEN** the child branch SHALL be checked out

#### Scenario: Multiple children
- **WHEN** `st up` is run while on a branch with multiple children
- **THEN** the command SHALL exit with an error listing the child branch names
- **AND** the error SHALL suggest using `st switch` to select

#### Scenario: No children (at tip)
- **WHEN** `st up` is run while on a branch with no children
- **THEN** the command SHALL exit with an error indicating already at the tip of the stack

#### Scenario: Not in stack
- **WHEN** `st up` is run while on a branch not in the stack
- **THEN** the command SHALL exit with an error indicating the branch is not in the stack

### Requirement: Navigate Down (st down)
The `st down` command SHALL check out the parent branch of the current branch. The current branch MUST be in the stack and MUST NOT be the root. If the current branch is the root, the command SHALL error indicating already at the bottom.

#### Scenario: Has parent
- **WHEN** `st down` is run while on a tracked branch that is not the root
- **THEN** the parent branch SHALL be checked out

#### Scenario: At root
- **WHEN** `st down` is run while on the root branch
- **THEN** the command SHALL exit with an error indicating already at the bottom of the stack

#### Scenario: Not in stack
- **WHEN** `st down` is run while on a branch not in the stack
- **THEN** the command SHALL exit with an error indicating the branch is not in the stack

### Requirement: Navigate to Top (st top)
The `st top` command SHALL check out the tip branch of the current lineage. Starting from the current branch, it SHALL follow single-child links until it reaches a branch with no children. If any branch along the path has multiple children, the command SHALL error listing the children at the fork point. If already at the tip, the command SHALL print a message indicating already at the top.

#### Scenario: Linear path to tip
- **WHEN** `st top` is run while on a branch with a single-child path to the tip
- **THEN** the tip branch SHALL be checked out

#### Scenario: Fork in path
- **WHEN** `st top` is run and a branch along the path has multiple children
- **THEN** the command SHALL exit with an error indicating a fork at that branch
- **AND** the error SHALL list the children

#### Scenario: Already at tip
- **WHEN** `st top` is run while already on the tip branch
- **THEN** the command SHALL print that already at the top of the stack

### Requirement: Navigate to Bottom (st bottom)
The `st bottom` command SHALL check out the lowest tracked branch in the current lineage (the first child of root). Starting from the current branch, it SHALL follow parent links until reaching a branch whose parent is the root. If the current branch is the root, the command SHALL check out the first child of root (erroring if multiple children, same as `st up`). If already at the bottom tracked branch, the command SHALL print a message.

#### Scenario: Navigate to bottom from mid-stack
- **WHEN** `st bottom` is run while on a mid-stack branch
- **THEN** the first branch above root in the current lineage SHALL be checked out

#### Scenario: Already at bottom
- **WHEN** `st bottom` is run while on the first branch above root
- **THEN** the command SHALL print that already at the bottom of the stack

#### Scenario: On root with single child
- **WHEN** `st bottom` is run while on the root branch with a single child
- **THEN** the child branch SHALL be checked out
