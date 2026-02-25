package main

import (
	"github.com/spf13/cobra"
	"github.com/cpave3/staccato/pkg/forge"
)

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show PR status for the entire stack",
		Long:  "Displays the stack tree with PR status annotations for each branch.",
		RunE: func(cmd *cobra.Command, args []string) error {
			g, gitRunner, printer, _, err := getContext()
			if err != nil {
				return err
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

			printer.StackStatus(g, currentBranch, prStatus)

			return nil
		},
	}
}
