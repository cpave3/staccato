# PRD: MCP Server for Staccato

## Overview

Add a stdio-based MCP (Model Context Protocol) server to Staccato, enabling LLMs to programmatically manage git stacks. The first prompt will teach LLMs how to split monolithic commits/PRs into logically segmented stacked PRs.

## Problem Statement

Developers and AI coding agents often create large, monolithic PRs that are difficult to review. Staccato already provides excellent tooling for managing stacked branches, but there's no programmatic interface for LLMs to use these capabilities. By exposing Staccato via MCP, coding agents can automatically decompose large changes into reviewable stacks.

## Goals

1. Expose Staccato operations as MCP tools for LLM consumption
2. Provide a comprehensive prompt that teaches LLMs the "split monolithic PR" workflow
3. Enable seamless integration with Claude Desktop, Cursor, and other MCP-compatible clients

---

## Implementation Plan

### Phase 1: Dependencies

**Dependencies to add:**
```go
// go.mod
require github.com/mark3labs/mcp-go v0.27.0
```

### Phase 2: MCP Server Infrastructure

**Files to create:**
- `pkg/mcp/server.go` - Server setup, capabilities, stdio transport
- `pkg/mcp/tools.go` - Tool definitions (schemas)
- `pkg/mcp/prompts.go` - Prompt definitions
- `pkg/mcp/types.go` - Shared response types
- `cmd/st/mcp.go` - CLI subcommand

**Server capabilities:**
```go
capabilities := mcp.ServerCapabilities{
    Tools:   &mcp.ToolCapabilities{},
    Prompts: &mcp.PromptCapabilities{ListChanged: false},
}
```

**Startup behavior:**
- Server will fail fast with clear error if not run inside a git repository
- Validates `.git` directory exists before starting stdio transport

### Phase 3: Implement MCP Tools

#### Stack Information Tools (Read-Only)

| Tool | Description | Parameters |
|------|-------------|------------|
| `st_log` | Display stack hierarchy as JSON tree | none |
| `st_status` | Show stack with PR status annotations | none |
| `st_current` | Get current branch and stack position | none |

#### Branch Creation Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `st_new` | Create branch from root | `branch_name` (required) |
| `st_append` | Create child of current branch | `branch_name` (required) |
| `st_insert` | Insert branch before current | `branch_name` (required) |

#### Stack Management Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `st_attach` | Attach existing branch to stack | `branch_name`, `parent` (required) |
| `st_restack` | Rebase lineage onto parents | `to_current` (optional) |
| `st_continue` | Resume after conflict resolution | none |

#### Sync & PR Tools

| Tool | Description | Parameters |
|------|-------------|------------|
| `st_sync` | Fetch, detect merged, restack, push | `dry_run`, `down_only` (optional) |
| `st_pr` | Create PR targeting parent branch | none |

#### Git Helper Tools (for split workflow)

| Tool | Description | Parameters |
|------|-------------|------------|
| `st_git_log` | Get commit log for analysis | `range`, `limit`, `stat` |
| `st_git_diff_stat` | Get diff statistics | `ref` (required) |
| `st_git_cherry_pick` | Cherry-pick commits | `commits` (required) |
| `st_git_checkout` | Switch branches | `branch` (required) |
| `st_git_reset` | Reset HEAD (soft/mixed/hard) | `ref` (required), `mode` (soft/mixed/hard) |
| `st_git_add` | Stage files or hunks | `paths` (required), `patch` (optional bool) |
| `st_git_commit` | Create commit | `message` (required) |
| `st_git_status` | Show working tree status | none |
| `st_git_diff` | Show unstaged/staged changes | `staged` (optional bool), `paths` (optional) |

### Phase 4: Implement the Split-Monolithic-PR Prompt

**File:** `pkg/mcp/prompts.go` (loads from `prd/prompts/split-monolithic-pr.md`)

The prompt guides LLMs through:

1. **Analyze** - Examine commits and file changes
2. **Group** - Identify logical segments (by feature, layer, subsystem, risk)
3. **Order** - Determine dependency order (foundation first)
4. **Create** - Build stack with `st new` -> `st append` chain
5. **Populate** - Cherry-pick commits to appropriate branches
6. **Verify** - Ensure each branch builds/tests independently
7. **Submit** - Push and create stacked PRs

**Prompt arguments:**
- `base_branch` - Target branch (default: "main")
- `source_branch` - Branch with monolithic changes (default: current)

### Phase 5: CLI Integration

Add `st mcp` subcommand:

```go
// cmd/st/mcp.go
var mcpCmd = &cobra.Command{
    Use:   "mcp",
    Short: "Start MCP server for LLM integration",
    RunE:  runMCP,
}
```

Usage: `st mcp` (runs stdio server)

---

## File Structure

```
staccato/
├── prd/
│   ├── mcp-server.md                    # This PRD
│   └── prompts/
│       └── split-monolithic-pr.md       # Prompt template
├── cmd/st/
│   ├── mcp.go                           # NEW: MCP subcommand
│   └── ... (existing)
├── pkg/mcp/                             # NEW: MCP package
│   ├── server.go                        # Server setup
│   ├── tools.go                         # Tool definitions
│   ├── prompts.go                       # Prompt definitions
│   ├── types.go                         # Response types
│   └── handlers/
│       ├── stack.go                     # st_log, st_status, st_current
│       ├── branch.go                    # st_new, st_append, st_insert, st_attach
│       ├── restack.go                   # st_restack, st_continue
│       ├── sync.go                      # st_sync
│       ├── pr.go                        # st_pr
│       └── git.go                       # st_git_* helpers
└── go.mod                               # Add mcp-go dependency
```

---

## Verification

### Testing the MCP Server

1. Build: `go build -o st ./cmd/st/`
2. Run: `./st mcp` (starts stdio server)
3. Test with MCP Inspector or Claude Desktop

### Integration Test

```bash
# In a test repo with a monolithic branch
echo '{"jsonrpc":"2.0","id":1,"method":"tools/list"}' | ./st mcp
echo '{"jsonrpc":"2.0","id":2,"method":"prompts/list"}' | ./st mcp
```

### E2E Test Pattern

Add to `cmd/st/main_test.go`:
- Test MCP server starts and responds to JSON-RPC
- Test each tool returns valid responses
- Test prompt returns expected template

---

## Critical Files to Modify

| File | Change |
|------|--------|
| `go.mod` | Add `github.com/mark3labs/mcp-go` dependency |
| `cmd/st/main.go` | Register `mcpCmd` subcommand |
| `pkg/git/git.go` | Add `CherryPick()`, `Reset()`, `Add()`, `Commit()`, `Status()`, `Diff()` methods |

## Design Decisions

| Decision | Choice | Rationale |
|----------|--------|-----------|
| Repo requirement | Require valid git repo | Fail fast with clear error if not in a git repository |
| Transport | Stdio only | Keeps it simple; works with Claude Desktop, Cursor, etc. |
| Tool namespacing | `st_git_` prefix | Groups all tools under st namespace for consistency |
