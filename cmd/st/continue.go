package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/cpave3/staccato/pkg/attach"
	"github.com/cpave3/staccato/pkg/backup"
	"github.com/cpave3/staccato/pkg/restack"
)

func continueCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "continue",
		Short: "Resume restack after conflict resolution",
		Long:  "Continues a restack operation that was paused due to conflicts.",
		RunE: func(cmd *cobra.Command, args []string) error {
			g, git, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			// Check if rebase is in progress
			inProgress, err := git.IsRebaseInProgress()
			if err != nil {
				return fmt.Errorf("failed to check rebase status: %w", err)
			}

			if !inProgress {
				return fmt.Errorf("no rebase in progress - nothing to continue")
			}

			backupMgr := backup.NewManager(git, repoPath)
			engine := restack.NewEngine(git, backupMgr)

			currentBranch, _ := git.GetCurrentBranch()
			attacher := attach.NewAttacher(git, printer)
			rootBranch := attacher.FindRoot(g, currentBranch)
			if rootBranch == "" {
				rootBranch = g.Root
			}

			// Continue the restack
			result, err := engine.Continue(g, rootBranch, nil)

			// Save graph state
			saveContext(g, repoPath)

			if err != nil {
				if result.Conflicts {
					printer.ConflictDetected(result.ConflictsAt)
					return fmt.Errorf("still have conflicts to resolve")
				}
				return err
			}

			printer.RestackComplete(len(result.Completed))

			return nil
		},
	}
}
