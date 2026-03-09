## 1. Thread Printer Through Functions

- [x] 1.1 Add `*output.Printer` parameter to `doAttachRecursively` signature
- [x] 1.2 Add `*output.Printer` parameter to `attachInteractive` and `attachRecursively` signatures
- [x] 1.3 Add `*output.Printer` parameter to `attachWithParent` signature
- [x] 1.4 Update `attachCmd` to pass `printer` from `getContext()` to all helper calls

## 2. Replace fmt.Printf Calls

- [x] 2.1 Replace `fmt.Printf("✔ Set '%s' as stack root\n", ...)` calls with `printer.Success("Set '%s' as stack root", ...)`
- [x] 2.2 Replace `fmt.Printf("✔ Attached '%s' as child of '%s'\n", ...)` calls with `printer.Success("Attached '%s' as child of '%s'", ...)`
- [x] 2.3 Replace `fmt.Printf("\nParent '%s' is not yet in the stack...\n", ...)` with `printer.Println(...)`
- [x] 2.4 Replace `fmt.Printf("'%s' already has parent '%s'\n", ...)` with `printer.Println(...)`
- [x] 2.5 Replace `fmt.Printf("✔ Relocated '%s' under '%s'\n", ...)` with `printer.Success("Relocated '%s' under '%s'", ...)`

## 3. Verify

- [x] 3.1 Run `go build ./cmd/st/` to confirm compilation
- [x] 3.2 Run `go test ./cmd/st/ -v -count=1` to confirm E2E tests pass
- [x] 3.3 Run `task check` to confirm build + lint passes
