package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// helpFooter ends every help page.
const helpFooter = "Run `st help` for the AI-agent primer."

// argDoc documents one positional argument in a command's help.
type argDoc struct {
	Name        string
	Description string
}

// argDocs holds the positional arguments each command documents.
var argDocs = map[*cobra.Command][]argDoc{}

// withArgs records the positional arguments listed under "Arguments:" in
// cmd's help. The command's Use says how they are written on the command line.
func withArgs(cmd *cobra.Command, docs ...argDoc) *cobra.Command {
	argDocs[cmd] = docs
	return cmd
}

// examplePreviewLimit is how many examples of each of its commands a group's
// help shows.
const examplePreviewLimit = 3

// helpText renders a command's help page: summary, usage, description,
// commands, arguments, options and examples, in that order.
func helpText(cmd *cobra.Command) string {
	children := listedCommands(cmd)
	var out strings.Builder
	fmt.Fprintf(&out, "%s\n\nUsage:\n  %s\n", cmd.Short, usageLine(cmd))
	if description := strings.TrimSpace(cmd.Long); description != "" {
		fmt.Fprintf(&out, "\n%s\n", description)
	}
	if len(children) > 0 {
		out.WriteString("\nCommands:\n")
		for _, child := range children {
			fmt.Fprintf(&out, "  %-18s %s\n", child.Name(), child.Short)
		}
	}
	if args := argDocs[cmd]; len(args) > 0 {
		out.WriteString("\nArguments:\n")
		for _, arg := range args {
			fmt.Fprintf(&out, "  %-18s %s\n", arg.Name, arg.Description)
		}
	}
	if options := optionLines(cmd); len(options) > 0 {
		out.WriteString("\nOptions:\n")
		for _, option := range options {
			out.WriteString(option)
		}
	}
	if examples := helpExamples(cmd, children); len(examples) > 0 {
		if len(children) > 0 {
			out.WriteString("\nExamples (run subcommand help for more specific examples):\n")
		} else {
			out.WriteString("\nExamples:\n")
		}
		for _, example := range examples {
			out.WriteString(example)
		}
	}
	fmt.Fprintf(&out, "\n%s\n", helpFooter)
	return out.String()
}

// usageError adds to an error about how a command was invoked the way it is
// meant to be invoked.
func usageError(cmd *cobra.Command, err error) error {
	return fmt.Errorf("%w\n\nUsage:\n  %s\n\nRun `%s --help` for details.", err, usageLine(cmd), cmd.CommandPath())
}

// explainMisuse makes every command under cmd answer wrong arguments with a
// usageError.
func explainMisuse(cmd *cobra.Command) {
	if validate := cmd.Args; validate != nil {
		cmd.Args = func(cmd *cobra.Command, args []string) error {
			if err := validate(cmd, args); err != nil {
				return usageError(cmd, err)
			}
			return nil
		}
	}
	for _, child := range cmd.Commands() {
		explainMisuse(child)
	}
}

// listedCommands returns the subcommands a help page lists.
func listedCommands(cmd *cobra.Command) []*cobra.Command {
	var listed []*cobra.Command
	for _, child := range cmd.Commands() {
		if child.IsAvailableCommand() {
			listed = append(listed, child)
		}
	}
	return listed
}

// helpExamples returns the example lines of a help page. A command shows its
// own examples. A group previews the first few of each of its commands, after
// its own when the group can be run by itself.
func helpExamples(cmd *cobra.Command, children []*cobra.Command) []string {
	var lines []string
	if len(children) == 0 || cmd.Runnable() {
		for _, example := range exampleLines(cmd) {
			lines = append(lines, "  "+example+"\n")
		}
	}
	for _, child := range children {
		examples := exampleLines(child)
		shown := min(len(examples), examplePreviewLimit)
		for _, example := range examples[:shown] {
			lines = append(lines, "  "+example+"\n")
		}
		if more := len(examples) - shown; more > 0 {
			lines = append(lines, fmt.Sprintf("    (and %d more; run `%s --help`)\n", more, child.CommandPath()))
		}
	}
	return lines
}

// usageLine is the command as it is typed: its path, the operands from its Use
// and, when it takes flags that Use does not already place, "[options]". A
// group ends in "<command>".
func usageLine(cmd *cobra.Command) string {
	line := cmd.CommandPath()
	if len(listedCommands(cmd)) > 0 && !cmd.Runnable() {
		return line + " <command>"
	}
	_, operands, _ := strings.Cut(cmd.Use, " ")
	if operands != "" {
		line += " " + operands
	}
	if hasOwnFlags(cmd) && !strings.Contains(operands, "--") && !strings.Contains(operands, "[options]") {
		line += " [options]"
	}
	if len(listedCommands(cmd)) > 0 {
		line += " | <command>"
	}
	return line
}

// hasOwnFlags reports whether a command defines flags of its own, apart from
// --help.
func hasOwnFlags(cmd *cobra.Command) bool {
	own := false
	cmd.NonInheritedFlags().VisitAll(func(flag *pflag.Flag) {
		if !flag.Hidden && flag.Name != "help" {
			own = true
		}
	})
	return own
}

// optionLines lists the flags a command accepts, its own before the ones it
// inherits.
func optionLines(cmd *cobra.Command) []string {
	var lines []string
	add := func(flag *pflag.Flag) {
		if flag.Hidden || flag.Name == "help" {
			return
		}
		label := "--" + flag.Name
		if flag.Shorthand != "" {
			label = "-" + flag.Shorthand + ", " + label
		}
		value, usage := pflag.UnquoteUsage(flag)
		if value != "" {
			label += " <" + value + ">"
		}
		lines = append(lines, fmt.Sprintf("  %-28s %s\n", label, usage))
	}
	cmd.NonInheritedFlags().VisitAll(add)
	cmd.InheritedFlags().VisitAll(add)
	return lines
}

