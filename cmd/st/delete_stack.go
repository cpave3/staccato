package main

import (
	"fmt"
	"strings"

	"github.com/cpave3/staccato/pkg/restack"
	"github.com/spf13/cobra"
)

func deleteStackCmd() *cobra.Command {
	var branches bool
	var force bool

	cmd := &cobra.Command{
		Use:   "delete-stack",
		Short: "Remove the current lineage from the stack graph",
		Long: `Removes all non-root branches in the current lineage from the stack graph.
By default, git branches are left intact (graph-only removal).
Use --branches to also delete the git branches.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, gitRunner, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			if err := requireBranch(gitRunner); err != nil {
				return err
			}

			currentBranch, _ := gitRunner.GetCurrentBranch()

			// Cannot run on root branch
			if currentBranch == g.Root {
				return fmt.Errorf("cannot delete-stack while on the root branch '%s'", g.Root)
			}

			// Compute lineage and filter out root
			lineage := restack.GetLineage(g, currentBranch)
			var toRemove []string
			for _, b := range lineage {
				if b != g.Root {
					toRemove = append(toRemove, b)
				}
			}

			if len(toRemove) == 0 {
				printer.Println("No branches to remove.")
				return nil
			}

			// Print summary
			printer.Println("Removing %d branch(es) from stack: %s", len(toRemove), strings.Join(toRemove, ", "))

			// When --branches is set, check for unpushed branches
			if branches {
				hasRemote, _ := gitRunner.HasRemote()
				if hasRemote && !force {
					var unpushed []string
					for _, b := range toRemove {
						if !gitRunner.RemoteBranchExists(b) {
							unpushed = append(unpushed, b)
						}
					}
					if len(unpushed) > 0 {
						return fmt.Errorf("branches not pushed to remote: %s\nUse --force to delete anyway", strings.Join(unpushed, ", "))
					}
				}
			}

			// Remove from graph in reverse order (leaves first)
			for i := len(toRemove) - 1; i >= 0; i-- {
				g.RemoveBranch(toRemove[i])
			}

			// Save graph
			if err := saveContext(g, repoPath, gitRunner); err != nil {
				return fmt.Errorf("failed to save graph: %w", err)
			}

			// Check out root
			if err := gitRunner.CheckoutBranch(g.Root); err != nil {
				return fmt.Errorf("failed to checkout root branch '%s': %w", g.Root, err)
			}

			// Delete git branches if requested
			if branches {
				for _, b := range toRemove {
					if err := gitRunner.DeleteBranch(b, true); err != nil {
						printer.Warning("failed to delete git branch '%s': %v", b, err)
					}
				}
			}

			printer.Success("Deleted stack (%d branches removed), now on '%s'", len(toRemove), g.Root)
			return nil
		},
	}

	cmd.Flags().BoolVar(&branches, "branches", false, "Also delete the git branches")
	cmd.Flags().BoolVarP(&force, "force", "f", false, "Force delete even if branches have unpushed commits")

	return cmd
}
