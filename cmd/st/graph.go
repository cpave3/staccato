package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/cpave3/staccato/pkg/graph"
)

func graphCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graph",
		Short: "Manage the stack graph storage mode",
		Long:  "Commands for sharing or localizing the stack graph.",
	}
	cmd.AddCommand(graphShareCmd())
	cmd.AddCommand(graphLocalCmd())
	cmd.AddCommand(graphWhichCmd())
	return cmd
}

func graphShareCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "share",
		Short: "Share the graph via a git ref (pushable/fetchable)",
		Long:  "Moves the local graph into a git ref so it can be pushed and fetched by teammates.",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, gitRunner, _, repoPath, err := getContext()
			if err != nil {
				return err
			}

			email, err := gitRunner.GetUserEmail()
			if err != nil {
				return fmt.Errorf("git user.email not configured — set it with: git config user.email you@example.com")
			}
			userRef := graph.UserGraphRef(email)

			if gitRunner.RefExists(userRef) {
				return fmt.Errorf("graph is already shared (stored at %s)", userRef)
			}

			localPath := filepath.Join(repoPath, graph.DefaultGraphPath)
			data, err := os.ReadFile(localPath)
			if err != nil {
				return fmt.Errorf("no local graph found at %s", graph.DefaultGraphPath)
			}

			// Validate it's valid JSON
			var g graph.Graph
			if err := json.Unmarshal(data, &g); err != nil {
				return fmt.Errorf("local graph is invalid: %w", err)
			}

			if err := gitRunner.WriteBlobRef(userRef, data); err != nil {
				return fmt.Errorf("failed to write shared ref: %w", err)
			}

			if err := os.Remove(localPath); err != nil {
				fmt.Printf("Warning: could not remove local file: %v\n", err)
			}

			// Configure fetch refspec if remote exists
			hasRemote, _ := gitRunner.HasRemote()
			if hasRemote {
				refspec := "+refs/staccato/*:refs/staccato/*"
				if !gitRunner.HasFetchRefspec("refs/staccato") {
					gitRunner.AddFetchRefspec(refspec)
				}
			}

			fmt.Println("Graph shared. Push with `st sync` or `git push origin " + userRef + "`")
			return nil
		},
	}
}

func graphLocalCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "local",
		Short: "Move the graph back to local-only storage",
		Long:  "Moves the shared graph ref back to the local file, removing the git ref.",
		RunE: func(cmd *cobra.Command, args []string) error {
			sc, err := loadStaccatoContext()
			if err != nil {
				return err
			}

			if !sc.IsShared() {
				return fmt.Errorf("graph is already local (no shared ref found)")
			}

			data, err := sc.Git.ReadBlobRef(sc.SharedRef())
			if err != nil {
				return fmt.Errorf("failed to read shared ref: %w", err)
			}

			localPath := filepath.Join(sc.RepoPath, graph.DefaultGraphPath)
			dir := filepath.Dir(localPath)
			if err := os.MkdirAll(dir, 0755); err != nil {
				return fmt.Errorf("failed to create directory: %w", err)
			}
			if err := os.WriteFile(localPath, data, 0644); err != nil {
				return fmt.Errorf("failed to write local graph: %w", err)
			}

			if err := sc.Git.DeleteRef(sc.SharedRef()); err != nil {
				return fmt.Errorf("failed to delete shared ref: %w", err)
			}

			// Also clean up legacy ref if it exists
			if sc.Git.RefExists(graph.SharedGraphRefLegacy) {
				sc.Git.DeleteRef(graph.SharedGraphRefLegacy)
			}

			// Remove fetch refspec if remote exists
			hasRemote, _ := sc.Git.HasRemote()
			if hasRemote {
				refspec := "+refs/staccato/*:refs/staccato/*"
				if sc.Git.HasFetchRefspec("refs/staccato") {
					sc.Git.RemoveFetchRefspec(refspec)
				}
			}

			fmt.Printf("Graph moved to local storage (%s)\n", graph.DefaultGraphPath)
			return nil
		},
	}
}

func graphWhichCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "which",
		Short: "Show current graph storage mode",
		RunE: func(cmd *cobra.Command, args []string) error {
			sc, err := loadStaccatoContext()
			if err != nil {
				return err
			}

			if sc.IsShared() {
				fmt.Printf("Shared (%s)\n", sc.SharedRef())
			} else {
				fmt.Printf("Local (%s)\n", graph.DefaultGraphPath)
			}
			return nil
		},
	}
}
