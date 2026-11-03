package main

import (
	"fmt"
	"io"
	"strings"

	"github.com/cpave3/staccato/pkg/forge"
	"github.com/cpave3/staccato/pkg/graph"
	"github.com/cpave3/staccato/pkg/hooks"
	"github.com/cpave3/staccato/pkg/restack"
	"github.com/spf13/cobra"
)

func prCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pr",
		Short: "Create, view and list pull requests",
		Example: `st pr make --title "feat: add order API" --description "Adds the order API."
st pr view
st pr list`,
	}
	cmd.AddCommand(prMakeCmd())
	cmd.AddCommand(prViewCmd())
	cmd.AddCommand(prListCmd())
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

func prListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List the pull request link of each branch in the stack",
		Long: `Prints one line for each branch of the current lineage, from the bottom of the
stack up: the branch name, then the address of its pull request, or "No PR"
when it has none. A pull request that was merged or closed is marked
(merged) or (closed).

For the stack tree with each pull request's review and check status, use
'st status'; 'st status --json' gives the same addresses as JSON.

Needs a GitHub remote named origin and the GitHub CLI (gh), signed in.`,
		Example: `st pr list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, gitRunner, _, _, err := getContext()
			if err != nil {
				return err
			}

			if err := requireBranch(gitRunner); err != nil {
				return err
			}

			currentBranch, err := gitRunner.GetCurrentBranch()
			if err != nil {
				return fmt.Errorf("failed to get current branch: %w", err)
			}

			if _, tracked := g.GetBranch(currentBranch); !tracked && currentBranch != g.Root {
				return fmt.Errorf("branch '%s' is not in the stack — run 'st attach' first", currentBranch)
			}

			branches := prLineage(g, currentBranch)
			if len(branches) == 0 {
				fmt.Fprintln(cmd.OutOrStdout(), "no branches in the stack")
				return nil
			}

			f, err := forge.Detect(gitRunner)
			if err != nil {
				return err
			}

			prStatus, err := f.StackStatus(branches)
			if err != nil {
				return err
			}

			printPRList(cmd.OutOrStdout(), branches, prStatus)
			return nil
		},
	}
	return cmd
}

// prLineage returns the branches of branch's lineage that can have a pull
// request, from the bottom of the stack up, with siblings in name order.
func prLineage(g *graph.Graph, branch string) []string {
	var branches []string
	for _, name := range restack.GetAncestors(g, branch) {
		if name != g.Root {
			branches = append(branches, name)
		}
	}
	var descend func(parent string)
	descend = func(parent string) {
		for _, child := range childNames(g, parent) {
			branches = append(branches, child)
			descend(child)
		}
	}
	descend(branch)
	return branches
}

// printPRList prints one line per branch: its name, then its pull request's
// address, or "No PR" when it has none.
func printPRList(w io.Writer, branches []string, prStatus map[string]*forge.PRStatusInfo) {
	width := 0
	for _, b := range branches {
		width = max(width, len(b))
	}
	for _, b := range branches {
		link := "No PR"
		if info, ok := prStatus[b]; ok && info.HasPR {
			link = info.URL
			if info.State != "OPEN" {
				link += " (" + strings.ToLower(info.State) + ")"
			}
		}
		fmt.Fprintf(w, "%s%s  %s\n", b, strings.Repeat(" ", width-len(b)), link)
	}
}
