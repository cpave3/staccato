## ADDED Requirements

### Requirement: Universal Command Wrapper (st_run)
The MCP server SHALL register a tool named `st_run` that executes any `st` subcommand as a subprocess, capturing stdout and stderr. The tool SHALL accept a required `command` string parameter containing the subcommand and its arguments (e.g., `"log"`, `"up"`, `"restack --to-current"`). The tool SHALL be annotated as mutating.

#### Scenario: Run a read-only command
- **WHEN** `st_run` is called with `command: "log"`
- **THEN** the tool SHALL execute `st log` as a subprocess
- **AND** the tool SHALL return the combined stdout output as the tool result text

#### Scenario: Run a command with arguments
- **WHEN** `st_run` is called with `command: "restack --to-current"`
- **THEN** the tool SHALL execute `st restack --to-current` as a subprocess
- **AND** the tool SHALL return the output as the tool result text

#### Scenario: Command fails
- **WHEN** `st_run` is called and the subprocess exits with a non-zero status
- **THEN** the tool SHALL return an MCP error result containing the stderr output

#### Scenario: Recursive MCP invocation blocked
- **WHEN** `st_run` is called with `command: "mcp"`
- **THEN** the tool SHALL return an error result: `"cannot run 'st mcp' recursively"`
- **AND** the subprocess SHALL NOT be started

#### Scenario: Empty command
- **WHEN** `st_run` is called with an empty `command` string
- **THEN** the tool SHALL return an error result indicating the command is required

### Requirement: st_run Binary Resolution
The `st_run` tool SHALL resolve the `st` binary path using `os.Executable()` to ensure it runs the same binary that is serving MCP. The subprocess SHALL inherit the current working directory.

#### Scenario: Binary path resolution
- **WHEN** `st_run` executes a command
- **THEN** it SHALL use the path returned by `os.Executable()` as the binary
- **AND** the subprocess working directory SHALL be the repository root from `StaccatoContext`
