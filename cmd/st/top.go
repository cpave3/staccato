package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func topCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "top",
		Short: "Navigate to the tip of the stack",
		Long:  "Follows single-child links from the current branch to the tip of the stack.",
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

			// Walk to tip
			at := currentBranch
			for {
				children := g.GetChildren(at)
				switch len(children) {
				case 0:
					// Reached the tip
					if at == currentBranch {
						fmt.Println("already at the top of the stack")
						return nil
					}
					if err := gitRunner.CheckoutBranch(at); err != nil {
						return fmt.Errorf("failed to checkout: %w", err)
					}
					printer.Success("Checked out '%s'", at)
					return nil
				case 1:
					at = children[0].Name
				default:
					names := make([]string, len(children))
					for i, c := range children {
						names[i] = c.Name
					}
					return fmt.Errorf("multiple children at '%s': %s — use 'st switch' to select", at, strings.Join(names, ", "))
				}
			}
		},
	}
}
