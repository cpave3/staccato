package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func statCmd() *cobra.Command {
	var includeUncommitted bool
	cmd := &cobra.Command{
		Use:   "stat",
		Short: "Show changes on the current branch",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, gitRunner, _, _, err := getContext()
			if err != nil {
				return err
			}

			current, err := gitRunner.GetCurrentBranch()
			if err != nil {
				return fmt.Errorf("get current branch: %w", err)
			}

			branch, ok := g.GetBranch(current)
			if !ok {
				return fmt.Errorf("branch %q has no stack parent", current)
			}

			ref := branch.Parent + "...HEAD"
			if includeUncommitted {
				base, err := gitRunner.GetMergeBase(branch.Parent, "HEAD")
				if err != nil {
					return fmt.Errorf("get merge base: %w", err)
				}
				ref = base
			}

			stat, err := gitRunner.Run("diff", "--shortstat", ref)
			if err != nil {
				return fmt.Errorf("get diff stat: %w", err)
			}

			fmt.Fprintln(cmd.OutOrStdout(), stat)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&includeUncommitted, "include-uncommitted", "i", false, "Include staged and unstaged changes")
	return cmd
}
