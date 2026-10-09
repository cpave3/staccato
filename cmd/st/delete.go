package main

import (
	"fmt"

	"github.com/cpave3/staccato/pkg/hooks"
	"github.com/spf13/cobra"
)

func deleteCmd() *cobra.Command {
	var force bool

	cmd := &cobra.Command{
		Use:   "delete <branch>",
		Short: "Delete a branch and take it out of the stack",
		Long: `Removes the branch from the stack, gives its children its parent as their new
parent, and deletes the git branch. If it is the current branch, its parent is
checked out first. The children keep the deleted branch's commits, which become
part of their own change. Afterwards, run 'st top' and 'st restack' to bring
the stack's record up to date.

When the repository has a remote, a branch that was never pushed is refused
unless --force is given. To keep the git branch, use 'st detach'.`,
		Example: `st delete feature/old
st delete feature/old --force`,
		Args: cobra.ExactArgs(1),
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

			hooks.NewRunner(repoPath).Fire(hooks.Context{
				Event:    hooks.PostBranchDelete,
				RepoPath: repoPath,
				Branch:   branchName,
				Data:     map[string]any{"parent": b.Parent},
			})

			return nil
		},
	}

	cmd.Flags().BoolVarP(&force, "force", "f", false, "Delete the branch even if it was never pushed")

	return withArgs(cmd, argDoc{"branch", "Branch to delete"})
}
