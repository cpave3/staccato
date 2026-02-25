package main

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/cpave3/staccato/pkg/backup"
)

func backupCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "backup",
		Short: "Create a manual backup of all stack branches",
		Long: `Creates a manual snapshot of every branch in the stack (excluding the root).
Unlike automatic backups created during restack/insert, manual backups persist
until you delete them. Backup branches are named backups/<timestamp>/<branch>.`,
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
}
