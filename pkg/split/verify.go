package split

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"
	"time"

	stcontext "github.com/cpave3/staccato/pkg/context"
)

// verifyTailLines is how much of a failing command's output the error carries
// when the output was not streamed anywhere.
const verifyTailLines = 40

// verify checks out each slice in turn and runs command in it, the way
// `git rebase --exec` does, then returns to the starting checkout. It runs
// before any branch moves, so a failing slice leaves the repository as it was.
//
// The command runs through `sh -c` at the repository root with ST_SPLIT_SLICE,
// ST_SPLIT_SHA, ST_SPLIT_INDEX and ST_SPLIT_TOTAL set. Its output is streamed
// to out when out is non-nil; otherwise the tail of it is included in the
// error.
func verify(sc *stcontext.StaccatoContext, built []builtSlice, command string, out io.Writer) (err error) {
	g := sc.Git
	if dirty, err := trackedChanges(sc); err != nil {
		return err
	} else if dirty != "" {
		return fmt.Errorf("verifying checks out each slice in turn, and uncommitted changes would be carried into them — commit or stash them first")
	}

	start, _ := g.GetCurrentBranch()
	startSHA, err := g.GetCommitSHA("HEAD")
	if err != nil {
		return fmt.Errorf("resolve HEAD: %w", err)
	}

	// An interrupt stops the command and still returns to the starting
	// checkout, so a cancelled or timed-out run does not strand HEAD on a
	// slice.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	defer func() {
		var restoreErr error
		if start == "HEAD" {
			_, restoreErr = g.Run("checkout", "--quiet", "--detach", startSHA)
		} else {
			restoreErr = g.CheckoutBranch(start)
		}
		if restoreErr != nil && err == nil {
			err = fmt.Errorf("return to the starting checkout: %w", restoreErr)
		}
	}()

	for i, s := range built {
		// --no-overwrite-ignore: a slice that tracks a path which is an
		// ignored file here fails instead of overwriting that file.
		if _, err := g.Run("checkout", "--quiet", "--no-overwrite-ignore", "--detach", s.sha); err != nil {
			return fmt.Errorf("check out slice %q: %w", s.name, err)
		}
		if out != nil {
			fmt.Fprintf(out, "==> [%d/%d] %s: %s\n", i+1, len(built), s.name, command)
		}

		cmd := exec.CommandContext(ctx, "sh", "-c", command)
		cmd.Dir = sc.RepoPath
		cmd.Env = append(os.Environ(),
			"ST_SPLIT_SLICE="+s.name,
			"ST_SPLIT_SHA="+s.sha,
			fmt.Sprintf("ST_SPLIT_INDEX=%d", i+1),
			fmt.Sprintf("ST_SPLIT_TOTAL=%d", len(built)),
		)
		// Its own process group, so an interrupt reaches whatever it started.
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM) }
		cmd.WaitDelay = 5 * time.Second
		var captured bytes.Buffer
		if out != nil {
			cmd.Stdout, cmd.Stderr = out, out
		} else {
			cmd.Stdout, cmd.Stderr = &captured, &captured
		}
		runErr := cmd.Run()

		// The command must leave tracked files alone: its changes would be
		// carried into the next slice and into the starting checkout. They are
		// stashed rather than discarded.
		changed, err := trackedChanges(sc)
		if err != nil {
			return err
		}
		if changed != "" {
			if _, err := g.Run("stash", "push", "--quiet", "-m", "st split: changes made by the verify command in "+s.name); err != nil {
				return fmt.Errorf("slice %q: the verify command changed tracked files and they could not be stashed: %w", s.name, err)
			}
		}

		switch {
		case ctx.Err() != nil:
			return fmt.Errorf("interrupted while verifying slice %q — nothing was applied", s.name)
		case runErr != nil:
			msg := fmt.Sprintf("slice %q failed %q (%v) — nothing was applied", s.name, command, runErr)
			if tail := lastLines(captured.String(), verifyTailLines); tail != "" {
				msg += "\n" + tail
			}
			return fmt.Errorf("%s", msg)
		case changed != "":
			return fmt.Errorf("slice %q: the verify command changed tracked files, so the slices cannot be trusted — nothing was applied:\n%s\nthe changes were saved with 'git stash'", s.name, changed)
		}
	}
	return nil
}

// trackedChanges returns the porcelain status of tracked files, empty when
// there are no staged or unstaged changes. Untracked files are ignored.
func trackedChanges(sc *stcontext.StaccatoContext) (string, error) {
	out, err := sc.Git.Output(nil, "", "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return "", fmt.Errorf("check working tree: %w", err)
	}
	return strings.TrimRight(out, "\n"), nil
}

// lastLines returns the last n lines of s.
func lastLines(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) > n {
		lines = lines[len(lines)-n:]
	}
	return strings.Join(lines, "\n")
}
