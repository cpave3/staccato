package main

import (
	"fmt"

	"github.com/cpave3/staccato/pkg/backup"
	"github.com/spf13/cobra"
)

func backupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Snapshot every branch of the stack",
		Long: `Copies every branch st tracks to a backup branch named
backup/manual/<date>_<time>/<branch>. Unlike the automatic backups that
restacking makes and removes again, these stay until you delete them.

The snapshot's name is printed only with -v; 'st backup list' shows it either
way. To go back to a snapshot, point the branch at its backup with git:
'git branch -f <branch> backup/manual/<date>_<time>/<branch>', from another
branch. 'st restore' does not use these snapshots.`,
		Example: `st backup
st backup -v`,
		RunE: func(cmd *cobra.Command, args []string) error {
			g, gitRunner, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			// Collect branch names excluding root
			var branches []string
			for name := range g.Branches {
				if name != g.Root {
					branches = append(branches, name)
				}
			}

			if len(branches) == 0 {
				return fmt.Errorf("no branches in the stack to backup")
			}

			backupMgr := backup.NewManager(gitRunner, repoPath)
			timestamp, err := backupMgr.CreateManualBackup(branches)
			if err != nil {
				return fmt.Errorf("backup failed: %w", err)
			}

			printer.Info("Backup created: %s (%d branches)", timestamp, len(branches))
			return nil
		},
	}

	cmd.AddCommand(backupListCmd())
	cmd.AddCommand(backupCleanCmd())

	return cmd
}

func backupListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the backup branches",
		Long: `Lists every backup, automatic and manual, with its kind, when it was taken and
the branch it was taken from.`,
		Example: `st backup list`,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, gitRunner, printer, repoPath, err := getContext()
			if err != nil {
				return err
			}

			backupMgr := backup.NewManager(gitRunner, repoPath)
			all, err := backupMgr.ListAllBackups()
			if err != nil {
				return err
			}

			if len(all) == 0 {
				printer.Println("No backups found")
				return nil
			}

			printer.Println("Found %d backup(s):", len(all))
			for _, b := range all {
				printer.Println("  [%s] %s  %s", b.Kind, b.Timestamp.Format("2006-01-02 15:04:05"), b.SourceBranch)
			}

			return nil
		},
	}
}
