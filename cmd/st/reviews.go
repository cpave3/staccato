package main

import (
	"fmt"
	"os"

	"github.com/cpave3/staccato/pkg/forge"
	"github.com/cpave3/staccato/pkg/reviews"
	"github.com/spf13/cobra"
)

func reviewsCmd() *cobra.Command {
	var (
		current   bool
		toCurrent bool
		outPath   string
	)

	cmd := &cobra.Command{
		Use:   "reviews",
		Short: "Collect the stack's pull request feedback as Markdown",
		Long: `Fetches the inline comments, review submissions and general comments on the
pull requests of the stack's branches and prints them as one Markdown
document, with a section for each pull request.

By default it covers the current lineage. --current limits it to the current
branch, --to-current to the branches from the trunk up to the current one.
--out writes the document to a file instead of standard output.

Comments from bot accounts (logins ending in [bot]) are left out, except those
of the review bots coderabbitai[bot], cubic-dev-ai[bot] and greptile-apps[bot].

Needs a GitHub remote named origin and the GitHub CLI (gh), signed in.`,
		Example: `st reviews
st reviews --current
st reviews --to-current --out feedback.md`,
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

			// Determine scope
			scope := reviews.ScopeAll
			if current {
				scope = reviews.ScopeCurrent
			} else if toCurrent {
				scope = reviews.ScopeToCurrent
			}

			// Resolve branches
			branches := reviews.ResolveBranches(g, currentBranch, scope)
			if len(branches) == 0 {
				fmt.Fprintln(os.Stderr, "no branches in scope")
				return nil
			}

			// Detect forge and get PR numbers
			f, err := forge.Detect(gitRunner)
			if err != nil {
				return err
			}

			prStatus, err := f.StackStatus(branches)
			if err != nil {
				return fmt.Errorf("failed to get PR status: %w", err)
			}

			// Build PR map (branch -> PR number) for branches with open PRs
			prs := make(map[string]int)
			for _, branch := range branches {
				info, ok := prStatus[branch]
				if ok && info.HasPR && info.Number > 0 {
					prs[branch] = info.Number
				}
			}

			if len(prs) == 0 {
				fmt.Fprintln(os.Stderr, "no PRs found for branches in scope")
				return nil
			}

			// Get owner/repo from remote URL
			remoteURL, err := gitRunner.GetRemoteURL("origin")
			if err != nil {
				return fmt.Errorf("failed to get remote URL: %w", err)
			}
			owner, repo, err := reviews.ParseRemoteURL(remoteURL)
			if err != nil {
				return err
			}

			// Fetch all reviews
			items, err := reviews.FetchAll(owner, repo, prs, 5)
			if err != nil {
				return fmt.Errorf("failed to fetch reviews: %w", err)
			}

			// Thread replies and filter noise
			items = reviews.ThreadReplies(items)
			items = reviews.FilterNoise(items)

			// Format output
			result := reviews.ReviewResult{
				Items:     items,
				Scope:     scope,
				RepoOwner: owner,
				RepoName:  repo,
			}
			md := reviews.FormatMarkdown(result)

			// Output
			if outPath != "" {
				if err := os.WriteFile(outPath, []byte(md), 0644); err != nil {
					return fmt.Errorf("failed to write output: %w", err)
				}
				fmt.Fprintf(os.Stderr, "wrote feedback to %s\n", outPath)
			} else {
				fmt.Print(md)
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&current, "current", false, "Only the current branch's pull request")
	cmd.Flags().BoolVar(&toCurrent, "to-current", false, "Only the branches from the trunk up to the current one")
	cmd.Flags().StringVar(&outPath, "out", "", "Write the document to `path` instead of standard output")

	return cmd
}
