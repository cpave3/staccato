## 1. Multi-Conflict Restack Cycle Tests

- [x] 1.1 Add E2E test: sequential conflicts across two branches (root → s1 → s2 → s3, conflicts at s1 and s2, two continue cycles to completion)
- [x] 1.2 Add E2E test: verify graph state consistency after each continue (SHAs correct for rebased branches, unchanged for pending branches)
- [x] 1.3 Add E2E test: restack state file persists across multiple continues and is cleared on final success

## 2. Restore Integrity Tests

- [x] 2.1 Add E2E test: `st restore --all` after conflict returns exact pre-restack state (branch HEADs, graph SHAs, file contents)
- [x] 2.2 Add E2E test: `st restore --all` after partial continue (s1 resolved, s2 conflicts) returns to pre-restack state
- [x] 2.3 Add E2E test: restore aborts active rebase, clears restack state, leaves user on valid branch
- [x] 2.4 Add E2E test: verify no rebase artifacts remain after restore (no .git/rebase-merge directory)

## 3. Backup Integrity Tests

- [x] 3.1 Add E2E test: automatic backups survive through multiple continue cycles (backups still exist after first continue)
- [x] 3.2 Add E2E test: backups cleaned up only on final successful completion (not after intermediate continues)
- [x] 3.3 Add E2E test: restoring from backups after multiple continues returns to pre-restack state (not intermediate state)

## 4. Rerere Integration Tests

- [x] 4.1 Add E2E test: after resolving a conflict and completing restack, restoring and restacking again auto-resolves via rerere

## 5. Sync Conflict Interaction Tests

- [x] 5.1 Add E2E test: `st sync` triggers restack conflict, `st continue` resumes and completes successfully

## 6. Bug Fixes (if discovered)

- [x] 6.1 Fix any graph state consistency issues found during testing (e.g., children BaseSHA not updated after restore)
- [x] 6.2 Fix any restack state file issues found during testing
- [x] 6.3 Fix any backup cleanup edge cases found during testing
