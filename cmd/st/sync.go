package main

import (
	"fmt"

	"github.com/spf13/cobra"
	stcontext "github.com/cpave3/staccato/pkg/context"
	stync "github.com/cpave3/staccato/pkg/sync"
)

func syncCmd() *cobra.Command {
	var dryRun bool
	var downOnly bool

	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Fetch, detect merged branches, restack & push",
		Long: `Fetches from remote, detects branches merged on GitHub, removes them from
the stack graph (reparenting children), restacks remaining branches, and pushes.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, gitRunner, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			if err := requireBranch(gitRunner); err != nil {
				return err
			}

			if dryRun {
				printer.DryRunNotice()
			}

			printer.SyncFetching()

			sc := stcontext.NewContext(g, gitRunner, repoPath)

			result, err := stync.Run(sc, stync.Options{
				DryRun:   dryRun,
				DownOnly: downOnly,
			})

			if dryRun && result != nil {
				if result.TrunkUpdated {
					fmt.Printf("Would fast-forward '%s'\n", g.Root)
				}
				for _, branch := range result.MergedBranches {
					fmt.Printf("Would remove merged branch: %s\n", branch)
				}
				if len(result.MergedBranches) > 0 {
					fmt.Println("Would restack remaining branches")
				}
				for _, branch := range result.PushedBranches {
					fmt.Printf("Would push: %s\n", branch)
				}
				if len(result.PushedBranches) == 0 && len(result.MergedBranches) == 0 {
					fmt.Println("Nothing to do.")
				}
				return nil
			}

			if result != nil {
				if result.TrunkUpdated {
					printer.SyncTrunkUpdated(g.Root)
				}
				for _, branch := range result.MergedBranches {
					printer.SyncMergedDetected(branch)
				}
				if len(result.MergedBranches) == 0 {
					printer.SyncNoMergedBranches()
				}
				if result.StashedFromBranch != "" {
					printer.Warning("Stashing uncommitted changes from merged branch '%s'", result.StashedFromBranch)
				}
				for _, branch := range result.MergedBranches {
					printer.SyncBranchRemoved(branch)
				}

				if result.Conflicts {
					printer.ConflictDetected(result.ConflictsAt)
				}

				pushedCount := len(result.PushedBranches)
				for _, branch := range result.PushedBranches {
					printer.Info("Pushed: %s", branch)
				}

				printer.SyncSummary(len(result.MergedBranches), result.RestackedCount, pushedCount)
			}

			if err != nil {
				return err
			}

			return nil
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Show what would be pushed without pushing")
	cmd.Flags().BoolVar(&downOnly, "down", false, "Only pull changes from remote, skip pushing")

	return cmd
}
