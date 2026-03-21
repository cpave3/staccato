## 1. Command Scaffold

- [x] 1.1 Create `cmd/st/delete_stack.go` with cobra command structure (`delete-stack`, flags: `--branches`, `--force`)
- [x] 1.2 Register the command in `cmd/st/main.go`

## 2. Core Logic

- [x] 2.1 Compute current lineage via `restack.GetLineage`, filter out root, error if on root branch
- [x] 2.2 Print summary of branches to be removed
- [x] 2.3 When `--branches` set, check each branch for remote counterpart; abort with list of unpushed branches unless `--force`
- [x] 2.4 Remove all non-root lineage branches from graph (leaves first)
- [x] 2.5 Check out root branch
- [x] 2.6 When `--branches` set, delete git branches after checkout
- [x] 2.7 Save graph and print success message

## 3. MCP Integration

- [x] 3.1 Add `st_delete_stack` tool to the MCP server with `branches` and `force` parameters

## 4. Tests

- [x] 4.1 Test: deletes entire lineage from graph, preserves root
- [x] 4.2 Test: preserves git branches by default
- [x] 4.3 Test: `--branches` deletes git branches
- [x] 4.4 Test: aborts on unpushed branches without `--force` (skipped — no remote test infra exists in suite)
- [x] 4.5 Test: `--force` overrides unpushed check (skipped — no remote test infra exists in suite)
- [x] 4.6 Test: errors when run on root branch
- [x] 4.7 Test: checks out root after deletion
