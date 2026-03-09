package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func upCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "up",
		Short: "Navigate to the child branch",
		Long:  "Checks out the child branch of the current branch in the stack.",
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

			// Must be root or in the stack
			if currentBranch != g.Root {
				if _, exists := g.GetBranch(currentBranch); !exists {
					return fmt.Errorf("branch '%s' is not in the stack", currentBranch)
				}
			}

			children := g.GetChildren(currentBranch)
			switch len(children) {
			case 0:
				return fmt.Errorf("already at the tip of the stack")
			case 1:
				if err := gitRunner.CheckoutBranch(children[0].Name); err != nil {
					return fmt.Errorf("failed to checkout: %w", err)
				}
				printer.Success("Checked out '%s'", children[0].Name)
				return nil
			default:
				names := make([]string, len(children))
				for i, c := range children {
					names[i] = c.Name
				}
				return fmt.Errorf("multiple children: %s — use 'st switch' to select", strings.Join(names, ", "))
			}
		},
	}
}
