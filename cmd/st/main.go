package main

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/lipgloss"
	stcontext "github.com/cpave3/staccato/pkg/context"
	"github.com/cpave3/staccato/pkg/git"
	"github.com/cpave3/staccato/pkg/graph"
	"github.com/cpave3/staccato/pkg/output"
	"github.com/cpave3/staccato/pkg/staleness"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
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
		if strings.HasPrefix(err.Error(), "unknown command ") {
			fmt.Fprintln(os.Stderr, "Run `st --help` for the list of commands.")
		}
		os.Exit(1)
	}
}

func init() {
	// Commands are listed in the order they are added below, which follows
	// the life of a stack, instead of alphabetically.
	cobra.EnableCommandSorting = false

	rootCmd = &cobra.Command{
		Use:   "st",
		Short: "A deterministic, offline-first Git stack management CLI",
		Long: `A stack is a chain of branches, each built on the one below it, so that a
large change can be reviewed as several small pull requests. st records which
branch is stacked on which, and rebases ("restacks") the branches above when
one below them changes. Commit with git as usual.`,
		// main prints the error a command returns, once. A command that was
		// misused says how it is used; one that failed does not.
		SilenceErrors: true,
		SilenceUsage:  true,
	}

	rootCmd.PersistentFlags().BoolVarP(&verbose, "verbose", "v", false, "Print more detail about what a command does")
	rootCmd.SetHelpFunc(func(cmd *cobra.Command, _ []string) {
		out := cmd.OutOrStdout()
		// The banner is decoration for a person at a terminal.
		if cmd == cmd.Root() && isTerminal(out) {
			fmt.Fprintf(out, "\n%s\n", banner())
		}
		fmt.Fprint(out, helpText(cmd))
	})
	rootCmd.SetHelpCommand(helpCmd())
	rootCmd.SetFlagErrorFunc(usageError)

	// Create branches
	rootCmd.AddCommand(newCmd())
	rootCmd.AddCommand(appendCmd())
	rootCmd.AddCommand(insertCmd())
	// Move around and look
	rootCmd.AddCommand(upCmd())
	rootCmd.AddCommand(downCmd())
	rootCmd.AddCommand(topCmd())
	rootCmd.AddCommand(bottomCmd())
	rootCmd.AddCommand(switchCmdFunc())
	rootCmd.AddCommand(logCmd())
	rootCmd.AddCommand(statusCmd())
	rootCmd.AddCommand(statCmd())
	// Change a stack
	rootCmd.AddCommand(modifyCmd())
	rootCmd.AddCommand(restackCmd())
	rootCmd.AddCommand(continueCmd())
	rootCmd.AddCommand(abortCmd())
	rootCmd.AddCommand(moveCmd())
	rootCmd.AddCommand(attachCmd())
	rootCmd.AddCommand(detachCmd())
	rootCmd.AddCommand(deleteCmd())
	rootCmd.AddCommand(deleteStackCmd())
	rootCmd.AddCommand(splitCmd())
	rootCmd.AddCommand(execCmd())
	// Work with the remote
	rootCmd.AddCommand(syncCmd())
	rootCmd.AddCommand(prCmd())
	rootCmd.AddCommand(reviewsCmd())
	// Recover and maintain
	rootCmd.AddCommand(backupCmd())
	rootCmd.AddCommand(restoreCmd())
	rootCmd.AddCommand(graphCmd())
	rootCmd.AddCommand(skillCmd())

	explainMisuse(rootCmd)
}

// banner renders the staccato wordmark with a horizontal colour gradient.
func banner() string {
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
	return banner
}

// isTerminal reports whether w is a terminal, as opposed to a pipe or a file.
func isTerminal(w io.Writer) bool {
	f, ok := w.(*os.File)
	return ok && isatty.IsTerminal(f.Fd())
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

// loadStaccatoContext returns the full StaccatoContext (needed by commands that
// use shared-mode features like IsShared/SharedRef).
func loadStaccatoContext() (*stcontext.StaccatoContext, error) {
	return stcontext.Load("")
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
	sc := stcontext.NewContext(g, gitRunner, repoPath)
	return sc.Save()
}
