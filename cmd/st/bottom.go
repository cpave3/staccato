package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

func bottomCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "bottom",
		Short: "Navigate to the bottom of the stack",
		Long:  "Navigates to the first tracked branch above root in the current lineage.",
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

			// If on root, go to single child
			if currentBranch == g.Root {
				children := g.GetChildren(g.Root)
				switch len(children) {
				case 0:
					return fmt.Errorf("no branches in the stack")
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
					return fmt.Errorf("multiple children at root: %s — use 'st switch' to select", strings.Join(names, ", "))
				}
			}

			// Walk up to find the first child of root
			target := currentBranch
			for {
				b, exists := g.GetBranch(target)
				if !exists {
					return fmt.Errorf("branch '%s' is not in the stack", target)
				}
				if b.Parent == g.Root {
					break
				}
				target = b.Parent
			}

			if target == currentBranch {
				fmt.Println("already at the bottom of the stack")
				return nil
			}

			if err := gitRunner.CheckoutBranch(target); err != nil {
				return fmt.Errorf("failed to checkout: %w", err)
			}
			printer.Success("Checked out '%s'", target)
			return nil
		},
	}
}