// exampleLines returns the commands in a command's Example, one per line.
func exampleLines(cmd *cobra.Command) []string {
	var lines []string
	for _, line := range strings.Split(cmd.Example, "\n") {
		if line = strings.TrimSpace(line); line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

// helpCmd is 'st help': the agent primer, or the help page of the command
// named by its arguments.
func helpCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "help [command]",
		Short: "Print the AI-agent primer, or the help of a command",
		// main prints the returned error; cobra would print it a second time.
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, args []string) error {
			root := cmd.Root()
			if len(args) == 0 {
				fmt.Fprint(cmd.OutOrStdout(), agentPrimer(root))
				return nil
			}
			target, rest, err := root.Find(args)
			if err != nil || target == root || len(rest) > 0 {
				return fmt.Errorf("unknown command %q", strings.Join(args, " "))
			}
			return target.Help()
		},
	}
}

// agentPrimer is what an agent that has never used st reads first: the
// concepts, the main workflows, and where every command's own help is.
func agentPrimer(root *cobra.Command) string {
	var out strings.Builder
	out.WriteString(primerIntro)
	out.WriteString("\n## Commands\n\n")
	for _, cmd := range listedCommands(root) {
		fmt.Fprintf(&out, "- `%s --help` — %s\n", cmd.CommandPath(), cmd.Short)
	}
	out.WriteString(primerConventions)
	return out.String()
}

const primerIntro = "# Staccato CLI primer\n" + `
Staccato (` + "`st`" + `) manages stacks of git branches. A stack is a chain of
branches, each built on the one below it, so that a large change can be
reviewed as several small pull requests. ` + "`st`" + ` records which branch is stacked
on which and rebases ("restacks") the branches above when one below them
changes. Commit with git as usual; use ` + "`st`" + ` to create, move, rebase and push
the branches. Every command accepts ` + "`--help`" + `.

## Terms

- **Trunk** (or root): the branch that stacks are built on, normally ` + "`main`" + `.
- **Parent** and **child**: the branch below and the branch above another.
- **Lineage**: the current branch, the branches below it down to the trunk,
  and every branch stacked above it. Most commands act on the current lineage.
- **Tip**: the top branch of a stack, the one with no children.
- **Restack**: rebase each branch onto its parent.
- **Graph**: st's record of the stack, in ` + "`.git/stack/graph.json`" + ` unless shared.

## Build a stack

` + "```bash" + `
st new feature/api
git add -A && git commit -m "feat: add order API"
st append feature/frontend
git add -A && git commit -m "feat: add order page"
st log
` + "```" + `

` + "`st new`" + ` branches from the trunk and ` + "`st append`" + ` stacks on the current branch.
` + "`st log`" + ` shows the tree. Move with ` + "`st up`, `st down`, `st top`, `st bottom`" + ` or
` + "`git checkout`" + `.

## Change a branch in the middle of a stack

Check the branch out, then either amend its last commit, which restacks the
branches above it:

` + "```bash" + `
st modify --all
` + "```" + `

or add a commit with git and restack from the tip:

` + "```bash" + `
git commit -am "fix: validate totals"
st top
st restack
` + "```" + `

` + "`st restack`" + ` refuses to run from a branch that has children, unless
` + "`--to-current`" + ` is given, which leaves the branches above untouched.

## Conflicts

A restack stops at the first conflict, whichever command started it
(` + "`restack`, `insert`, `modify`, `move`, `attach --parent`, `sync`" + `). Resolve the
files, then:

` + "```bash" + `
git add <resolved-files>
st continue
` + "```" + `

It can stop again further up the stack; repeat until it finishes. ` + "`st abort`" + `
gives up instead and puts every branch back where it was.

## Push and open pull requests

` + "```bash" + `
st sync
st pr make --title "feat: add order API" --description "Adds the order API."
st status
` + "```" + `

` + "`st sync`" + ` fetches, fast-forwards the trunk, drops branches whose pull request
was merged, restacks the current lineage and pushes it. ` + "`st pr make`" + ` opens a
pull request from the current branch into its parent: run it on each branch,
from the bottom of the stack up. These need a remote named ` + "`origin`" + `, and pull
requests need the GitHub CLI (` + "`gh`" + `).

## More

- Adopt a branch made outside st: ` + "`st attach <branch> --parent <parent>`" + `.
- Find out which branches of a stack build: ` + "`st exec --help`" + `.
- Split a branch that is too large to review: ` + "`st split --help`" + `.
- Read the review feedback on the whole stack: ` + "`st reviews --help`" + `.
- Recover after a restack went wrong: ` + "`st restore --help`" + `.
`

const primerConventions = `
## Conventions

- Help prints to standard output and exits 0. ` + "`st <command> --help`" + ` works at
  every level, and ` + "`st help <command>`" + ` prints the same page.
- A command that fails exits 1 and prints the reason on standard error,
  followed by its usage if it was invoked wrongly. Warnings do not change the
  exit status and print with the normal output.
- ` + "`--json`" + `, on the commands that list it, prints machine-readable output.
- ` + "`-v`" + ` prints more detail. A few commands confirm what they did only with it.
- ` + "`st switch`, `st backup clean`" + `, and ` + "`st attach`" + ` without ` + "`--parent`" + ` or ` + "`--auto`" + `
  are interactive and need a terminal. In a script or an agent session use
  ` + "`git checkout <branch>`" + ` and ` + "`st attach --parent`" + ` instead.
- Commands that rewrite branches back them up first. Commit or stash your
  changes before running them.
`
