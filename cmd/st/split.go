package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/cpave3/staccato/pkg/split"
	"github.com/spf13/cobra"
)

func splitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "split",
		Short: "Split a branch into several stacked branches",
		Long: `Splits one branch of a stack into smaller stacked branches for review.
You decide where the boundaries go; st builds the slices and moves the branches.

  1. st split describe                    inventory of the branch, as JSON
  2. write a JSON plan that assigns its commits or files to slices
  3. st split apply plan.json --dry-run   check the plan; nothing moves
  4. st split apply plan.json             apply it

Run 'st split apply --help' for the plan format.`,
	}
	cmd.AddCommand(splitDescribeCmd())
	cmd.AddCommand(splitApplyCmd())
	return cmd
}

func splitDescribeCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "describe [branch]",
		Short: "Print a branch's commits and changed files as JSON",
		Long: `Prints the inventory needed to plan a split, as JSON:

  branch, parent, worktree
            what is described, the branch below it, and the checkout the
            command ran in
  base      the commit the branch's changes are measured from
  head      the branch's tip. Put it in the plan, so that a branch which has
            moved since is not split by a stale plan.
  commits   each commit: abbreviated sha, subject, files, insertions/deletions
  files     each changed file: status, insertions/deletions, the commits that
            touch it, and renamed_from when it is the new path of a rename
  totals    the net change
  churn     what every commit changed, added up
  children  branches stacked on top. A split moves them onto the top slice.

Choosing between commits and paths: cut at commits only when each commit
belongs wholly to one slice. Split by paths instead when a commit touches files
that belong in different slices, or when churn is much larger than totals,
which means the commits rework each other.

"needs_restack": true means the branch is behind its parent and has to be
restacked before it can be split.

Describes the current branch unless a branch name is given.`,
		Args:         cobra.MaximumNArgs(1),
		SilenceUsage: true,
		// main prints the returned error; cobra would print it a second time.
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			sc, err := loadStaccatoContext()
			if err != nil {
				return err
			}
			branch := ""
			if len(args) == 1 {
				branch = args[0]
			}
			description, err := split.Describe(sc, branch)
			if err != nil {
				return err
			}
			return writeJSON(cmd.OutOrStdout(), description)
		},
	}
}

