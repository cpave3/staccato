package main

import (
	"bytes"
	"os/exec"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestHelpForACommand(t *testing.T) {
	out := runSt(t, "new", "--help")

	for _, want := range []string{
		"Usage:\n  st new <branch>\n",
		"Arguments:\n  branch ",
		"Examples:\n  st new feature/api\n",
		"Run `st help` for the AI-agent primer.\n",
	} {
		assertContains(t, out, want)
	}
}

func noop(*cobra.Command, []string) {}

func TestHelpForAGroupPreviewsItsCommands(t *testing.T) {
	root := &cobra.Command{Use: "st", Short: "Root"}
	root.AddCommand(
		&cobra.Command{Use: "child <thing>", Short: "Child summary", Run: noop,
			Example: "st child one\nst child two\nst child three\nst child four"},
		&cobra.Command{Use: "other", Short: "Other summary", Run: noop, Example: "st other"},
		&cobra.Command{Use: "secret", Short: "Hidden summary", Run: noop, Hidden: true, Example: "st secret"},
	)

	out := helpText(root)

	for _, want := range []string{
		"Usage:\n  st <command>\n",
		"Commands:\n  child              Child summary\n  other              Other summary\n",
		// At most three examples of each command, and where the rest are.
		"Examples (run subcommand help for more specific examples):\n" +
			"  st child one\n  st child two\n  st child three\n" +
			"    (and 1 more; run `st child --help`)\n" +
			"  st other\n",
	} {
		assertContains(t, out, want)
	}
	for _, unwanted := range []string{"st child four", "secret"} {
		if strings.Contains(out, unwanted) {
			t.Errorf("group help contains %q:\n%s", unwanted, out)
		}
	}
}

func TestHelpForAGroupThatAlsoRuns(t *testing.T) {
	root := &cobra.Command{Use: "st", Short: "Root"}
	group := &cobra.Command{Use: "backup", Short: "Snapshot", Run: noop, Example: "st backup"}
	group.AddCommand(&cobra.Command{Use: "list", Short: "List snapshots", Run: noop, Example: "st backup list"})
	root.AddCommand(group)

	out := helpText(group)

	for _, want := range []string{
		"Usage:\n  st backup | <command>\n",
		"Commands:\n  list               List snapshots\n",
		// Its own examples come before those of its commands.
		"  st backup\n  st backup list\n",
	} {
		assertContains(t, out, want)
	}
}

func TestHelpForTheRoot(t *testing.T) {
	// Bare 'st' is the same page as 'st --help'.
	for _, args := range [][]string{{"--help"}, {}} {
		out := runSt(t, args...)
		for _, want := range []string{
			"Usage:\n  st <command>\n",
			"Commands:\n  new ",
			"Options:\n  -v, --verbose",
			"Examples (run subcommand help for more specific examples):\n  st new feature/api\n",
			"Run `st help` for the AI-agent primer.\n",
		} {
			assertContains(t, out, want)
		}
		// The banner is decoration for a terminal, not for a pipe.
		if strings.Contains(out, "█") {
			t.Errorf("st %v prints the banner when its output is not a terminal:\n%s", args, out)
		}
	}
}

func TestHelpGoesToStdout(t *testing.T) {
	cmd := exec.Command(stBinary, "new", "--help")
	setCoverEnv(cmd)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		t.Fatalf("st new --help failed: %v", err)
	}
	if !strings.Contains(stdout.String(), "Usage:") || stderr.Len() != 0 {
		t.Errorf("stdout = %q, stderr = %q, want the help on stdout only", stdout.String(), stderr.String())
	}
}

