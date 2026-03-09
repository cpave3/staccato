package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func detachCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "detach [branch-name]",
		Short: "Remove a branch from the stack graph",
		Long: `Removes a branch from the stack graph while keeping the git branch intact.
If no branch name is provided, uses the current branch.
Children of the detached branch are reparented to the detached branch's parent.`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			g, git, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			if err := requireBranch(git); err != nil {
				return err
			}

			var branchName string
			if len(args) > 0 {
				branchName = args[0]
			} else {
				branchName, _ = git.GetCurrentBranch()
			}

			// Cannot detach root
			if branchName == g.Root {
				return fmt.Errorf("cannot detach the root branch '%s'", branchName)
			}

			// Must be in graph
			if _, exists := g.GetBranch(branchName); !exists {
				return fmt.Errorf("branch '%s' is not in the stack", branchName)
			}

			parent := g.Branches[branchName].Parent
			children := g.GetChildren(branchName)

			// Reparent children
			if len(children) > 0 {
				g.ReparentChildren(branchName, parent)
			}

			// Remove from graph
			g.RemoveBranch(branchName)

			// Save
			if err := saveContext(g, repoPath, git); err != nil {
				return fmt.Errorf("failed to save graph: %w", err)
			}

			printer.Success("Detached '%s' from stack", branchName)

			if len(children) > 0 {
				var names []string
				for _, c := range children {
					names = append(names, c.Name)
				}
				printer.Println("  Reparented %d child branch(es) to '%s': %v", len(children), parent, names)
				printer.Println("  Run 'st restack' to update the reparented branches")
			}

			return nil
		},
	}
}
