## Why

The codebase has an `output.Printer` abstraction for consistent CLI output with icons and formatting, but `cmd/st/attach.go` bypasses it with 12+ raw `fmt.Printf` calls using hardcoded icons. This inconsistency means attach output can't be controlled by the verbose flag and doesn't follow the same formatting patterns as other commands.

## What Changes

- Replace all raw `fmt.Printf` calls in `cmd/st/attach.go` with equivalent `Printer` method calls
- Thread the `Printer` instance through `attachInteractive`, `attachRecursively`, `doAttachRecursively`, and `attachWithParent` functions
- Use `printer.Success()` for success messages, `printer.Println()` for informational output

## Capabilities

### New Capabilities
- `attach-printer-consistency`: Consistent use of `output.Printer` in the attach command, replacing raw `fmt.Printf` calls with proper printer methods.

### Modified Capabilities

## Impact

- `cmd/st/attach.go`: Replace ~12 `fmt.Printf` calls with `Printer` methods, update function signatures to accept `*output.Printer`
- `cmd/st/main_test.go`: May need minor updates if attach test helpers change signatures
