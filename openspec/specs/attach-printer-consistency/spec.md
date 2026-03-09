## Requirements

### Requirement: Attach command uses Printer for all output

All user-facing output in `cmd/st/attach.go` SHALL use `output.Printer` methods instead of raw `fmt.Printf` calls.

#### Scenario: Success messages use printer.Success

- **WHEN** a branch is successfully attached, set as root, or relocated
- **THEN** the output SHALL be produced via `printer.Success()` (not `fmt.Printf` with hardcoded `✔`)

#### Scenario: Informational messages use printer.Println

- **WHEN** a non-success informational message is printed (e.g., "Parent is not yet in the stack")
- **THEN** the output SHALL be produced via `printer.Println()` (not `fmt.Printf`)

### Requirement: Printer is threaded through all attach helpers

All attach helper functions (`doAttachRecursively`, `attachInteractive`, `attachRecursively`, `attachWithParent`) SHALL accept a `*output.Printer` parameter and use it for output.

#### Scenario: attachCmd passes printer to helpers

- **WHEN** the `attach` command is executed
- **THEN** the `Printer` from `getContext()` SHALL be passed to all helper functions

### Requirement: Output content is unchanged

The visible output (icons, messages, formatting) SHALL remain identical after the refactor.

#### Scenario: Same icons and messages

- **WHEN** a branch is attached with `--parent` flag
- **THEN** the output messages SHALL contain the same text and icons as before the refactor
