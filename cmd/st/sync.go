package main

import (
	"fmt"
	"strings"

	stcontext "github.com/cpave3/staccato/pkg/context"
	"github.com/cpave3/staccato/pkg/hooks"
	"github.com/cpave3/staccato/pkg/output"
	stync "github.com/cpave3/staccato/pkg/sync"
	"github.com/spf13/cobra"
)

func syncCmd() *cobra.Command {
	var dryRun bool
	var downOnly bool
	var asJSON bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Fetch, drop merged branches, restack and push the stack",
		Long: `Brings the stack up to date with the remote and publishes it:

  1. fetches from origin, pruning remote branches that were deleted
  2. fast-forwards the trunk to origin's
  3. finds the branches whose pull request was merged, squash merges included,
     removes them from the stack and deletes them, and gives their children
     the merged branch's parent
  4. restacks the current lineage onto the updated trunk
  5. pushes the branches of the current lineage, with --force-with-lease

--down stops after step 4, so nothing is pushed. --dry-run fetches and then
prints what steps 2, 3 and 5 would do, without doing them.

If the restack stops on a conflict, resolve it, run 'st continue', then run
'st sync' again to push. A merged branch that is checked out in another
worktree is taken out of the stack but deleted only by a later sync.

A branch whose push is refused (by a hook, a protected branch, a lease that
is out of date, or a proxy holding it for approval) does not stop the others
from being pushed. After the summary, st sync prints a warning naming each
such branch with git's reason, and then exits 1.

With --json, st sync prints only a JSON object on standard output once it is
over:

  {
    "trunk_updated": true,
    "merged_branches": [],
    "restacked_count": 2,
    "pushed_branches": ["feature-core", "feature-api"],
    "failed_pushes": [
      {"branch": "feature-ui",
       "reason": "! [remote rejected] feature-ui -> feature-ui (pending ap-1)"}
    ],
    "conflict_at": ""
  }

Every list is present, and empty when there is nothing in it. With --dry-run,
"pushed_branches" lists the branches that would be pushed. When the restack
stops on a conflict, "conflict_at" names the branch and st sync exits 1.`,
		Example: `st sync
st sync --dry-run
st sync --down
st sync --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, gitRunner, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			if err := requireBranch(gitRunner); err != nil {
				return err
			}

			if dryRun && !asJSON {
				printer.DryRunNotice()
			}

			// Fire pre-sync hook (can block), skip on dry-run
			if !dryRun {
				hookRunner := hooks.NewRunner(repoPath)
				if err := hookRunner.Fire(hooks.Context{
					Event:    hooks.PreSync,
					RepoPath: repoPath,
				}); err != nil {
					return fmt.Errorf("pre-sync hook: %w", err)
				}
			}

			if !asJSON {
				printer.SyncFetching()
			}

			sc := stcontext.NewContext(g, gitRunner, repoPath)

			result, err := stync.Run(sc, stync.Options{
				DryRun:   dryRun,
				DownOnly: downOnly,
			})

			if asJSON && result != nil {
				if jsonErr := writeJSON(cmd.OutOrStdout(), newSyncReport(result)); jsonErr != nil {
					return jsonErr
				}
			}

			if dryRun && result != nil {
				if !asJSON {
					printDryRun(g.Root, result)
				}
				return err
			}

			if result != nil && !asJSON {
				printSyncResult(printer, g.Root, result)
			}

			if err != nil {
				return err
			}

			// Fire post-sync hook (skip on dry-run)
			if !dryRun && result != nil {
				hooks.NewRunner(repoPath).Fire(hooks.Context{
					Event:    hooks.PostSync,
					RepoPath: repoPath,
					Data: map[string]any{
						"merged_branches":            result.MergedBranches,
						"pushed_branches":            result.PushedBranches,
						"failed_pushes":              nonNil(result.FailedPushes),
						"restacked_count":            result.RestackedCount,
						"deferred_branch_deletions":  result.DeferredBranchDeletions,
						"completed_branch_deletions": result.CompletedBranchDeletions,
					},
				})
			}

			if result != nil && len(result.FailedPushes) > 0 {
				return failedPushesError(result.FailedPushes)
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Fetch, then print what sync would do without doing it")
	cmd.Flags().BoolVar(&downOnly, "down", false, "Fetch and restack only; push nothing")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print what sync did as JSON")

	return cmd
}

// syncReport is the JSON form of a sync's result.
type syncReport struct {
	TrunkUpdated   bool               `json:"trunk_updated"`
	MergedBranches []string           `json:"merged_branches"`
	RestackedCount int                `json:"restacked_count"`
	PushedBranches []string           `json:"pushed_branches"`
	FailedPushes   []stync.FailedPush `json:"failed_pushes"`
	ConflictAt     string             `json:"conflict_at"`
}

func newSyncReport(result *stync.Result) syncReport {
	return syncReport{
		TrunkUpdated:   result.TrunkUpdated,
		MergedBranches: nonNil(result.MergedBranches),
		RestackedCount: result.RestackedCount,
		PushedBranches: nonNil(result.PushedBranches),
		FailedPushes:   nonNil(result.FailedPushes),
		ConflictAt:     result.ConflictsAt,
	}
}

// nonNil returns s, or an empty slice for nil, so that it encodes as [] rather than null.
func nonNil[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s
}

func printDryRun(root string, result *stync.Result) {
	if result.TrunkUpdated {
		fmt.Printf("Would fast-forward '%s'\n", root)
	}
	for _, branch := range result.MergedBranches {
		fmt.Printf("Would remove merged branch: %s\n", branch)
	}
	if len(result.MergedBranches) > 0 {
		fmt.Println("Would restack remaining branches")
	}
	for _, branch := range result.PushedBranches {
		fmt.Printf("Would push: %s\n", branch)
	}
	if len(result.PushedBranches) == 0 && len(result.MergedBranches) == 0 {
		fmt.Println("Nothing to do.")
	}
}

func printSyncResult(printer *output.Printer, root string, result *stync.Result) {
	if result.TrunkUpdated {
		printer.SyncTrunkUpdated(root)
	}
	for _, branch := range result.MergedBranches {
		printer.SyncMergedDetected(branch)
	}
	if len(result.MergedBranches) == 0 {
		printer.SyncNoMergedBranches()
	}
	if result.StashedFromBranch != "" {
		printer.Warning("Stashing uncommitted changes from merged branch '%s'", result.StashedFromBranch)
	}
	deferred := make(map[string]bool, len(result.DeferredBranchDeletions))
	for _, branch := range result.DeferredBranchDeletions {
		deferred[branch] = true
		printer.SyncBranchDeletionDeferred(branch)
	}
	for _, branch := range result.CompletedBranchDeletions {
		printer.SyncBranchCleanupCompleted(branch)
	}
	for _, branch := range result.MergedBranches {
		if !deferred[branch] {
			printer.SyncBranchRemoved(branch)
		}
	}

	if result.Conflicts {
		printer.ConflictDetected(result.ConflictsAt)
	}

	for _, branch := range result.PushedBranches {
		printer.Info("Pushed: %s", branch)
	}

	printer.SyncSummary(len(result.MergedBranches), result.RestackedCount, len(result.PushedBranches))

	for _, failed := range result.FailedPushes {
		printer.Warning("failed to push %s: %s", failed.Branch, failed.Reason)
	}
}

// failedPushesError is st sync's error when some pushes were refused; the
// warnings above it give the reasons.
func failedPushesError(failed []stync.FailedPush) error {
	names := make([]string, len(failed))
	for i, f := range failed {
		names[i] = f.Branch
	}
	return fmt.Errorf("%d push(es) failed: %s", len(failed), strings.Join(names, ", "))
}
