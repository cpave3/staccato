package main

import (
	"fmt"

	"github.com/cpave3/staccato/pkg/backup"
	"github.com/cpave3/staccato/pkg/restack"
	"github.com/spf13/cobra"
)

func modifyCmd() *cobra.Command {
	var all bool
	var message string

	cmd := &cobra.Command{
		Use:   "modify",
		Short: "Amend the current branch and restack downstream",
		Long:  "Amends the HEAD commit with staged changes (or all changes with --all) and restacks downstream branches.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, gitRunner, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			currentBranch, err := gitRunner.GetCurrentBranch()
			if err != nil {
				return fmt.Errorf("failed to get current branch: %w", err)
			}

			// Must be in the stack (not root alone)
			if currentBranch == g.Root {
				return fmt.Errorf("cannot modify root branch")
			}
			if _, exists := g.GetBranch(currentBranch); !exists {
				return fmt.Errorf("branch '%s' is not in the stack", currentBranch)
			}

			// Stage all if --all
			if all {
				if _, err := gitRunner.Run("add", "-A"); err != nil {
					return fmt.Errorf("failed to stage changes: %w", err)
				}
			}

			// Check if there's anything to amend
			if message == "" {
				hasChanges, err := gitRunner.HasUncommittedChanges()
				if err != nil {
					return fmt.Errorf("failed to check changes: %w", err)
				}
				// Check staged specifically
				staged, _ := gitRunner.Diff(true, nil)
				if !hasChanges && staged == "" {
					return fmt.Errorf("nothing to modify — stage changes or use --all or --message")
				}
			}

			// Amend
			amendArgs := []string{"commit", "--amend", "--no-edit"}
			if message != "" {
				amendArgs = []string{"commit", "--amend", "-m", message}
			}
			if _, err := gitRunner.Run(amendArgs...); err != nil {
				return fmt.Errorf("failed to amend commit: %w", err)
			}

			// Update current branch SHA in graph
			newHead, _ := gitRunner.GetCommitSHA(currentBranch)
			b, _ := g.GetBranch(currentBranch)
			g.UpdateBranch(currentBranch, b.BaseSHA, newHead)

			// Restack downstream if any
			downstream := restack.GetDownstreamBranches(g, currentBranch)
			if len(downstream) > 0 {
				allBranches := append([]string{currentBranch}, downstream...)
				backupMgr := backup.NewManager(gitRunner, repoPath)
				engine := restack.NewEngine(gitRunner, backupMgr)
				result, restackErr := engine.RestackLineage(g, currentBranch, allBranches)

				if err := saveContext(g, repoPath, gitRunner); err != nil {
					return fmt.Errorf("failed to save graph: %w", err)
				}

				if restackErr != nil {
					if result != nil && result.Conflicts {
						printer.ConflictDetected(result.ConflictsAt)
						return fmt.Errorf("conflict at '%s' — resolve and run 'st continue'", result.ConflictsAt)
					}
					return fmt.Errorf("restack failed: %w", restackErr)
				}

				// Clean up backups on success
				if result != nil && len(result.Backups) > 0 {
					backupMgr.CleanupStackBackups(allBranches)
				}

				// Checkout back to the modified branch
				gitRunner.CheckoutBranch(currentBranch)
				printer.Success("Modified '%s' and restacked %d downstream branches", currentBranch, len(result.Completed))
			} else {
				if err := saveContext(g, repoPath, gitRunner); err != nil {
					return fmt.Errorf("failed to save graph: %w", err)
				}
				printer.Success("Modified '%s'", currentBranch)
			}

			return nil
		},
	}

	cmd.Flags().BoolVarP(&all, "all", "a", false, "Stage all changes before amending")
	cmd.Flags().StringVarP(&message, "message", "m", "", "Update commit message")

	return cmd
}
