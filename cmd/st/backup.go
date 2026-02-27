package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/cpave3/staccato/pkg/backup"
)

func backupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Create a manual backup or manage backups",
		Long: `Creates a manual snapshot of every branch in the stack (excluding the root).
Unlike automatic backups created during restack/insert, manual backups persist
until you delete them.

Subcommands:
  list   List all backup branches
  clean  Interactively select and delete old backups`,
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
		Short: "List all backup branches",
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
