package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func downCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "down",
		Short: "Navigate to the parent branch",
		Long:  "Checks out the parent branch of the current branch in the stack.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, gitRunner, printer, _, err := getContext()
			if err != nil {
				return err
			}

			currentBranch, err := gitRunner.GetCurrentBranch()
			if err != nil {
				return fmt.Errorf("failed to get current branch: %w", err)
			}

			// At root — nowhere to go
			if currentBranch == g.Root {
				return fmt.Errorf("already at the bottom of the stack")
			}

			b, exists := g.GetBranch(currentBranch)
			if !exists {
				return fmt.Errorf("branch '%s' is not in the stack", currentBranch)
			}

			if err := gitRunner.CheckoutBranch(b.Parent); err != nil {
				return fmt.Errorf("failed to checkout: %w", err)
			}
			printer.Success("Checked out '%s'", b.Parent)
			return nil
		},
	}
}