func TestHelpCommand(t *testing.T) {
	t.Run("prints_the_agent_primer", func(t *testing.T) {
		out := runSt(t, "help")
		assertContains(t, out, "# Staccato CLI primer")
	})

	t.Run("prints_the_help_of_a_command", func(t *testing.T) {
		if got, want := runSt(t, "help", "new"), runSt(t, "new", "--help"); got != want {
			t.Errorf("st help new =\n%s\nwant the same as st new --help:\n%s", got, want)
		}
		if got, want := runSt(t, "help", "split", "apply"), runSt(t, "split", "apply", "--help"); got != want {
			t.Errorf("st help split apply =\n%s\nwant the same as st split apply --help:\n%s", got, want)
		}
	})

	t.Run("refuses_an_unknown_command", func(t *testing.T) {
		out := runStExpectError(t, "help", "nope")
		assertContains(t, out, `unknown command "nope"`)
	})
}

func TestAgentPrimer(t *testing.T) {
	primer := agentPrimer(rootCmd)

	for _, want := range []string{
		"# Staccato CLI primer",
		// The workflows an agent needs before it reads any one command's help.
		"st new feature/api", "st append ", "st modify --all", "st restack", "st continue", "st abort", "st sync", "st pr make",
		// Where to look next, and what to expect from every command.
		"`st split --help`", "`st exec --help`", "--json", "standard error",
	} {
		if !strings.Contains(primer, want) {
			t.Errorf("primer does not mention %q", want)
		}
	}
	// Every command is listed with its summary.
	for _, cmd := range listedCommands(rootCmd) {
		if entry := "- `" + cmd.CommandPath() + " --help` — " + cmd.Short; !strings.Contains(primer, entry) {
			t.Errorf("primer does not list %q", entry)
		}
	}
}

// allCommands returns every command a help page lists, at every depth.
func allCommands(parent *cobra.Command) []*cobra.Command {
	var all []*cobra.Command
	for _, cmd := range listedCommands(parent) {
		all = append(all, cmd)
		all = append(all, allCommands(cmd)...)
	}
	return all
}

func TestEveryCommandIsDocumented(t *testing.T) {
	for _, cmd := range allCommands(rootCmd) {
		path := cmd.CommandPath()
		// 59 characters keep a line of a "Commands:" list within 80 columns.
		if cmd.Short == "" || strings.HasSuffix(cmd.Short, ".") || len(cmd.Short) > 59 {
			t.Errorf("%s: summary %q, want one line of at most 59 characters without a full stop", path, cmd.Short)
		}
		// A group is explained by its commands; a command explains itself.
		if !cmd.HasAvailableSubCommands() && strings.TrimSpace(cmd.Long) == "" {
			t.Errorf("%s: no description", path)
		}
		examples := exampleLines(cmd)
		if len(examples) == 0 {
			t.Errorf("%s: no examples", path)
		}
		for _, example := range examples {
			if example != path && !strings.HasPrefix(example, path+" ") {
				t.Errorf("%s: example %q does not start with the command", path, example)
			}
		}
		// Whatever Use says the command takes is explained under Arguments.
		_, operands, _ := strings.Cut(cmd.Use, " ")
		fields := strings.Fields(operands)
		for i, operand := range fields {
			name := strings.Trim(operand, "<>[].")
			// A flag written into the usage, and the value that follows it,
			// are described under Options.
			if strings.HasPrefix(name, "-") || name == "options" || (i > 0 && strings.HasPrefix(fields[i-1], "--")) {
				continue
			}
			documented := false
			for _, arg := range argDocs[cmd] {
				documented = documented || arg.Name == name
			}
			if !documented {
				t.Errorf("%s: argument %q in its usage is not described", path, name)
			}
		}
	}
}

// shellWords splits a command line into words the way a shell would, as far as
// the examples in the help need: quotes group, and nothing is expanded.
func shellWords(line string) []string {
	var words []string
	var word strings.Builder
	var quote rune
	inWord := false
	for _, r := range line {
		switch {
		case quote != 0:
			if r == quote {
				quote = 0
			} else {
				word.WriteRune(r)
			}
		case r == '"' || r == '\'':
			quote, inWord = r, true
		case r == ' ':
			if inWord {
				words = append(words, word.String())
				word.Reset()
				inWord = false
			}
		default:
			word.WriteRune(r)
			inWord = true
		}
	}
	if inWord {
		words = append(words, word.String())
	}
	return words
}

