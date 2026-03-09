package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/spf13/cobra"
	stcontext "github.com/cpave3/staccato/pkg/context"
	"github.com/cpave3/staccato/pkg/git"
	"github.com/cpave3/staccato/pkg/graph"
	"github.com/cpave3/staccato/pkg/output"
	"github.com/cpave3/staccato/pkg/staleness"
)

var (
	verbose bool
	rootCmd *cobra.Command
)

// isTrunkBranch returns true if the branch name is a common trunk/root branch.
func isTrunkBranch(name string) bool {
	return stcontext.IsTrunkBranch(name)
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func init() {
	bannerLines := []string{
		` ███████╗ ████████╗  █████╗   ██████╗  ██████╗  █████╗  ████████╗  ██████╗`,
		` ██╔════╝ ╚══██╔══╝ ██╔══██╗ ██╔════╝ ██╔════╝ ██╔══██╗ ╚══██╔══╝ ██╔═══██╗`,
		` ███████╗    ██║    ███████║ ██║      ██║      ███████║    ██║    ██║   ██║`,
		` ╚════██║    ██║    ██╔══██║ ██║      ██║      ██╔══██║    ██║    ██║   ██║`,
		` ███████║    ██║    ██║  ██║ ╚██████╗ ╚██████╗ ██║  ██║    ██║    ╚██████╔╝`,
		` ╚══════╝    ╚═╝    ╚═╝  ╚═╝  ╚═════╝  ╚═════╝ ╚═╝  ╚═╝    ╚═╝     ╚═════╝`,
	}
	// Find the longest line (in runes) for gradient calculation
	maxLen := 0
	for _, l := range bannerLines {
		if n := len([]rune(l)); n > maxLen {
			maxLen = n
		}
	}
	// Horizontal gradient from #524180 to #ad8499
	r0, g0, b0 := 0x52, 0x41, 0x80
	r1, g1, b1 := 0xad, 0x84, 0x99
	var banner string
	for _, line := range bannerLines {
		runes := []rune(line)
		for i, ch := range runes {
			t := float64(i) / float64(maxLen-1)
			r := int(float64(r0) + t*float64(r1-r0))
			g := int(float64(g0) + t*float64(g1-g0))
			b := int(float64(b0) + t*float64(b1-b0))
			color := fmt.Sprintf("#%02x%02x%02x", r, g, b)
			banner += lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color(color)).Render(string(ch))
		}
		banner += "\n"
	}

	rootCmd = &cobra.Command{
		Use:   "st",
		Short: "A deterministic, offline-first Git stack management CLI",
		Long: "\n" + banner + "\n\n" +
			"Staccato provides branch-level stacking with deterministic restacking, automatic backups,\n" +
			"and lazy attachment for retrofitting existing branches.",
	}

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")

	// Add subcommands
	rootCmd.AddCommand(newCmd())
	rootCmd.AddCommand(appendCmd())
	rootCmd.AddCommand(insertCmd())
	rootCmd.AddCommand(restackCmd())
	rootCmd.AddCommand(continueCmd())
	rootCmd.AddCommand(attachCmd())
	rootCmd.AddCommand(restoreCmd())
	rootCmd.AddCommand(syncCmd())
	rootCmd.AddCommand(logCmd())
	rootCmd.AddCommand(switchCmdFunc())
	rootCmd.AddCommand(backupCmd())
	rootCmd.AddCommand(prCmd())
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(graphCmd())
	rootCmd.AddCommand(detachCmd())
	rootCmd.AddCommand(mcpCmd())
	rootCmd.AddCommand(modifyCmd())
	rootCmd.AddCommand(deleteCmd())
	rootCmd.AddCommand(moveCmd())
	rootCmd.AddCommand(abortCmd())
	rootCmd.AddCommand(upCmd())
	rootCmd.AddCommand(downCmd())
	rootCmd.AddCommand(topCmd())
	rootCmd.AddCommand(bottomCmd())
}

// requireBranch checks that HEAD is not detached. Returns an error if HEAD is detached.
func requireBranch(gitRunner *git.Runner) error {
	branch, err := gitRunner.GetCurrentBranch()
	if err != nil {
		return fmt.Errorf("HEAD is detached — check out a branch first")
	}
	if branch == "HEAD" {
		return fmt.Errorf("HEAD is detached — check out a branch first")
	}
	return nil
}

// warnDirtyTree prints a warning if the working tree has uncommitted changes.
func warnDirtyTree(gitRunner *git.Runner, printer *output.Printer) {
	dirty, err := gitRunner.HasUncommittedChanges()
	if err != nil {
		return
	}
	if dirty {
		printer.Warning("you have uncommitted changes — consider committing or stashing first")
	}
}

// getContext loads the graph and git runner for commands
func getContext() (*graph.Graph, *git.Runner, *output.Printer, string, error) {
	printer := output.NewPrinter(verbose)

	sc, err := stcontext.Load("")
	if err != nil {
		return nil, nil, nil, "", err
	}

	return sc.Graph, sc.Git, printer, sc.RepoPath, nil
}

// checkStaleness performs an offline check and prints a warning if local state is behind remote.
func checkStaleness(g *graph.Graph, gitRunner *git.Runner, printer *output.Printer) {
	hasRemote, _ := gitRunner.HasRemote()
	if !hasRemote {
		return
	}
	report := staleness.Check(g, gitRunner)
	if report.IsStale() {
		var msgs []string
		for _, s := range report.Signals {
			msgs = append(msgs, s.Message)
		}
		printer.StalenessWarning(msgs)
	}
}

// saveContext saves the graph
func saveContext(g *graph.Graph, repoPath string, gitRunner *git.Runner) error {
	sc := &stcontext.StaccatoContext{
		Graph:    g,
		Git:      gitRunner,
		RepoPath: repoPath,
	}
	return sc.Save()
}
