package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/cpave3/staccato/pkg/backup"
	"github.com/cpave3/staccato/pkg/restack"
)

func syncCmd() *cobra.Command {
	var dryRun bool
	var downOnly bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Fetch, detect merged branches, restack & push",
		Long: `Fetches from remote, detects branches merged on GitHub, removes them from
the stack graph (reparenting children), restacks remaining branches, and pushes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, gitRunner, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			if dryRun {
				printer.DryRunNotice()
			}

			// 1. Check remote
			hasRemote, _ := gitRunner.HasRemote()
			if !hasRemote {
				return fmt.Errorf("no remote configured")
			}

			originalBranch, _ := gitRunner.GetCurrentBranch()

			// 2. Fetch with prune
			printer.SyncFetching()
			if err := gitRunner.FetchPrune(); err != nil {
				return fmt.Errorf("fetch failed: %w", err)
			}

			// 3. Fetch with read-only detection for dry-run
			trunk := g.Root

			if dryRun {
				// Dry-run: only report what would happen, no local modifications
				if gitRunner.RemoteBranchExists(trunk) {
					isAnc, err := gitRunner.IsAncestor(trunk, "origin/"+trunk)
					if err == nil && isAnc {
						behindTrunk, _ := gitRunner.IsAncestor("origin/"+trunk, trunk)
						if !behindTrunk {
							fmt.Printf("Would fast-forward '%s'\n", trunk)
						}
					}
				}

				// Detect merged branches
				sorted, err := restack.TopologicalSort(g, trunk)
				if err != nil {
					return fmt.Errorf("failed to sort branches: %w", err)
				}

				var mergedBranches []string
				for _, branch := range sorted {
					merged := false
					isAnc, err := gitRunner.IsAncestor(branch, "origin/"+trunk)
					if err == nil && isAnc {
						merged = true
					}
					if !merged && !gitRunner.RemoteBranchExists(branch) {
						diffEmpty, err := gitRunner.DiffIsEmpty("origin/"+trunk, branch)
						if err == nil && diffEmpty {
							merged = true
						}
					}
					if merged {
						mergedBranches = append(mergedBranches, branch)
						fmt.Printf("Would remove merged branch: %s\n", branch)
					}
				}

				if len(mergedBranches) > 0 {
					fmt.Println("Would restack remaining branches")
				}

				// Report what would be pushed
				remaining := restack.GetLineage(g, originalBranch)
				pushCount := 0
				if !downOnly {
					for _, branch := range remaining {
						if branch == trunk {
							continue
						}
						fmt.Printf("Would push: %s\n", branch)
						pushCount++
					}
				}

				if pushCount == 0 && len(mergedBranches) == 0 {
					fmt.Println("Nothing to do.")
				}

				return nil
			}

			// 3. Fast-forward trunk
			if gitRunner.RemoteBranchExists(trunk) {
				if originalBranch == trunk {
					if err := gitRunner.MergeFFOnly("origin/" + trunk); err != nil {
						printer.Info("Could not fast-forward '%s' (may have local changes)", trunk)
					} else {
						printer.SyncTrunkUpdated(trunk)
					}
				} else {
					if err := gitRunner.FastForwardBranch(trunk, "origin/"+trunk); err != nil {
						printer.Info("Could not fast-forward '%s'", trunk)
					} else {
						printer.SyncTrunkUpdated(trunk)
					}
				}
			}

			// 4. Detect merged branches (topological order)
			sorted, err := restack.TopologicalSort(g, trunk)
			if err != nil {
				return fmt.Errorf("failed to sort branches: %w", err)
			}

			var mergedBranches []string
			for _, branch := range sorted {
				merged := false

				// Check regular merge: branch is ancestor of origin/trunk
				isAnc, err := gitRunner.IsAncestor(branch, "origin/"+trunk)
				if err == nil && isAnc {
					merged = true
				}

				// Check squash merge: remote branch gone AND diff to trunk is empty
				if !merged && !gitRunner.RemoteBranchExists(branch) {
					diffEmpty, err := gitRunner.DiffIsEmpty("origin/"+trunk, branch)
					if err == nil && diffEmpty {
						merged = true
					}
				}

				if merged {
					mergedBranches = append(mergedBranches, branch)
					printer.SyncMergedDetected(branch)
				}
			}

			// 5. Remove merged branches
			if len(mergedBranches) == 0 {
				printer.SyncNoMergedBranches()
			}

			for _, branch := range mergedBranches {
				b, exists := g.GetBranch(branch)
				if !exists {
					continue
				}
				parent := b.Parent

				g.ReparentChildren(branch, parent)
				g.RemoveBranch(branch)

				// If user is on this branch, checkout trunk
				cur, _ := gitRunner.GetCurrentBranch()
				if cur == branch {
					gitRunner.CheckoutBranch(trunk)
					originalBranch = trunk
				}

				gitRunner.DeleteBranch(branch, true)
				printer.SyncBranchRemoved(branch)
			}

			// 6. Save graph if branches were removed
			if len(mergedBranches) > 0 {
				if err := saveContext(g, repoPath, gitRunner); err != nil {
					return fmt.Errorf("failed to save graph: %w", err)
				}
			}

			// 7. Restack remaining branches (after trunk update or branch removal)
			restackedCount := 0
			if len(g.Branches) > 0 {
				backupMgr := backup.NewManager(gitRunner, repoPath)
				engine := restack.NewEngine(gitRunner, backupMgr)
				result, err := engine.Restack(g, trunk)
				if err != nil {
					if result != nil && result.Conflicts {
						printer.ConflictDetected(result.ConflictsAt)
						saveContext(g, repoPath, gitRunner)
						return fmt.Errorf("conflict during restack - resolve and run 'st continue'")
					}
					return fmt.Errorf("restack failed: %w", err)
				}
				restackedCount = len(result.Completed)
				saveContext(g, repoPath, gitRunner)
			}

			// 8. Push remaining branches in current lineage (force-with-lease since they may have been rebased)
			remaining := restack.GetLineage(g, originalBranch)
			pushedCount := 0
			if !downOnly {
				for _, branch := range remaining {
					if branch == trunk {
						continue
					}
					forceNeeded := true
					if err := gitRunner.Push(branch, forceNeeded); err != nil {
						printer.Error("Failed to push %s: %v", branch, err)
					} else {
						printer.Info("Pushed: %s", branch)
						pushedCount++
					}
				}
			}

			// 9. Restore original branch if it still exists
			if originalBranch != "" {
				exists, _ := gitRunner.BranchExists(originalBranch)
				if exists {
					gitRunner.CheckoutBranch(originalBranch)
				}
			}

			// 10. Print summary
			printer.SyncSummary(len(mergedBranches), restackedCount, pushedCount)

			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be pushed without pushing")
	cmd.Flags().BoolVar(&downOnly, "down", false, "Only pull changes from remote, skip pushing")

	return cmd
}
