package main

import (
	"fmt"
	"sort"

	"github.com/cpave3/staccato/pkg/git"
	"github.com/cpave3/staccato/pkg/graph"
	"github.com/cpave3/staccato/pkg/restack"
	"github.com/spf13/cobra"
)

func graphRepairCmd() *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "repair",
		Short: "Correct the base and head recorded for the stack's branches",
		Long: `For every branch of the current stack, compares the base and head commits the
graph records with the branches as they are in git, prints each correction as
'<branch>: base <sha>, head <sha>', and writes them to the graph. Use it after
restoring or moving branches with git directly. Nothing is printed when the
graph is already right. --dry-run prints the corrections without writing them.

Check out a branch of the stack first, and finish or abort any rebase. Other
stacks are not looked at. When a branch and its parent have diverged and the
original fork point cannot be told, it refuses instead of guessing.`,
		Example: `st graph repair --dry-run
st graph repair`,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, gitRunner, _, repoPath, err := getContext()
			if err != nil {
				return err
			}
			inProgress, err := gitRunner.IsRebaseInProgress()
			if err != nil {
				return err
			}
			if inProgress {
				return fmt.Errorf("finish or abort the current rebase before repairing the graph")
			}
			current, err := gitRunner.GetCurrentBranch()
			if err != nil {
				return err
			}
			if current == g.Root {
				return fmt.Errorf("check out a branch in the stack to repair")
			}
			branch, exists := g.GetBranch(current)
			if !exists {
				return fmt.Errorf("branch %s is not in the graph", current)
			}
			for branch.Parent != g.Root {
				current = branch.Parent
				branch, exists = g.GetBranch(current)
				if !exists {
					return fmt.Errorf("parent %s is not in the graph", current)
				}
			}
			names := restack.GetStackBranches(g, current)
			sort.Strings(names)
			type correction struct{ name, base, head string }
			var corrections []correction
			for _, name := range names {
				branch := g.Branches[name]
				base, head, err := repairedBranch(gitRunner, name, branch)
				if err != nil {
					return err
				}
				if base != branch.BaseSHA || head != branch.HeadSHA {
					corrections = append(corrections, correction{name, base, head})
				}
			}
			for _, fix := range corrections {
				fmt.Printf("%s: base %s, head %s\n", fix.name, fix.base, fix.head)
				if !dryRun {
					g.UpdateBranch(fix.name, fix.base, fix.head)
				}
			}
			if !dryRun && len(corrections) > 0 {
				return saveContext(g, repoPath, gitRunner)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Print the corrections without writing them")
	return cmd
}

// repairedBranch uses the recorded fork point when the parent has moved.
// A merge-base alone cannot distinguish a restored child from inherited parent commits.
func repairedBranch(r *git.Runner, name string, branch *graph.Branch) (string, string, error) {
	head, err := r.GetCommitSHA(name)
	if err != nil {
		return "", "", fmt.Errorf("cannot repair %s: %w", name, err)
	}
	parent, err := r.GetCommitSHA(branch.Parent)
	if err != nil {
		return "", "", fmt.Errorf("cannot repair %s: parent %s: %w", name, branch.Parent, err)
	}
	parentInBranch, err := r.IsAncestor(parent, head)
	if err != nil {
		return "", "", err
	}
	if parentInBranch {
		return parent, head, nil
	}
	inBranch, err := r.IsAncestor(branch.BaseSHA, head)
	if err != nil || !inBranch {
		return "", "", fmt.Errorf("cannot repair %s: parent and branch have diverged without a trustworthy base; restore the original base or reattach explicitly", name)
	}
	return branch.BaseSHA, head, nil
}