func splitApplyCmd() *cobra.Command {
	var dryRun, asJSON bool
	var execCmd string
	cmd := &cobra.Command{
		Use:   "apply <plan.json | ->",
		Short: "Split a branch according to a JSON plan",
		Long: `Builds every slice of the plan, then moves the branches in one step.
A plan that cannot be carried out changes nothing.

A plan lists the slices from the bottom of the stack to the top:

  {
    "branch": "feature",
    "head": "<head from st split describe>",
    "slices": [
      {"name": "feature-core", "message": "core: add types", "paths": ["pkg/core/"]},
      {"name": "feature-api", "commits": ["1a2b3c4d5e6f"]},
      {"name": "feature", "message": "ui: add page", "rest": true}
    ]
  }

Plan fields:

  "branch"         branch to split (default: the current branch)
  "head"           expected tip of the branch, full or abbreviated; the split
                   is refused if the branch has moved since the plan was
                   written
  "slices"         two or more slices, bottom to top
  "drop_original"  true to delete the original branch name. Required when no
                   slice is named after the branch being split.

Each slice has a "name", which becomes its branch, and takes its content from
exactly one source:

  "commits"  existing commits of the branch, kept with their message and
             author, in the order listed. Takes no "message".
  "paths"    git pathspecs relative to the repository root: a file, a
             directory ("pkg/core/"), a glob ("*.sql"), or pathspec magic
             (":(exclude)docs/"). Every changed file they match is brought
             to its final state, deletions included, in one new commit
             authored by you. A path takes only files that no earlier slice
             has taken, and taking none is an error. Needs a "message".
  "rest"     true: everything the earlier slices left out, in one new commit.
             Last slice only. Needs a "message".

Name the top slice after the branch being split, so that its pull request and
child branches stay attached.

A "paths" slice may also carry "overrides", for a file whose changes belong to
two slices. Each maps a path to a file holding the content that path has in
this slice, which is only partway to final. A later slice lists the path to
finish it:

  {"name": "feature-core", "message": "...", "paths": ["pkg/core/"],
   "overrides": {"README.md": "README.core.md"}},
  {"name": "feature", "message": "...", "paths": ["pkg/api/", "README.md"]}

Relative content files are resolved against the plan's directory.

The top slice must end with exactly the content the branch had before the
split: a plan that leaves anything out is refused, with the missing files
listed. Ending with a "rest" slice guarantees it. Branches stacked on top are
moved onto the top slice. Pass '-' to read the plan from standard input.

The working tree is not touched, so uncommitted changes are fine (except with
--exec). A checked-out branch stays checked out; if its name goes to a lower
slice or is dropped, you end on the top slice.

When a split rewrites commits, the originals are kept on a branch named in the
output (backup/manual/<time>/<branch>). There is no undo command, and
'st restore' does not use that branch.

With --dry-run, the slices are built and reported with their sizes and commit
ids, and nothing moves. The ids are previews: applying builds the commits again.

With --exec, each slice is checked out in turn and the command is run in it
(for example a build), like 'git rebase --exec'. The split is applied only if
the command succeeds in every slice. This needs a working tree without
uncommitted changes. ST_SPLIT_SLICE holds the slice's name.`,
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		// main prints the returned error; cobra would print it a second time.
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			data, baseDir, err := readPlan(cmd.InOrStdin(), args[0])
			if err != nil {
				return err
			}
			plan, err := split.ParsePlan(data)
			if err != nil {
				return err
			}

			sc, err := loadStaccatoContext()
			if err != nil {
				return err
			}
			result, applyErr := split.Apply(sc, plan, split.Options{
				DryRun:     dryRun,
				Exec:       execCmd,
				ExecOutput: cmd.ErrOrStderr(),
				BaseDir:    baseDir,
			})
			if applyErr != nil {
				return applyErr
			}
			if asJSON {
				if err := writeJSON(cmd.OutOrStdout(), result); err != nil {
					return err
				}
			} else {
				printSplitResult(cmd.OutOrStdout(), result)
			}
			split.FireHooks(sc.RepoPath, result)
			return nil
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Build and report the slices without moving any branch")
	cmd.Flags().StringVar(&execCmd, "exec", "", "Shell command to run in each slice first; if it fails anywhere, nothing is applied")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print the result as JSON")
	return cmd
}

// readPlan returns the plan's bytes and the directory its relative override
// sources resolve against.
func readPlan(stdin io.Reader, arg string) ([]byte, string, error) {
	if arg == "-" {
		data, err := io.ReadAll(stdin)
		if err != nil {
			return nil, "", fmt.Errorf("read plan: %w", err)
		}
		wd, err := os.Getwd()
		if err != nil {
			return nil, "", fmt.Errorf("get working directory: %w", err)
		}
		return data, wd, nil
	}
	data, err := os.ReadFile(arg)
	if err != nil {
		return nil, "", fmt.Errorf("read plan: %w", err)
	}
	dir, err := filepath.Abs(filepath.Dir(arg))
	if err != nil {
		return nil, "", fmt.Errorf("resolve plan directory: %w", err)
	}
	return data, dir, nil
}

func printSplitResult(w io.Writer, result *split.Result) {
	if result.DryRun {
		fmt.Fprintf(w, "Dry run: %q would become %d branches on %q:\n", result.Branch, len(result.Slices), result.Parent)
	} else {
		fmt.Fprintf(w, "Split %q into %d branches on %q:\n", result.Branch, len(result.Slices), result.Parent)
	}
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	for i, s := range result.Slices {
		fmt.Fprintf(tw, "  %d.\t%s\t%d commit(s)\t%d file(s)\t+%d/-%d\t%s\n", i+1, s.Name, s.Commits, s.Files, s.Insertions, s.Deletions, shortSHA(s.SHA))
	}
	tw.Flush()
	for _, warning := range result.Warnings {
		fmt.Fprintf(w, "Note: %s\n", warning)
	}
	if result.Verified != "" {
		fmt.Fprintf(w, "Verified each slice with: %s\n", result.Verified)
	}
	if result.DryRun {
		fmt.Fprintln(w, "Nothing was changed.")
		return
	}
	if len(result.Moved) > 0 {
		fmt.Fprintf(w, "Moved %d descendant branch(es) onto the split: %s\n", len(result.Moved), strings.Join(result.Moved, ", "))
	}
	if len(result.NeedsRestack) > 0 {
		fmt.Fprintf(w, "Left untouched because they were already behind their parent: %s (run 'st restack' on them)\n", strings.Join(result.NeedsRestack, ", "))
	}
	if result.Removed != "" {
		fmt.Fprintf(w, "Removed branch %q: no slice kept its name\n", result.Removed)
	}
	if result.Backup != "" {
		fmt.Fprintf(w, "Original commits kept at %s\n", result.Backup)
	}
}

func shortSHA(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

func writeJSON(w io.Writer, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return fmt.Errorf("encode output: %w", err)
	}
	_, err = fmt.Fprintln(w, string(data))
	return err
}
