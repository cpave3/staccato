package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"text/tabwriter"

	"github.com/cpave3/staccato/pkg/output"
	"github.com/cpave3/staccato/pkg/transfer"
	"github.com/cpave3/staccato/pkg/version"
	"github.com/spf13/cobra"
)

func graphExportCmd() *cobra.Command {
	var opts transfer.ExportOptions
	var outputFile string
	cmd := &cobra.Command{
		Use:   "export",
		Short: "Write a stack's graph as a document for another clone",
		Long: `Writes the stack the current branch belongs to (from its bottom branch, the
one on the trunk, up through every branch stacked above) as a JSON document
that 'st graph import' merges into another clone's graph. --branch picks the
stack of another branch, and --all takes every branch st tracks.

The document records the trunk and, for each branch, its parent, the commit
it is based on, and the commit its git branch points to now:

  {"format": 1, "kind": "staccato.graph", "st_version": "v0.5.0",
   "root": "main",
   "branches": [{"name": "feat-a", "parent": "main",
                 "base_sha": "<commit>", "head_sha": "<commit>"}, ...]}

Branches are listed parents first, siblings by name.

The other clone fetches the branches from the remote, so each one has to be
pushed: its tip has to equal its remote-tracking branch (origin/<branch>, or
another remote with --remote). Run 'st sync' first. The check reads the
remote-tracking branches and does not contact the remote. It fails naming
every branch that is unpushed, behind or diverged; --allow-unpushed writes
the local tips anyway.

The document goes to standard output, or to the file --output names. Nothing
in this clone is changed.`,
		Example: `st graph export > stack.json
st graph export --all --output stack.json
st graph export --branch feat-a --allow-unpushed`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.All && opts.Branch != "" {
				return usageError(cmd, fmt.Errorf("--branch and --all cannot be used together"))
			}
			sc, err := loadStaccatoContext()
			if err != nil {
				return err
			}
			opts.STVersion = version.String()
			doc, err := transfer.Export(sc, opts)
			if err != nil {
				return err
			}
			data, err := transfer.Encode(doc)
			if err != nil {
				return err
			}
			if outputFile != "" {
				return os.WriteFile(outputFile, data, 0o644)
			}
			_, err = cmd.OutOrStdout().Write(data)
			return err
		},
	}
	cmd.Flags().StringVar(&opts.Branch, "branch", "", "Export the stack containing `branch` instead of the current one")
	cmd.Flags().BoolVar(&opts.All, "all", false, "Export every branch in the graph")
	cmd.Flags().StringVarP(&outputFile, "output", "o", "", "Write the document to `file` instead of standard output")
	cmd.Flags().StringVar(&opts.Remote, "remote", transfer.DefaultRemote, "Compare each branch with its tracking branch on `remote`")
	cmd.Flags().BoolVar(&opts.AllowUnpushed, "allow-unpushed", false, "Write branches whose tip differs from the remote's")
	return cmd
}

