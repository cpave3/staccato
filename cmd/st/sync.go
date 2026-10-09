package main

import (
	"fmt"

	stcontext "github.com/cpave3/staccato/pkg/context"
	"github.com/cpave3/staccato/pkg/hooks"
	stync "github.com/cpave3/staccato/pkg/sync"
	"github.com/spf13/cobra"
)

func syncCmd() *cobra.Command {
	var dryRun bool
	var downOnly bool

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
worktree is taken out of the stack but deleted only by a later sync.`,
		Example: `st sync
st sync --dry-run
st sync --down`,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, gitRunner, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			if err := requireBranch(gitRunner); err != nil {
				return err
			}

			if dryRun {
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

			printer.SyncFetching()

			sc := stcontext.NewContext(g, gitRunner, repoPath)

			result, err := stync.Run(sc, stync.Options{
				DryRun:   dryRun,
				DownOnly: downOnly,
			})

			if dryRun && result != nil {
				if result.TrunkUpdated {
					fmt.Printf("Would fast-forward '%s'\n", g.Root)
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
				return nil
			}

			if result != nil {
				if result.TrunkUpdated {
					printer.SyncTrunkUpdated(g.Root)
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

				pushedCount := len(result.PushedBranches)
				for _, branch := range result.PushedBranches {
					printer.Info("Pushed: %s", branch)
				}

				printer.SyncSummary(len(result.MergedBranches), result.RestackedCount, pushedCount)
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
						"restacked_count":            result.RestackedCount,
						"deferred_branch_deletions":  result.DeferredBranchDeletions,
						"completed_branch_deletions": result.CompletedBranchDeletions,
					},
				})
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Fetch, then print what sync would do without doing it")
	cmd.Flags().BoolVar(&downOnly, "down", false, "Fetch and restack only; push nothing")

	return cmd
}
