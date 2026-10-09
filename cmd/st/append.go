package main

import (
	"fmt"

	"github.com/cpave3/staccato/pkg/hooks"
	"github.com/spf13/cobra"
)

func appendCmd() *cobra.Command {
	return withArgs(&cobra.Command{
		Use:   "append <branch>",
		Short: "Create a branch stacked on top of the current branch",
		Long: `Creates <branch> at the current branch's tip, records the current branch as
its parent, and checks it out. Uncommitted changes come along, so commit the
current branch's work first.

The current branch has to be the trunk or a branch in the stack; add one with
'st attach'. From the trunk this does the same as 'st new'.`,
		Example: `st append feature/frontend`,
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

			parentBranch, _ := git.GetCurrentBranch()

			// If current branch is not in graph and not root, we need to attach it first
			if parentBranch != g.Root {
				if _, exists := g.GetBranch(parentBranch); !exists {
					return fmt.Errorf("current branch '%s' is not in the stack. Run 'st attach' first", parentBranch)
				}
			}

			// Check if branch already exists
			if exists, _ := git.BranchExists(branchName); exists {
				return fmt.Errorf("branch '%s' already exists — use 'st attach %s' to add it to the stack", branchName, branchName)
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

			hooks.NewRunner(repoPath).Fire(hooks.Context{
				Event:    hooks.PostBranchCreate,
				RepoPath: repoPath,
				Branch:   branchName,
				Data:     map[string]any{"parent": parentBranch},
			})

			return nil
		},
	}, argDoc{"branch", "Name of the branch to create"})
}
