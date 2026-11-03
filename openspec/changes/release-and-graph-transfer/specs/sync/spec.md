## MODIFIED Requirements

### Requirement: Push with Upstream Tracking

The sync command SHALL push remaining stack branches to the remote with upstream tracking and force-with-lease. A push that fails SHALL be reported, by branch, with git's reason, and SHALL make the command exit non-zero once the remaining branches have been pushed.

#### Scenario: Successful push
- **WHEN** restacking completes without conflicts
- **AND** the `--down` flag is not set
- **THEN** the command MUST push each non-trunk branch in the current lineage using `git push -u origin <branch> --force-with-lease`
- **AND** each successfully pushed branch MUST be recorded in PushedBranches

#### Scenario: Push failure for a branch
- **WHEN** pushing a specific branch fails
- **THEN** that branch MUST NOT appear in PushedBranches
- **AND** it MUST be recorded in FailedPushes with git's reason: the status line git printed for the refused ref (for example `! [remote rejected] x -> x (pending ap-…)` or `! [rejected] x -> x (stale info)`), or, when there is none, the last `fatal:`, `error:` or `remote:` line other than git's generic `error: failed to push some refs to …` and its `hint:` lines
- **AND** the sync command MUST continue pushing remaining branches (non-fatal)
- **AND** after the summary, the command MUST print a warning naming each failed branch and its reason
- **AND** the command MUST exit with a non-zero status
- **AND** the post-sync hook MUST still fire, with `failed_pushes` in its data

#### Scenario: Failed pushes in JSON output
- **WHEN** `st sync --json` runs and a branch fails to push
- **THEN** the output MUST include `failed_pushes`, a list of `{"branch", "reason"}`
- **AND** `failed_pushes` MUST be an empty list when every push succeeded

#### Scenario: Trunk branch excluded from push
- **WHEN** pushing branches in the lineage
- **THEN** the trunk/root branch MUST be excluded from the push list

#### Scenario: Shared graph ref push
- **WHEN** the `--down` flag is not set
- **AND** the shared graph ref exists
- **THEN** the command MUST push the shared graph ref to the remote
- **AND** a failure to push it MUST be reported as a warning with git's reason, recorded in FailedPushes under the ref's name, and set a non-zero exit status

## ADDED Requirements

### Requirement: Sync JSON Output

`st sync --json` SHALL print one JSON object on standard output once the sync is over, and nothing else on standard output:

```json
{
  "trunk_updated": true,
  "merged_branches": [],
  "restacked_count": 2,
  "pushed_branches": ["a", "b"],
  "failed_pushes": [{"branch": "c", "reason": "! [remote rejected] c -> c (pending ap-1)"}],
  "conflict_at": ""
}
```

Every list is present, empty when there is nothing in it. With `--dry-run`, `pushed_branches` lists the branches that would be pushed. A sync that stops on a conflict still prints the object, with `conflict_at` set, and exits non-zero.

#### Scenario: JSON when everything pushed
- **WHEN** `st sync --json` pushes every branch
- **THEN** standard output MUST be a single JSON object with `failed_pushes` an empty list, and the command MUST exit 0
