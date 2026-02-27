package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/cpave3/staccato/pkg/backup"
	"github.com/cpave3/staccato/pkg/restack"
)

func insertCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "insert <branch-name>",
		Short: "Insert a branch before the current branch",
		Long: `Inserts a new branch before the current branch in the stack.
The current branch and all downstream branches will be reparented and restacked.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			branchName := args[0]

			g, git, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			currentBranch, _ := git.GetCurrentBranch()

			// Get current branch's parent
			currentBranchInfo, exists := g.GetBranch(currentBranch)
			if !exists {
				return fmt.Errorf("current branch '%s' is not in the stack", currentBranch)
			}

			oldParent := currentBranchInfo.Parent

			// Create backup manager
			backupMgr := backup.NewManager(git, repoPath)

			// Create backups of all affected branches
			downstreamBranches := restack.GetDownstreamBranches(g, currentBranch)
			affectedBranches := append([]string{currentBranch}, downstreamBranches...)

			backups, err := backupMgr.CreateBackupsForStack(affectedBranches)
			if err != nil {
				return fmt.Errorf("failed to create backups: %w", err)
			}

			// Create new branch from old parent
			err = git.CheckoutBranch(oldParent)
			if err != nil {
				return fmt.Errorf("failed to checkout parent: %w", err)
			}

			err = git.CreateAndCheckoutBranch(branchName)
			if err != nil {
				return fmt.Errorf("failed to create branch: %w", err)
			}

			// Get SHAs
			baseSHA, _ := git.GetCommitSHA(oldParent)
			headSHA, _ := git.GetCommitSHA(branchName)

			// Add new branch to graph
			g.AddBranch(branchName, oldParent, baseSHA, headSHA)

			// Reparent current branch to new branch
			g.Branches[currentBranch].Parent = branchName

			// Save graph first
			if err := saveContext(g, repoPath, git); err != nil {
				return fmt.Errorf("failed to save graph: %w", err)
			}

			printer.BranchInserted(branchName, currentBranch)

			// Now restack downstream branches
			printer.Println("Restacking downstream branches...")

			engine := restack.NewEngine(git, backupMgr)
			result, err := engine.Restack(g, branchName)
			if err != nil {
				if result.Conflicts {
					printer.ConflictDetected(result.ConflictsAt)
					return fmt.Errorf("conflict during restack")
				}

				// Restore backups on error
				printer.Error("Restack failed, restoring from backups...")
				backupMgr.RestoreStack(backups)
				return err
			}

			// Cleanup backups
			backupMgr.CleanupStackBackups(affectedBranches)

			printer.RestackComplete(len(result.Completed))

			// Checkout the newly inserted branch
			git.CheckoutBranch(branchName)

			return nil
		},
	}
}
