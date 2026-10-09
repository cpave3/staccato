package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/cpave3/staccato/pkg/graph"
	"github.com/spf13/cobra"
)

func graphCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "graph",
		Short: "Store, share and repair the stack graph",
		Long: `The graph is st's record of which branch is stacked on which, with the commit
each branch was last based on. It is kept in a file inside .git unless shared.`,
		Example: `st graph which
st graph share
st graph repair --dry-run`,
	}
	cmd.AddCommand(graphShareCmd())
	cmd.AddCommand(graphLocalCmd())
	cmd.AddCommand(graphWhichCmd())
	cmd.AddCommand(graphRepairCmd())
	return cmd
}

func graphShareCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "share",
		Short: "Keep the graph in a git ref that can be pushed and fetched",
		Long: `Moves the graph from the local file (.git/stack/graph.json) into a git ref of
your own under refs/staccato/, derived from your git user.email, and sets
origin up to fetch refs/staccato/*. 'st sync' then pushes the graph with the
branches, so that another clone can fetch it.`,
		Example: `st graph share`,
		RunE: func(cmd *cobra.Command, args []string) error {
			_, gitRunner, _, _, err := getContext()
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

			gitDir, err := gitRunner.GitCommonDir()
			if err != nil || gitDir == "" {
				return fmt.Errorf("failed to resolve git directory: %w", err)
			}
			localPath := filepath.Join(gitDir, "stack", "graph.json")
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
		Short: "Move the graph back to the local file",
		Long: `Copies the shared graph back to .git/stack/graph.json, deletes the git ref,
and stops fetching refs/staccato/*.`,
		Example: `st graph local`,
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

			localPath := filepath.Join(sc.GitDir(), "stack", "graph.json")
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
		Use:     "which",
		Short:   "Show where the graph is stored",
		Long:    `Prints 'Local' with the file's path, or 'Shared' with the git ref.`,
		Example: `st graph which`,
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
