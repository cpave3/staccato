# AGENTS.md

## Project Overview

**st** is a Git stack management CLI written in Go. It provides deterministic, branch-level stacking inspired by Graphite and Git Town.

## Architecture

### Project Structure
```
st/
├── cmd/st/           # Cobra CLI entrypoint
│   └── main.go       # All commands defined here
├── pkg/
│   ├── graph/        # Stack graph model & JSON persistence
│   ├── git/          # Git operations wrapper (subprocess)
│   ├── backup/       # Backup/restore system
│   ├── restack/      # Topological sort & restack engine
│   ├── attach/       # Lazy attachment logic
│   └── output/       # CLI formatting utilities
├── go.mod            # Module definition
└── st                # Compiled binary
```

### Key Components

#### pkg/graph
- `Graph` struct: Root branch + map of tracked branches
- `Branch` struct: name, parent, base_sha, head_sha
- JSON persistence to `.git/stack/graph.json`
- Versioned for future compatibility

#### pkg/git
- `Runner` struct: Wraps git subprocess calls
- Operations: branch, checkout, rebase, merge-base, rerere
- Error handling with meaningful messages

#### pkg/backup
- `Manager` struct: Handles backup creation/restoration
- Backup naming: `backup/<branch>/<timestamp>`
- Stack-level backup operations
- Cleanup old backups

#### pkg/restack
- `Engine` struct: Orchestrates restack operations
- `TopologicalSort()`: Orders branches (parents first)
- `Restack()`: Rebases stack with conflict handling
- `Continue()`: Resumes after conflict resolution
- `Abort()`: Restores from backups

#### pkg/attach
- `Attacher` struct: Handles lazy attachment
- `SuggestParents()`: Uses merge-base to find candidates
- `AutoAttach()`: Automatic or manual parent selection
- `FindRoot()`: Traces ancestry to find root

#### pkg/output
- `Printer` struct: Consistent CLI output
- Icons: ✔ success, ⚠ warning, ✘ error
- Colored/formatted output

## Commands

### new
Creates branch from root, checks it out.

### append
Creates branch from current, checks it out.

### insert
Creates branch before current, reparents current, restacks downstream, checks out new branch.

### restack
- Topological sort
- Create backups
- Rebase each branch onto parent
- Stop on conflict
- Cleanup backups on success

### continue
Resume restack after manual conflict resolution.

### attach
Add existing branch to graph:
- Suggest parents via merge-base
- Auto mode or manual selection
- Recursive attachment to root

### restore
Restore branch/stack from backup.

### sync
Push branches to remote (explicit, never automatic).

### log
Display stack hierarchy tree.

## Testing

### Test Philosophy
- Integration tests over unit tests
- Test behavior through public interfaces
- Use real git repositories in temp dirs
- One test per behavior (vertical slices)

### Running Tests
```bash
# All tests
go test ./...

# Specific packages
go test ./pkg/graph/... -v
go test ./pkg/git/... -v
go test ./pkg/backup/... -v
go test ./pkg/restack/... -v
go test ./pkg/attach/... -v
```

### Test Patterns
Each package has `*_test.go` files:
- `graph_test.go`: Graph operations, persistence, cycle detection
- `git_test.go`: Git operations, branch management
- `backup_test.go`: Backup creation, restoration, listing
- `restack_test.go`: Topological sort, restack logic
- `attach_test.go`: Attachment, parent suggestions

## Git Operations

All git operations go through `git.Runner`:
- Subprocess execution via `exec.Command`
- Repository path context
- Combined stdout/stderr capture
- Trimmed output

Key operations:
- `CreateAndCheckoutBranch(name)` - Create and switch
- `Rebase(target)` - Rebase current onto target
- `GetMergeBase(a, b)` - Find common ancestor
- `IsRebaseInProgress()` - Check for active rebase
- `EnableRerere()` - Enable conflict recording

## Error Handling

- Return errors with context: `fmt.Errorf("context: %w", err)`
- Check rebase status to detect conflicts
- Restore backups on failure
- Clear error messages to user

## Stack Graph Format

JSON format in `.git/stack/graph.json`:

```json
{
  "version": 1,
  "root": "main",
  "branches": {
    "feature": {
      "name": "feature",
      "parent": "main",
      "base_sha": "abc...",
      "head_sha": "def..."
    }
  }
}
```

## Common Tasks

### Adding a new command
1. Add function in `cmd/st/main.go`
2. Use `cobra.Command` structure
3. Call `rootCmd.AddCommand()` in `init()`
4. Implement using existing packages
5. Test manually with real git repo

### Adding a new package
1. Create `pkg/<name>/<name>.go`
2. Create `pkg/<name>/<name>_test.go`
3. Follow existing patterns (struct + New* constructor)
4. Write integration tests first
5. Export only public interface

### Modifying existing commands
1. Read current implementation in `cmd/st/main.go`
2. Understand data flow: getContext() → operation → saveContext()
3. Make changes
4. Test with real repository
5. Update any affected tests

## Dependencies

- `github.com/spf13/cobra` - CLI framework
- Standard library only for packages

## Build & Run

```bash
# Build
go build -o st ./cmd/st/

# Install to PATH
ln -sf $(pwd)/st ~/.local/bin/st

# Run
st --help
st new feature-branch
```

## Code Style

- Go standard formatting (`gofmt`)
- Clear variable names
- Document public functions
- Keep functions focused
- Error messages: lowercase, no period

## Future Considerations

- Stack graph versioning for migrations
- More sophisticated conflict handling
- Interactive UI for parent selection
- PR creation integration
- Configurable backup retention
