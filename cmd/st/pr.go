package main

import (
	"fmt"

	"github.com/cpave3/staccato/pkg/forge"
	"github.com/cpave3/staccato/pkg/hooks"
	"github.com/spf13/cobra"
)

func prCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pr",
		Short: "Create and view pull requests",
		Example: `st pr make --title "feat: add order API" --description "Adds the order API."
st pr view`,
	}
	cmd.AddCommand(prMakeCmd())
	cmd.AddCommand(prViewCmd())
	return cmd
}

func prMakeCmd() *cobra.Command {
	var web bool
	var title string
	var description string

	cmd := &cobra.Command{
		Use:   "make",
		Short: "Open a pull request from the current branch into its parent",
		Long: `Creates a pull request whose base is the current branch's parent in the stack,
so that the pull requests of a stack chain in the same order as its branches.
Run it on each branch, from the bottom of the stack up. The branch is pushed
first if origin does not have it.

Uses the GitHub CLI (gh). Without --title, gh asks for the title and body
interactively, which needs a terminal: scripts and agents should pass --title
and --description. --web opens the form in a browser instead.`,
		Example: `st pr make --title "feat: add order API" --description "Adds the order API."
st pr make --web`,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, gitRunner, _, repoPath, err := getContext()
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

			if err := f.CreatePR(forge.PRCreateOpts{
				Head:        currentBranch,
				Base:        branchInfo.Parent,
				Web:         web,
				Title:       title,
				Description: description,
			}); err != nil {
				return err
			}

			hooks.NewRunner(repoPath).Fire(hooks.Context{
				Event:    hooks.PostPRCreate,
				RepoPath: repoPath,
				Branch:   currentBranch,
				Data:     map[string]any{"base": branchInfo.Parent, "web": web, "title": title, "description": description},
			})

			return nil
		},
	}

	cmd.Flags().BoolVarP(&web, "web", "w", false, "Open the pull request form in the browser")
	cmd.Flags().StringVarP(&title, "title", "t", "", "Pull request title `text`")
	cmd.Flags().StringVarP(&description, "description", "d", "", "Pull request body `text`")
	return cmd
}

func prViewCmd() *cobra.Command {
	var web bool

	cmd := &cobra.Command{
		Use:   "view",
		Short: "Show the pull request of the current branch",
		Long: `Prints the current branch's pull request, as 'gh pr view' does. --web opens it
in the browser instead. Needs the GitHub CLI (gh), signed in.`,
		Example: `st pr view
st pr view --web`,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, gitRunner, _, repoPath, err := getContext()
			if err != nil {
				return err
			}

			currentBranch, err := gitRunner.GetCurrentBranch()
			if err != nil {
				return fmt.Errorf("failed to get current branch: %w", err)
			}

			f, err := forge.Detect(gitRunner)
			if err != nil {
				return err
			}

			if err := f.ViewPR(forge.PRViewOpts{
				Web: web,
			}); err != nil {
				return err
			}

			hooks.NewRunner(repoPath).Fire(hooks.Context{
				Event:    hooks.PostPRView,
				RepoPath: repoPath,
				Branch:   currentBranch,
				Data:     map[string]any{"web": web},
			})

			return nil
		},
	}

	cmd.Flags().BoolVarP(&web, "web", "w", false, "Open the pull request in the browser")
	return cmd
}
