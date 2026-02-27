package main

import (
	"github.com/spf13/cobra"
)

func logCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "log",
		Short: "Display stack hierarchy",
		Long:  "Shows the current stack structure as a tree.",
		RunE: func(cmd *cobra.Command, args []string) error {
			g, git, printer, _, err := getContext()
			if err != nil {
				return err
			}

			checkStaleness(g, git, printer)

			currentBranch, _ := git.GetCurrentBranch()

			printer.StackLog(g, currentBranch)

			return nil
		},
	}
}
