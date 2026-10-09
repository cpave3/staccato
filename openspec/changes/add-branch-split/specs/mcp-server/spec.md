## ADDED Requirements

### Requirement: Split Describe Tool (st_split_describe)
The MCP server SHALL expose an `st_split_describe` tool that returns the branch inventory defined by the `branch-split` capability as JSON text. It SHALL accept an optional `branch` string and SHALL reload the stack graph from storage before reading it.

#### Scenario: Describe a named branch
- **WHEN** `st_split_describe` is called with `branch: "feature"`
- **THEN** the tool SHALL return the JSON description of `feature`

#### Scenario: Graph changed since the server started
- **WHEN** a branch was added to the stack by another process after the server loaded its graph
- **THEN** `st_split_describe` SHALL still find that branch

#### Scenario: Tool annotation
- **WHEN** `st_split_describe` is registered
- **THEN** the tool SHALL have a read-only, non-destructive, idempotent annotation

### Requirement: Split Apply Tool (st_split_apply)
The MCP server SHALL expose an `st_split_apply` tool that applies a split plan using the same implementation as `st split apply`. It SHALL accept a required `plan` (the plan as an object, or as a JSON string) and an optional `dry_run` boolean, and SHALL return the result as JSON text. The plan SHALL be parsed strictly. The plan SHALL name its `branch`. The tool SHALL NOT offer a way to run a verification command. Relative override content files SHALL be resolved against the repository root.

#### Scenario: Apply a plan
- **WHEN** `st_split_apply` is called with a valid plan
- **THEN** the split SHALL be applied
- **AND** the result SHALL include `"applied": true`
- **AND** the server's in-memory graph SHALL reflect the split

#### Scenario: Plan as a JSON string
- **WHEN** `plan` is a string containing the plan as JSON
- **THEN** the tool SHALL apply it as if it were an object

#### Scenario: Dry run
- **WHEN** `st_split_apply` is called with `dry_run: true`
- **THEN** the result SHALL include `"dry_run": true`
- **AND** no branch SHALL move

#### Scenario: Misspelt key
- **WHEN** the plan contains an unknown field
- **THEN** the tool SHALL return an error result naming the field

#### Scenario: Branch not named
- **WHEN** the plan has no `branch`
- **THEN** the tool SHALL return an error result stating which worktree the server runs in

#### Scenario: Rejected plan
- **WHEN** the plan is rejected
- **THEN** the tool SHALL return an error result with the reason
- **AND** nothing SHALL be changed

#### Scenario: Tool annotation
- **WHEN** `st_split_apply` is registered
- **THEN** the tool SHALL have a destructive, non-read-only annotation

### Requirement: Run Tool Refuses Exec
The `st_run` tool SHALL reject a command that contains the `--exec` flag, in either the `--exec <cmd>` or the `--exec=<cmd>` form.

#### Scenario: Command with --exec
- **WHEN** `st_run` is called with `command: "split apply plan.json --exec make"`
- **THEN** the tool SHALL return an error result
- **AND** SHALL NOT run the command

## MODIFIED Requirements

### Requirement: Split Monolithic PR Prompt
The server SHALL register a prompt named `"split-monolithic-pr"` that guides an agent through splitting a large branch into stacked branches using `st_split_describe` and `st_split_apply`: reading the inventory, choosing boundaries, writing a plan, running a dry run, verifying each slice, and applying. The prompt SHALL NOT instruct the agent to reset, stage, cherry-pick or rebase by hand, and SHALL NOT refer to tools the server does not expose. The prompt accepts optional `base_branch` (default `"main"`) and `source_branch` (default `"current branch"`) arguments. Template placeholders `{{base_branch}}` and `{{source_branch}}` SHALL be replaced with the provided argument values.

#### Scenario: Prompt with default arguments
- **WHEN** the `split-monolithic-pr` prompt is requested without arguments
- **THEN** the rendered prompt SHALL use `"main"` as the base branch and `"current branch"` as the source branch

#### Scenario: Prompt with custom arguments
- **WHEN** the `split-monolithic-pr` prompt is requested with `base_branch: "develop"` and `source_branch: "feature/big-refactor"`
- **THEN** the rendered prompt SHALL substitute the provided values into the template

#### Scenario: Prompt names only tools that exist
- **WHEN** the prompt is rendered
- **THEN** every `st_` tool it names SHALL be registered on the server
