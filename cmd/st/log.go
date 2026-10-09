package main

import (
	"sort"

	"github.com/cpave3/staccato/pkg/graph"
	"github.com/spf13/cobra"
)

// stackNode is one branch of the tree that 'st log --json' prints.
type stackNode struct {
	Branch   string      `json:"branch"`
	Current  bool        `json:"current,omitempty"`
	Children []stackNode `json:"children,omitempty"`
}

func logCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Show the stack as a tree",
		Long: `Prints every branch st tracks as a tree under the trunk, each branch indented
below its parent. The current branch is marked with a filled dot.

With --json the tree is a JSON object, starting at the trunk:

  {
    "branch": "main",
    "children": [
      {"branch": "feature/api", "current": true,
       "children": [{"branch": "feature/frontend"}]}
    ]
  }

"children" are the branches stacked directly on a branch, in name order, so a
branch's parent is the node that contains it. "current": true marks the
checked-out branch; no node has it when that branch is not in the stack.`,
		Example: `st log
st log --json`,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, git, printer, _, err := getContext()
			if err != nil {
				return err
			}

			if err := requireBranch(git); err != nil {
				return err
			}

			currentBranch, _ := git.GetCurrentBranch()
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), stackTree(g, g.Root, currentBranch))
			}

			checkStaleness(g, git, printer)
			printer.StackLog(g, currentBranch)
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print the tree as JSON")
	return cmd
}

// stackTree builds the tree of branches stacked on branch.
func stackTree(g *graph.Graph, branch, currentBranch string) stackNode {
	node := stackNode{Branch: branch, Current: branch == currentBranch}
	for _, child := range childNames(g, branch) {
		node.Children = append(node.Children, stackTree(g, child, currentBranch))
	}
	return node
}

// childNames returns the names of a branch's children in a stable order.
func childNames(g *graph.Graph, parent string) []string {
	var names []string
	for _, child := range g.GetChildren(parent) {
		names = append(names, child.Name)
	}
	sort.Strings(names)
	return names
}
