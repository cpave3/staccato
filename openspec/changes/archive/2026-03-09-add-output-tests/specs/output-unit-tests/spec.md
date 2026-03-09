## ADDED Requirements

### Requirement: Core formatting methods produce correct output

Each `Printer` method (`Success`, `Warning`, `Error`, `Info`) SHALL prepend the corresponding icon and format the message with a trailing newline.

#### Scenario: Success prints with success icon

- **WHEN** `printer.Success("branch '%s' created", "feat")` is called
- **THEN** stdout SHALL contain `✔ branch 'feat' created\n`

#### Scenario: Warning prints with warning icon

- **WHEN** `printer.Warning("conflicts detected")` is called
- **THEN** stdout SHALL contain `⚠ conflicts detected\n`

#### Scenario: Error prints with error icon

- **WHEN** `printer.Error("failed to rebase")` is called
- **THEN** stdout SHALL contain `✘ failed to rebase\n`

### Requirement: Info respects verbose flag

The `Info` method SHALL only produce output when the printer is in verbose mode.

#### Scenario: Info prints when verbose is true

- **WHEN** a `Printer` is created with `verbose=true`
- **AND** `printer.Info("fetching...")` is called
- **THEN** stdout SHALL contain `ℹ fetching...\n`

#### Scenario: Info suppresses output when verbose is false

- **WHEN** a `Printer` is created with `verbose=false`
- **AND** `printer.Info("fetching...")` is called
- **THEN** stdout SHALL be empty

### Requirement: Print and Println produce undecorated output

`Print` SHALL write the formatted string without icon or newline. `Println` SHALL write the formatted string with a trailing newline but no icon.

#### Scenario: Print writes raw format

- **WHEN** `printer.Print("hello %s", "world")` is called
- **THEN** stdout SHALL contain exactly `hello world` with no trailing newline

#### Scenario: Println writes with newline

- **WHEN** `printer.Println("count: %d", 42)` is called
- **THEN** stdout SHALL contain `count: 42\n`

### Requirement: StackLog renders tree hierarchy

`StackLog` SHALL render the graph as an indented tree with `●` for the current branch and `○` for other branches.

#### Scenario: Nested stack renders with indentation

- **WHEN** `StackLog` is called with a graph containing root → A → B and current branch is A
- **THEN** stdout SHALL show root with `○`, A with `●` indented one level, B with `○` indented two levels

### Requirement: formatPRStatus covers all PR states

The `formatPRStatus` function SHALL format PR status info with the correct icon and label for each state.

#### Scenario: Merged PR

- **WHEN** PR state is `MERGED` with number 42
- **THEN** output SHALL be `#42 ✔ Merged`

#### Scenario: Open approved PR with failing CI

- **WHEN** PR state is `OPEN`, review status is `APPROVED`, check status is `fail`, number is 10
- **THEN** output SHALL be `#10 ✔ Approved | CI ✘`

#### Scenario: Open draft PR

- **WHEN** PR state is `OPEN`, draft is true, number is 5
- **THEN** output SHALL be `#5 Draft`
