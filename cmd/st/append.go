package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func appendCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "append <branch-name>",
		Short: "Create a child branch from the current branch",
		Long:  "Creates a new branch from the current branch and adds it as a child in the stack.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			branchName := args[0]

			g, git, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			parentBranch, _ := git.GetCurrentBranch()

			// If current branch is not in graph and not root, we need to attach it first
			if parentBranch != g.Root {
				if _, exists := g.GetBranch(parentBranch); !exists {
					return fmt.Errorf("current branch '%s' is not in the stack. Run 'st attach' first", parentBranch)
				}
			}

			// Create branch
			err = git.CreateAndCheckoutBranch(branchName)
			if err != nil {
				return fmt.Errorf("failed to create branch: %w", err)
			}

			// Get SHAs
			baseSHA, _ := git.GetCommitSHA(parentBranch)
			headSHA, _ := git.GetCommitSHA(branchName)

			// Add to graph
			g.AddBranch(branchName, parentBranch, baseSHA, headSHA)

			// Save graph
			if err := saveContext(g, repoPath, git); err != nil {
				return fmt.Errorf("failed to save graph: %w", err)
			}

			printer.BranchCreated(branchName, parentBranch)

			return nil
		},
	}
}
