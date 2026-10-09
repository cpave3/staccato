package main

import (
	"github.com/cpave3/staccato/pkg/forge"
	"github.com/cpave3/staccato/pkg/graph"
	"github.com/spf13/cobra"
)

// statusNode is one branch of the tree that 'st status --json' prints: a
// stackNode with the branch's pull request.
type statusNode struct {
	Branch       string       `json:"branch"`
	Current      bool         `json:"current,omitempty"`
	PRNumber     int          `json:"pr_number,omitempty"`
	PRState      string       `json:"pr_state,omitempty"`
	PRDraft      bool         `json:"pr_draft,omitempty"`
	ReviewStatus string       `json:"review_status,omitempty"`
	CheckStatus  string       `json:"check_status,omitempty"`
	PRURL        string       `json:"pr_url,omitempty"`
	Children     []statusNode `json:"children,omitempty"`
}

func statusCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show the stack with the pull request of each branch",
		Long: `Prints the stack tree with each branch's pull request: its number, whether it
is a draft, approved, waiting for review or has changes requested, merged or
closed, and whether its checks are failing or pending.

With --json the tree is a JSON object shaped like that of 'st log --json',
where a branch that has a pull request also carries:

  "pr_number"      the pull request's number
  "pr_state"       OPEN, MERGED or CLOSED
  "pr_draft"       true for a draft
  "review_status"  APPROVED, CHANGES_REQUESTED or REVIEW_REQUIRED
  "check_status"   pass, fail or pending
  "pr_url"         the pull request's address

A field is left out when it is empty or false, so a branch without a pull
request has none of them.

Needs a GitHub remote named origin and the GitHub CLI (gh), signed in.`,
		Example: `st status
st status --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, gitRunner, printer, _, err := getContext()
			if err != nil {
				return err
			}

			if err := requireBranch(gitRunner); err != nil {
				return err
			}

			if !asJSON {
				checkStaleness(g, gitRunner, printer)
			}

			currentBranch, _ := gitRunner.GetCurrentBranch()

			f, err := forge.Detect(gitRunner)
			if err != nil {
				return err
			}

			// Collect all branch names from the graph
			var branches []string
			for name := range g.Branches {
				branches = append(branches, name)
			}

			prStatus, err := f.StackStatus(branches)
			if err != nil {
				return err
			}

			if asJSON {
				return writeJSON(cmd.OutOrStdout(), statusTree(g, g.Root, currentBranch, prStatus))
			}
			printer.StackStatus(g, currentBranch, prStatus)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print the tree as JSON")
	return cmd
}

// statusTree builds the tree of branches stacked on branch, each with its pull
// request when it has one.
func statusTree(g *graph.Graph, branch, currentBranch string, prStatus map[string]*forge.PRStatusInfo) statusNode {
	node := statusNode{Branch: branch, Current: branch == currentBranch}
	if info, ok := prStatus[branch]; ok && info.HasPR {
		node.PRNumber = info.Number
		node.PRState = info.State
		node.PRDraft = info.IsDraft
		node.ReviewStatus = info.ReviewStatus
		node.CheckStatus = info.CheckStatus
		node.PRURL = info.URL
	}
	for _, child := range childNames(g, branch) {
		node.Children = append(node.Children, statusTree(g, child, currentBranch, prStatus))
	}
	return node
}