func graphImportCmd() *cobra.Command {
	var opts transfer.ImportOptions
	var dryRun, asJSON bool
	cmd := &cobra.Command{
		Use:   "import <file>",
		Short: "Merge a graph document from another clone into this one",
		Long: `Merges a document written by 'st graph export' in another clone into this
clone's graph, one branch at a time, and creates or fast-forwards the
branches to match. Use it to carry on in this clone with a stack built
elsewhere, such as by an agent on another machine.

It first fetches every branch the document names from origin (or --remote)
into its remote-tracking branch; --no-fetch uses the commits already here.
Then, for each branch, it compares the document's head with the local
branch and decides:

  added           no local branch: it is created at the document's head,
                  tracking origin/<branch>, with the document's entry
  updated         the local branch is at the document's head: the graph
                  takes the document's entry ("unchanged" when it already
                  had it)
  fast-forwarded  the local branch is behind: it is moved to the document's
                  head and the graph takes the document's entry
  kept-local      the local branch has the document's head and more: both
                  are kept as they are
  conflict        each side has commits the other lacks: the branch and its
                  entry are left alone. Compare the branch with
                  origin/<branch> and reconcile by hand, then import again
  skipped         it could not be done now: the branch has to be
                  fast-forwarded but is checked out with uncommitted changes
                  (commit or stash them) or in another worktree, or git
                  refused, or its parent was not merged

Branches the document does not name are not touched, and no branch is
deleted. Everything is decided and checked before anything changes; then the
branches are moved in one transaction and the graph is saved once, locally
or to the shared ref, whichever this clone uses. The branch checked out here
is fast-forwarded in the working tree. Nothing is restacked: if the trunk has
moved on, run 'st restack' afterwards.

The document's trunk has to be this graph's, unless this graph tracks no
branches yet, in which case it takes the document's. A document from a newer
st is refused with a message to update st.

Prints each branch with its outcome, and the reason for a conflict or a skip.
--json prints {"root", "branches": [{"name", "outcome", "reason"}]} instead.
--dry-run fetches, decides and prints, and changes no branch and no graph.
Exits 0 when every branch was merged, and 1 when any was a conflict or
skipped, after merging the others.`,
		Example: `st graph import stack.json
st graph import --dry-run stack.json
st graph import - < stack.json
st graph import --json --no-fetch stack.json`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var data []byte
			var err error
			if args[0] == "-" {
				data, err = io.ReadAll(cmd.InOrStdin())
			} else {
				data, err = os.ReadFile(args[0])
			}
			if err != nil {
				return fmt.Errorf("reading the document: %w", err)
			}
			doc, err := transfer.Decode(data)
			if err != nil {
				return err
			}
			sc, err := loadStaccatoContext()
			if err != nil {
				return err
			}
			plan, err := transfer.Plan(sc, doc, opts)
			if err != nil {
				return err
			}
			if !dryRun {
				if err := transfer.Apply(sc, plan); err != nil {
					return err
				}
			}

			out := cmd.OutOrStdout()
			if asJSON {
				if err := writeJSON(out, plan); err != nil {
					return err
				}
			} else {
				printImport(out, plan, dryRun)
			}
			if n := plan.Unmerged(); n > 0 {
				return fmt.Errorf("%d of %d branches were not merged (conflict or skipped): see the reasons above", n, len(plan.Branches))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&opts.Remote, "remote", transfer.DefaultRemote, "Fetch the document's branches from `remote`")
	cmd.Flags().BoolVar(&opts.NoFetch, "no-fetch", false, "Use the commits already in this repository; fetch nothing")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Fetch and print what would happen, changing nothing")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print the outcome of each branch as JSON")
	return withArgs(cmd, argDoc{"file", "The document from 'st graph export', or - to read standard input"})
}

func printImport(w io.Writer, plan *transfer.ImportPlan, dryRun bool) {
	if len(plan.Branches) == 0 {
		fmt.Fprintln(w, "The document names no branches.")
		return
	}
	verb := "Imported"
	if dryRun {
		verb = "Would import"
	}
	fmt.Fprintf(w, "%s %d branch(es) onto %s:\n", verb, len(plan.Branches), plan.Root)
	var table strings.Builder
	tw := tabwriter.NewWriter(&table, 0, 0, 2, ' ', 0)
	for _, b := range plan.Branches {
		icon := output.SuccessIcon
		switch b.Outcome {
		case transfer.Conflict:
			icon = output.ErrorIcon
		case transfer.Skipped:
			icon = output.WarningIcon
		}
		fmt.Fprintf(tw, "  %s %s\t%s\t%s\n", icon, b.Name, b.Outcome, b.Reason)
	}
	tw.Flush()
	for _, row := range strings.Split(strings.TrimRight(table.String(), "\n"), "\n") {
		fmt.Fprintln(w, strings.TrimRight(row, " "))
	}
	if dryRun {
		fmt.Fprintln(w, "Dry run: nothing was changed.")
	}
}
