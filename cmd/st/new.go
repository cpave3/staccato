package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func newCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "new <branch-name>",
		Short: "Create a new branch from the current root/trunk",
		Long:  "Creates a new branch from the current root branch and adds it to the stack graph.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			branchName := args[0]

			g, git, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			if err := requireBranch(git); err != nil {
				return err
			}

			checkStaleness(g, git, printer)

			// Check if branch already exists
			if exists, _ := git.BranchExists(branchName); exists {
				return fmt.Errorf("branch '%s' already exists — use 'st attach %s' to add it to the stack", branchName, branchName)
			}

			// Create branch from root
			err = git.CreateAndCheckoutBranch(branchName)
			if err != nil {
				return fmt.Errorf("failed to create branch: %w", err)
			}

			// Get SHAs
			baseSHA, _ := git.GetCommitSHA(g.Root)
			headSHA, _ := git.GetCommitSHA(branchName)

			// Add to graph
			g.AddBranch(branchName, g.Root, baseSHA, headSHA)

			// Save graph
			if err := saveContext(g, repoPath, git); err != nil {
				return fmt.Errorf("failed to save graph: %w", err)
			}

			printer.BranchCreated(branchName, g.Root)

			return nil
		},
	}
}