// checkExample fails the test if an example names a command or a flag that
// does not exist.
func checkExample(t *testing.T, example string) {
	t.Helper()
	words := shellWords(example)
	if len(words) == 0 || words[0] != "st" {
		t.Errorf("example %q does not start with st", example)
		return
	}
	// What follows a shell operator is not st's.
	for i, word := range words {
		if word == "|" || word == "&&" || word == ";" || strings.Contains(word, ">") {
			words = words[:i]
			break
		}
	}
	cmd, rest, err := rootCmd.Find(words[1:])
	if err != nil || cmd == rootCmd {
		t.Errorf("example %q names no st command", example)
		return
	}
	for _, word := range rest {
		if word == "--" {
			break
		}
		// A lone "-" is an operand meaning standard input, not a flag.
		if word == "-" || !strings.HasPrefix(word, "-") {
			// 'st exec' hands everything after the command's first word to it.
			if cmd.Name() == "exec" {
				break
			}
			continue
		}
		name, _, _ := strings.Cut(strings.TrimLeft(word, "-"), "=")
		known := cmd.LocalFlags().Lookup(name) != nil || cmd.InheritedFlags().Lookup(name) != nil
		if !strings.HasPrefix(word, "--") {
			known = cmd.LocalFlags().ShorthandLookup(name) != nil || cmd.InheritedFlags().ShorthandLookup(name) != nil
		}
		if !known {
			t.Errorf("example %q uses %s, which %s does not accept", example, word, cmd.CommandPath())
		}
	}
}

func TestExamplesUseRealCommandsAndFlags(t *testing.T) {
	for _, cmd := range allCommands(rootCmd) {
		for _, example := range exampleLines(cmd) {
			checkExample(t, example)
		}
	}
	// The primer's worked examples are held to the same standard.
	checked := 0
	for _, line := range strings.Split(agentPrimer(rootCmd), "\n") {
		if strings.HasPrefix(line, "st ") {
			checkExample(t, line)
			checked++
		}
	}
	if checked < 8 {
		t.Errorf("checked %d st commands in the primer, want its worked examples found", checked)
	}
}

func TestErrors(t *testing.T) {
	t.Run("a_failed_command_gives_its_reason_once_and_no_usage", func(t *testing.T) {
		setupRepoWithStack(t)
		runSt(t, "new", "f1")

		out := runStExpectError(t, "up")

		if n := strings.Count(out, "already at the tip of the stack"); n != 1 {
			t.Errorf("the reason is printed %d times, want once:\n%s", n, out)
		}
		if strings.Contains(out, "Usage:") {
			t.Errorf("a command that was used correctly prints its usage:\n%s", out)
		}
	})

	t.Run("a_missing_argument_shows_how_the_command_is_used", func(t *testing.T) {
		out := runStExpectError(t, "new")

		if n := strings.Count(out, "accepts 1 arg(s), received 0"); n != 1 {
			t.Errorf("the reason is printed %d times, want once:\n%s", n, out)
		}
		assertContains(t, out, "Usage:\n  st new <branch>\n")
		assertContains(t, out, "Run `st new --help` for details.")
	})

	t.Run("an_unknown_flag_shows_how_the_command_is_used", func(t *testing.T) {
		out := runStExpectError(t, "restack", "--nope")

		if n := strings.Count(out, "unknown flag: --nope"); n != 1 {
			t.Errorf("the reason is printed %d times, want once:\n%s", n, out)
		}
		assertContains(t, out, "Usage:\n  st restack [options]\n")
		assertContains(t, out, "Run `st restack --help` for details.")
	})

	t.Run("an_unknown_command_points_to_the_list", func(t *testing.T) {
		out := runStExpectError(t, "nope")

		if n := strings.Count(out, `unknown command "nope"`); n != 1 {
			t.Errorf("the reason is printed %d times, want once:\n%s", n, out)
		}
		assertContains(t, out, "Run `st --help` for the list of commands.")
	})
}
