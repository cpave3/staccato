package main

import (
	"fmt"

	"github.com/cpave3/staccato/pkg/backup"
	"github.com/cpave3/staccato/pkg/restack"
	"github.com/spf13/cobra"
)

func moveCmd() *cobra.Command {
	var onto string

	cmd := &cobra.Command{
		Use:   "move",
		Short: "Move the current branch onto a new parent",
		Long:  "Reparents the current branch onto a different parent and restacks downstream branches.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if onto == "" {
				return fmt.Errorf("--onto is required")
			}

			g, gitRunner, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			currentBranch, err := gitRunner.GetCurrentBranch()
			if err != nil {
				return fmt.Errorf("failed to get current branch: %w", err)
			}

			// Must be in the stack
			if currentBranch == g.Root {
				return fmt.Errorf("cannot move the root branch")
			}
			if _, exists := g.GetBranch(currentBranch); !exists {
				return fmt.Errorf("branch '%s' is not in the stack", currentBranch)
			}

			// Cannot move onto self
			if onto == currentBranch {
				return fmt.Errorf("cannot move '%s' onto itself", currentBranch)
			}

			// Target must be in the stack (root or tracked)
			if onto != g.Root {
				if _, exists := g.GetBranch(onto); !exists {
					return fmt.Errorf("target '%s' is not in the stack", onto)
				}
			}

			// Cycle detection: target cannot be a descendant
			descendants := restack.GetDownstreamBranches(g, currentBranch)
			for _, d := range descendants {
				if d == onto {
					return fmt.Errorf("cannot move onto '%s' — would create a cycle", onto)
				}
			}

			// Reparent
			b, _ := g.GetBranch(currentBranch)
			b.Parent = onto

			// Update BaseSHA to the new parent's HEAD
			newBase, _ := gitRunner.GetCommitSHA(onto)
			b.BaseSHA = newBase

			// Restack the moved branch and its descendants
			allBranches := append([]string{currentBranch}, descendants...)
			backupMgr := backup.NewManager(gitRunner, repoPath)
			engine := restack.NewEngine(gitRunner, backupMgr)
			result, restackErr := engine.RestackLineage(g, currentBranch, allBranches)

			if err := saveContext(g, repoPath, gitRunner); err != nil {
				return fmt.Errorf("failed to save graph: %w", err)
			}

			if restackErr != nil {
				if result != nil && result.Conflicts {
					printer.ConflictDetected(result.ConflictsAt)
					return fmt.Errorf("conflict at '%s' — resolve and run 'st continue'", result.ConflictsAt)
				}
				return fmt.Errorf("restack failed: %w", restackErr)
			}

			// Clean up backups
			if result != nil && len(result.Backups) > 0 {
				backupMgr.CleanupStackBackups(allBranches)
			}

			gitRunner.CheckoutBranch(currentBranch)
			printer.Success("Moved '%s' onto '%s'", currentBranch, onto)
			return nil
		},
	}

	cmd.Flags().StringVar(&onto, "onto", "", "Target parent branch")

	return cmd
}
