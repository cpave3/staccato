package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/cpave3/staccato/pkg/forge"
)

func prCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pr",
		Short: "Interact with pull requests",
		Long:  "Commands for creating and viewing pull requests on your hosting provider.",
	}
	cmd.AddCommand(prMakeCmd())
	cmd.AddCommand(prViewCmd())
	return cmd
}

func prMakeCmd() *cobra.Command {
	var web bool

	cmd := &cobra.Command{
		Use:   "make",
		Short: "Create a PR for the current branch",
		Long: `Creates a pull request targeting the parent branch in the stack.
This ensures PRs in a stack are chained correctly.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, gitRunner, _, _, err := getContext()
			if err != nil {
				return err
			}

			currentBranch, err := gitRunner.GetCurrentBranch()
			if err != nil {
				return fmt.Errorf("failed to get current branch: %w", err)
			}

			branchInfo, exists := g.GetBranch(currentBranch)
			if !exists {
				return fmt.Errorf("branch '%s' is not in the stack — run 'st attach' first", currentBranch)
			}

			f, err := forge.Detect(gitRunner)
			if err != nil {
				return err
			}

			if !gitRunner.RemoteBranchExists(currentBranch) {
				fmt.Printf("Branch '%s' has not been pushed — pushing now...\n", currentBranch)
				if err := gitRunner.Push(currentBranch, false); err != nil {
					return fmt.Errorf("failed to push branch: %w", err)
				}
			}

			return f.CreatePR(forge.PRCreateOpts{
				Head: currentBranch,
				Base: branchInfo.Parent,
				Web:  web,
			})
		},
	}

	cmd.Flags().BoolVarP(&web, "web", "w", false, "Open in browser")
	return cmd
}

func prViewCmd() *cobra.Command {
	var web bool

	cmd := &cobra.Command{
		Use:   "view",
		Short: "View the PR for the current branch",
		Long:  "Shows the pull request associated with the current branch.",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, gitRunner, _, _, err := getContext()
			if err != nil {
				return err
			}

			f, err := forge.Detect(gitRunner)
			if err != nil {
				return err
			}

			return f.ViewPR(forge.PRViewOpts{
				Web: web,
			})
		},
	}

	cmd.Flags().BoolVarP(&web, "web", "w", false, "Open in browser")
	return cmd
}
