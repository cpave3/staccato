## 1. Test Setup

- [x] 1.1 Create `pkg/output/output_test.go` with a stdout capture helper function
- [x] 1.2 Add test for `NewPrinter` and `SetVerbose`

## 2. Core Formatting Tests

- [x] 2.1 Test `Success` outputs with `✔` icon prefix
- [x] 2.2 Test `Warning` outputs with `⚠` icon prefix
- [x] 2.3 Test `Error` outputs with `✘` icon prefix
- [x] 2.4 Test `Info` prints with `ℹ` icon when verbose is true
- [x] 2.5 Test `Info` suppresses output when verbose is false
- [x] 2.6 Test `Print` outputs raw formatted string without newline
- [x] 2.7 Test `Println` outputs formatted string with newline

## 3. Convenience Method Tests

- [x] 3.1 Test `BranchCreated` message format
- [x] 3.2 Test `RestackComplete` message format
- [x] 3.3 Test `ConflictDetected` outputs warning and instructions

## 4. Tree Rendering Tests

- [x] 4.1 Test `StackLog` with a simple root → child → grandchild graph
- [x] 4.2 Test `StackLog` highlights current branch with `●`

## 5. PR Status Tests

- [x] 5.1 Test `formatPRStatus` for MERGED state
- [x] 5.2 Test `formatPRStatus` for CLOSED state
- [x] 5.3 Test `formatPRStatus` for OPEN with draft, approved, changes-requested, and CI states

## 6. Verify

- [x] 6.1 Run `go test ./pkg/output/ -v -count=1` and confirm all tests pass
- [x] 6.2 Run `task check` to confirm build + lint passes
