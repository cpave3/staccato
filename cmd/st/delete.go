package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func deleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <branch-name>",
		Short: "Delete a branch from the stack",
		Long:  "Removes a branch from the stack graph, reparents its children to its parent, and deletes the git branch.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			branchName := args[0]

			g, gitRunner, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			// Cannot delete root
			if branchName == g.Root {
				return fmt.Errorf("cannot delete the root branch '%s'", branchName)
			}

			// Must be in the stack
			b, exists := g.GetBranch(branchName)
			if !exists {
				return fmt.Errorf("branch '%s' is not in the stack", branchName)
			}

			// Check for unpushed commits (warn unless --force)
			if !force && !gitRunner.RemoteBranchExists(branchName) {
				hasRemote, _ := gitRunner.HasRemote()
				if hasRemote {
					fmt.Printf("Warning: '%s' has not been pushed to remote\n", branchName)
					fmt.Println("Use --force to delete anyway")
					return fmt.Errorf("branch '%s' has unpushed commits — use --force to delete", branchName)
				}
			}

			// If we're on this branch, checkout parent first
			currentBranch, _ := gitRunner.GetCurrentBranch()
			if currentBranch == branchName {
				if err := gitRunner.CheckoutBranch(b.Parent); err != nil {
					return fmt.Errorf("failed to checkout parent: %w", err)
				}
			}

			// Reparent children to deleted branch's parent
			g.ReparentChildren(branchName, b.Parent)

			// Remove from graph
			g.RemoveBranch(branchName)

			// Save graph before deleting git branch
			if err := saveContext(g, repoPath, gitRunner); err != nil {
				return fmt.Errorf("failed to save graph: %w", err)
			}

			// Delete git branch
			if err := gitRunner.DeleteBranch(branchName, true); err != nil {
				printer.Warning("failed to delete git branch: %v", err)
			}

			printer.Success("Deleted '%s' from stack", branchName)
			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force delete even if branch has unpushed commits")

	return cmd
}
