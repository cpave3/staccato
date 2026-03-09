## ADDED Requirements

### Requirement: Detached HEAD Guard

Commands that operate on the current branch SHALL verify that HEAD is not detached before proceeding. If HEAD is detached, the command SHALL return a clear error.

#### Scenario: Command run with detached HEAD

- **WHEN** the user runs `st new`, `st append`, `st insert`, `st restack`, `st continue`, `st attach`, `st sync`, `st log`, `st detach`, or `st status` while HEAD is detached
- **THEN** the command SHALL return an error: "HEAD is detached — check out a branch first"
- **AND** no graph modifications SHALL occur

#### Scenario: Command run on valid branch

- **WHEN** the user runs any command while on a valid branch
- **THEN** the detached HEAD check SHALL pass silently
- **AND** the command SHALL proceed normally

### Requirement: Dirty Working Tree Warning

Commands that trigger rebase operations SHALL warn the user if the working tree has uncommitted changes. The warning SHALL NOT block the operation — it is informational only.

#### Scenario: Dirty tree before restack

- **WHEN** the user runs `st restack` and the working tree has uncommitted changes
- **THEN** a warning SHALL be printed: "Warning: you have uncommitted changes — consider committing or stashing first"
- **AND** the restack SHALL proceed

#### Scenario: Dirty tree before insert

- **WHEN** the user runs `st insert <branch>` and the working tree has uncommitted changes
- **THEN** a warning SHALL be printed before the operation begins

#### Scenario: Dirty tree before attach with rebase

- **WHEN** the user runs `st attach --parent <new-parent>` on an already-tracked branch (relocation triggers rebase)
- **AND** the working tree has uncommitted changes
- **THEN** a warning SHALL be printed before the rebase begins

#### Scenario: Clean tree shows no warning

- **WHEN** the user runs any rebase-triggering command with a clean working tree
- **THEN** no dirty tree warning SHALL be printed
