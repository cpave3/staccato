package main

import (
	"fmt"
	"io"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/cpave3/staccato/pkg/output"
	"github.com/cpave3/staccato/pkg/stackexec"
	"github.com/spf13/cobra"
)

func execCmd() *cobra.Command {
	var failFast, asJSON bool
	cmd := &cobra.Command{
		Use:   "exec [options] <command> [args...]",
		Short: "Run a command on every branch and report which pass",
		Long: `Runs a command on every branch of the current stack, one branch at a time,
and reports for each branch whether the command passed or failed. Use it to
find out which branches of a stack build, pass their tests, or lint cleanly.

Which branches:

  The current lineage: the branch you are on, the branches below it down to
  the trunk, and every branch stacked above it. The trunk itself and other
  stacks are left out ('st log' shows the stack). Branches are visited from
  the bottom of the stack to the top, so a parent runs before its children.
  From the trunk, the command runs on every branch st tracks.

  You have to be on a branch of the stack ('st attach' adds one), and tracked
  files must have no uncommitted changes. Untracked files are fine.

The command:

  One argument is a shell command, run by 'sh -c'. Quote it to use pipes,
  '&&', redirection or variables:

    st exec "npm ci && npm test"

  Several arguments are the words of a single command and are passed on
  unchanged, without shell syntax. Flags of st exec go before the command;
  everything after the command's first word belongs to the command:

    st exec go test -v ./...
    st exec --fail-fast cargo build --release

  The command runs at the root of the repository with the branch's tip checked
  out, and with these environment variables set:

    ST_EXEC_BRANCH  the branch being checked
    ST_EXEC_PARENT  the branch below it in the stack (the trunk, for the
                    bottom branch)
    ST_EXEC_SHA     the commit that is checked out: the branch's tip
    ST_EXEC_INDEX   the branch's position in the run, counting from 1
    ST_EXEC_TOTAL   how many branches the run covers

  It passes on a branch when it exits with status 0 and fails on any other
  status. It gets no standard input, so it cannot prompt, and there is no
  timeout: interrupt a command that hangs (Ctrl-C), which stops it and
  returns you to your branch.

What is reported:

  While it runs, a line like '==> [2/11] feature-api: go build ./...' announces
  each branch and is followed by the command's own output. All of that goes to
  standard error. The report goes to standard output once the run is over:

    Ran on 3 branch(es): go build ./...
      ✔ feature-core  passed  1.2s
      ✘ feature-api   failed  0.8s  exit 1
      ✔ feature-ui    passed  1.4s
    2 passed, 1 failed

  So 'st exec "<command>" 2>/dev/null' prints the report alone.

  The command runs on every branch even after it has failed on one. With
  --fail-fast it stops at the first branch it fails on, and the branches above
  that one are reported as skipped.

  With --json the report is a JSON object instead:

    {
      "command": "go build ./...",
      "branches": [
        {"branch": "feature-core", "parent": "main", "sha": "<commit>",
         "status": "passed", "exit_code": 0, "duration_ms": 1200},
        {"branch": "feature-api", "parent": "feature-core", "sha": "<commit>",
         "status": "failed", "exit_code": 1, "duration_ms": 800},
        {"branch": "feature-ui", "parent": "feature-api", "sha": "<commit>",
         "status": "skipped", "duration_ms": 0,
         "note": "not run: --fail-fast stopped at feature-api"}
      ],
      "passed": 1,
      "failed": 1,
      "skipped": 1
    }

  "status" is "passed", "failed" or "skipped". "exit_code" is the command's
  exit status; it is left out when the command did not run to completion on
  that branch, and "note" then says why (skipped, the branch could not be
  checked out, or the run was interrupted). "interrupted": true is added when
  a signal stopped the run.

Exit status:

  0  the command passed on every branch
  1  the command failed on at least one branch, the run was interrupted, or
     st exec could not run. The message on standard error says which.

What happens to your checkout:

  Each branch's tip is checked out in this working tree in turn, as a detached
  HEAD, the way 'git rebase --exec' does it. When the run ends, also after a
  failure or Ctrl-C, the branch you started on is checked out again. No branch
  is moved, and nothing is committed or pushed.

  - The command is for checking branches, not for changing them. A commit it
    makes lands on a detached HEAD and is not added to the branch.
  - If it modifies tracked files (a formatter, a lockfile update), the changes
    are saved with 'git stash' before the next branch and noted in the report.
    'git stash list' shows them.
  - Untracked and ignored files, such as build output and dependency
    directories, stay where they are from one branch to the next, so builds
    are incremental. A command that needs a clean start has to clean up
    itself.
  - A branch that cannot be checked out, for example because an untracked
    file is in the way, is reported as failed and the run goes on.`,
		Example: `st exec "go build ./..."
st exec --fail-fast "npm ci && npm test"
st exec --json "task build" 2>/dev/null
st exec go test -v ./...
st exec 'task build > "/tmp/build-$ST_EXEC_INDEX.log" 2>&1'`,
		Args: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				return fmt.Errorf("st exec needs a command to run, for example: st exec \"task build\"")
			}
			// A first word with a space in it is a quoted shell command, and
			// the words after it would be handed to a program of that name.
			if len(args) > 1 && strings.ContainsAny(args[0], " \t\n") {
				return fmt.Errorf("%q is followed by more arguments (%s): pass the whole command as one quoted argument, and put the flags of st exec before the command, for example: st exec --fail-fast \"task build\"", args[0], strings.Join(args[1:], " "))
			}
			return nil
		},
		SilenceUsage: true,
		// main prints the returned error; cobra would print it a second time.
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			sc, err := loadStaccatoContext()
			if err != nil {
				return err
			}
			result, err := stackexec.Run(sc, stackexec.Options{
				Command:  shellCommand(args),
				FailFast: failFast,
				Output:   cmd.ErrOrStderr(),
			})
			// A run that had to stop still reports the branches it reached.
			if result != nil {
				if asJSON {
					if err := writeJSON(cmd.OutOrStdout(), result); err != nil {
						return err
					}
				} else {
					printExecResult(cmd.OutOrStdout(), result)
				}
			}
			if err != nil {
				return err
			}
			switch {
			case result.Interrupted:
				return fmt.Errorf("interrupted: %d of %d branches were not run", result.Skipped, len(result.Branches))
			case result.Failed > 0:
				return fmt.Errorf("%d of %d branches failed", result.Failed, len(result.Branches))
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&failFast, "fail-fast", false, "Stop at the first branch the command fails on; the branches above it are reported as skipped")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print the report as JSON")
	// Everything after the command's first word belongs to the command, so
	// that 'st exec go test -v ./...' does not read -v as a flag of st.
	cmd.Flags().SetInterspersed(false)
	return withArgs(cmd,
		argDoc{"command", "A shell command line; or, when args follow, the program to run"},
		argDoc{"args", "Further words of the command, passed on unchanged"})
}

