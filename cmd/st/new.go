package main

import (
	"fmt"

	"github.com/cpave3/staccato/pkg/hooks"
	"github.com/spf13/cobra"
)

func newCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "new <branch>",
		Short: "Start a new stack: create a branch on the trunk",
		Long: `Creates <branch> from the trunk and checks it out, whichever branch you are
on. The trunk is the stack's root branch, normally main or master.

To stack on top of the current branch instead, use 'st append'. A branch name
that already exists is refused: add that branch to the stack with 'st attach'.`,
		Example: `st new feature/api`,
		Args:    cobra.ExactArgs(1),
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
			err = git.CreateAndCheckoutBranchFrom(branchName, g.Root)
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

			hooks.NewRunner(repoPath).Fire(hooks.Context{
				Event:    hooks.PostBranchCreate,
				RepoPath: repoPath,
				Branch:   branchName,
				Data:     map[string]any{"parent": g.Root},
			})

			return nil
		},
	}
	return withArgs(cmd, argDoc{"branch", "Name of the branch to create"})
}
