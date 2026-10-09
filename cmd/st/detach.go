package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func detachCmd() *cobra.Command {
	return withArgs(&cobra.Command{
		Use:   "detach [branch]",
		Short: "Stop tracking a branch, keeping the git branch",
		Long: `Removes the branch from the stack; without [branch], the current branch. The
git branch and its commits are left alone. Its children are given its parent
as their new parent. They keep the detached branch's commits, which become
part of their own change.

Afterwards, run 'st restack' from the tip of the stack to bring the stack's
record up to date. To delete the git branch as well, use 'st delete'.`,
		Example: `st detach
st detach feature/old`,
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
				printer.Println("  Run 'st restack' from the tip of the stack to update the reparented branches")
			}

			return nil
		},
	}, argDoc{"branch", "Branch to stop tracking (default: the current branch)"})
}