// shellCommand turns the arguments of 'st exec' into the command line handed
// to the shell. One argument is a shell command as written. Several are the
// words of one command, each quoted so that the shell passes it on unchanged.
func shellCommand(args []string) string {
	if len(args) == 1 {
		return args[0]
	}
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = shellQuote(arg)
	}
	return strings.Join(quoted, " ")
}

// shellQuote quotes s for a POSIX shell, leaving plain words as they are.
func shellQuote(s string) string {
	plain := s != "" && strings.IndexFunc(s, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || strings.ContainsRune("_@%+=:,./-", r))
	}) < 0
	if plain {
		return s
	}
	return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
}

func printExecResult(w io.Writer, result *stackexec.Result) {
	fmt.Fprintf(w, "Ran on %d branch(es): %s\n", len(result.Branches), result.Command)
	var table strings.Builder
	tw := tabwriter.NewWriter(&table, 0, 0, 2, ' ', 0)
	for _, b := range result.Branches {
		icon, duration := output.SuccessIcon, ""
		var details []string
		switch b.Status {
		case stackexec.StatusFailed:
			icon = output.ErrorIcon
			if b.ExitCode != nil {
				details = append(details, fmt.Sprintf("exit %d", *b.ExitCode))
			}
		case stackexec.StatusSkipped:
			icon = "-"
		}
		if b.Status != stackexec.StatusSkipped {
			duration = fmt.Sprintf("%.1fs", (time.Duration(b.DurationMS) * time.Millisecond).Seconds())
		}
		if b.Note != "" {
			details = append(details, b.Note)
		}
		fmt.Fprintf(tw, "  %s %s\t%s\t%s\t%s\n", icon, b.Branch, b.Status, duration, strings.Join(details, "; "))
	}
	tw.Flush()
	// A row without details would otherwise end in the column's padding.
	for _, row := range strings.Split(strings.TrimRight(table.String(), "\n"), "\n") {
		fmt.Fprintln(w, strings.TrimRight(row, " "))
	}
	summary := fmt.Sprintf("%d passed, %d failed", result.Passed, result.Failed)
	if result.Skipped > 0 {
		summary += fmt.Sprintf(", %d skipped", result.Skipped)
	}
	fmt.Fprintln(w, summary)
}
