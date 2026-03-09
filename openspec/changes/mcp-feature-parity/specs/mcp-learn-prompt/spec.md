## ADDED Requirements

### Requirement: Learn Staccato Prompt
The MCP server SHALL register a prompt named `"learn-staccato"` that provides a comprehensive guide to stacking workflows with Staccato. The prompt SHALL accept no arguments. The content SHALL cover: what stacking is, the `st` command reference, common workflows (create stack, modify mid-stack, sync, resolve conflicts), and how MCP tools map to CLI commands.

#### Scenario: Request learn prompt
- **WHEN** the `learn-staccato` prompt is requested
- **THEN** the server SHALL return the rendered markdown content from the embedded template

### Requirement: Learn Staccato Resource
The server SHALL register the learn-staccato content as an MCP resource at URI `"staccato://prompts/learn-staccato"` with MIME type `"text/markdown"`.

#### Scenario: Read learn resource
- **WHEN** a client reads the resource at `"staccato://prompts/learn-staccato"`
- **THEN** the server SHALL return the raw markdown content

### Requirement: Learn Prompt Content Coverage
The learn-staccato prompt content SHALL include the following sections:
1. **What is Stacking** — explanation of stacked PRs and their benefits
2. **Getting Started** — initializing a stack, creating the first branch
3. **Core Commands** — reference for all `st` subcommands with brief descriptions
4. **Common Workflows** — step-by-step guides for: creating a stack, modifying mid-stack, syncing with remote, resolving conflicts, navigating the stack
5. **MCP Tools** — mapping between typed MCP tools and their CLI equivalents, and guidance on when to use `st_run` vs typed tools

#### Scenario: Content includes command reference
- **WHEN** the learn prompt content is rendered
- **THEN** it SHALL list all `st` subcommands with descriptions
- **AND** it SHALL describe the stacking workflow from creation through merge
