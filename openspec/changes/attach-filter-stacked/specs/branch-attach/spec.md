## MODIFIED Requirements

### Requirement: Interactive TUI Attachment (Default Mode)

When neither `--auto` nor `--parent` is specified, the command SHALL launch an interactive TUI that displays all local git branches (excluding the branch being attached) as parent candidates. The user SHALL be able to navigate, search, and select a parent branch.

#### Scenario: TUI displays all branches as candidates
- **WHEN** the TUI launches for attaching branch `feature-x`
- **THEN** all local git branches except `feature-x` SHALL be listed as candidates
- **AND** the current checked-out branch SHALL be marked with a filled circle indicator

#### Scenario: User selects a parent via Enter
- **WHEN** the user highlights a branch in the TUI and presses Enter
- **THEN** the highlighted branch SHALL be selected as the parent
- **AND** the target branch SHALL be attached as a child of the selected parent in the graph
- **AND** the graph SHALL be persisted

#### Scenario: User quits without selecting
- **WHEN** the user presses `q` or `Esc` in the TUI (outside search mode)
- **THEN** the TUI SHALL exit without attaching anything

#### Scenario: TUI search mode
- **WHEN** the user presses `/` in the TUI
- **THEN** the TUI SHALL enter search mode
- **AND** typed characters SHALL filter the displayed branches by substring match (case-insensitive)
- **AND** matching branches SHALL be visually highlighted
- **AND** pressing Enter SHALL exit search mode and move the cursor to the first match
- **AND** pressing Esc SHALL exit search mode and clear the search query
- **AND** pressing Backspace SHALL remove the last character from the search query

#### Scenario: No candidates available
- **WHEN** there are no other local branches besides the branch being attached
- **THEN** the command SHALL return an error: "no existing branches to use as parent for '<branch>'"

#### Scenario: TUI relaunches for tracked branches
- **WHEN** the user runs `st attach` on a branch that is already tracked in the graph
- **THEN** the TUI SHALL still launch (allowing the user to modify the stack interactively)

#### Scenario: Recursive TUI hides already-stacked branches
- **WHEN** the TUI launches during recursive attachment (attaching a parent that is not yet tracked)
- **THEN** branches already tracked in the graph SHALL be excluded from the candidate list
- **AND** the graph root branch SHALL still be shown as a candidate (it is a valid parent)
- **AND** the branch being attached SHALL still be excluded from the candidate list

#### Scenario: Top-level TUI shows all branches including stacked
- **WHEN** the user runs `st attach` directly (not during recursive attachment)
- **THEN** all local git branches (except the one being attached) SHALL be shown as candidates
- **AND** branches already in the graph SHALL NOT be filtered out (to support relocation)
