package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/cpave3/staccato/pkg/backup"
	"github.com/cpave3/staccato/pkg/restack"
)

func restackCmd() *cobra.Command {
	var toCurrent bool
	cmd := &cobra.Command{
		Use:   "restack",
		Short: "Restack the entire stack",
		Long: `Rebases all branches in the stack onto their parents in topological order.
Creates backups before any destructive operations. Stops on first conflict.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, git, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			if err := requireBranch(git); err != nil {
				return err
			}

			checkStaleness(g, git, printer)

			warnDirtyTree(git, printer)

			currentBranch, _ := git.GetCurrentBranch()

			// Check if current branch is in the stack
			if currentBranch != g.Root {
				if _, exists := g.GetBranch(currentBranch); !exists {
					return fmt.Errorf("current branch '%s' is not in the stack", currentBranch)
				}
			}

			// Get only the current lineage (not all branches under root)
			lineageBranches := restack.GetLineage(g, currentBranch)

			// Check if we're at the tip
			if !restack.IsBranchAtTip(g, currentBranch) {
				if !toCurrent {
					printer.Warning("You are not at the tip of your stack lineage")
					printer.Println("  Use --to-current to restack only up to '%s'", currentBranch)
					printer.Println("  Or switch to the tip branch and run 'st restack'")
					return fmt.Errorf("specify --to-current or switch to the tip branch")
				}
				lineageBranches = restack.GetAncestors(g, currentBranch)
			}

			printer.RestackStart(currentBranch)

			// Create backup manager
			backupMgr := backup.NewManager(git, repoPath)

			// Perform restack for this lineage only
			engine := restack.NewEngine(git, backupMgr)
			result, err := engine.RestackLineage(g, currentBranch, lineageBranches)

			// Save graph state (even if there was an error)
			saveContext(g, repoPath, git)

			if err != nil {
				if result.Conflicts {
					// Save restack state so continue knows the lineage
					restack.SaveRestackState(repoPath, &restack.RestackState{
						Lineage: lineageBranches,
					})
					printer.ConflictDetected(result.ConflictsAt)
					return fmt.Errorf("conflict during restack at '%s' — resolve the conflicts and run 'st continue'", result.ConflictsAt)
				}

				// Check if we should restore
				if len(result.Backups) > 0 {
					printer.Error("Restack failed, run 'st restore' to recover")
				}
				return err
			}

			// Cleanup backups and restack state on success
			restack.ClearRestackState(repoPath)
			if len(result.Backups) > 0 {
				backupMgr.CleanupStackBackups(lineageBranches)
			}

			printer.RestackComplete(len(result.Completed))

			return nil
		},
	}
	cmd.Flags().BoolVar(&toCurrent, "to-current", false, "Restack only up to the current branch")
	return cmd
}
