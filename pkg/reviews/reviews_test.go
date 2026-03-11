package reviews

import (
	"fmt"
	"strings"
	"testing"

	"github.com/cpave3/staccato/pkg/graph"
)

func TestFilterBots_RemovesGenericBots(t *testing.T) {
	items := []FeedbackItem{
		{Author: "dependabot[bot]", Body: "bump version"},
		{Author: "human-reviewer", Body: "looks good"},
	}
	filtered := FilterBots(items)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 item, got %d", len(filtered))
	}
	if filtered[0].Author != "human-reviewer" {
		t.Errorf("expected human-reviewer, got %s", filtered[0].Author)
	}
}

func TestFilterBots_KeepsReviewBots(t *testing.T) {
	items := []FeedbackItem{
		{Author: "coderabbitai[bot]", Body: "review comment"},
		{Author: "greptile-apps[bot]", Body: "analysis"},
		{Author: "dependabot[bot]", Body: "bump"},
	}
	filtered := FilterBots(items)
	if len(filtered) != 2 {
		t.Fatalf("expected 2 items, got %d", len(filtered))
	}
	for _, item := range filtered {
		if item.AuthorType != "Bot" {
			t.Errorf("expected Bot author type for %s, got %s", item.Author, item.AuthorType)
		}
	}
}

func TestFilterBots_MarksHumansCorrectly(t *testing.T) {
	items := []FeedbackItem{
		{Author: "dev1", Body: "fix this"},
	}
	filtered := FilterBots(items)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 item, got %d", len(filtered))
	}
	if filtered[0].AuthorType != "Human" {
		t.Errorf("expected Human, got %s", filtered[0].AuthorType)
	}
}

func TestThreadReplies_AttachesRepliesToParents(t *testing.T) {
	items := []FeedbackItem{
		{ID: 100, Author: "reviewer", Body: "fix this", InReplyTo: 0},
		{ID: 101, Author: "author", Body: "done", InReplyTo: 100},
		{ID: 102, Author: "reviewer", Body: "thanks", InReplyTo: 100},
	}
	threaded := ThreadReplies(items)
	if len(threaded) != 1 {
		t.Fatalf("expected 1 root item, got %d", len(threaded))
	}
	if len(threaded[0].Replies) != 2 {
		t.Fatalf("expected 2 replies, got %d", len(threaded[0].Replies))
	}
}

func TestThreadReplies_PreservesStandaloneItems(t *testing.T) {
	items := []FeedbackItem{
		{ID: 1, Author: "a", Body: "comment 1", InReplyTo: 0},
		{ID: 2, Author: "b", Body: "comment 2", InReplyTo: 0},
	}
	threaded := ThreadReplies(items)
	if len(threaded) != 2 {
		t.Fatalf("expected 2 items, got %d", len(threaded))
	}
}

func TestParseRemoteURL_HTTPS(t *testing.T) {
	owner, repo, err := ParseRemoteURL("https://github.com/rexlabsio/wings-api.git")
	if err != nil {
		t.Fatal(err)
	}
	if owner != "rexlabsio" || repo != "wings-api" {
		t.Errorf("expected rexlabsio/wings-api, got %s/%s", owner, repo)
	}
}

func TestParseRemoteURL_HTTPSNoGit(t *testing.T) {
	owner, repo, err := ParseRemoteURL("https://github.com/cpave3/staccato")
	if err != nil {
		t.Fatal(err)
	}
	if owner != "cpave3" || repo != "staccato" {
		t.Errorf("expected cpave3/staccato, got %s/%s", owner, repo)
	}
}

func TestParseRemoteURL_SSH(t *testing.T) {
	owner, repo, err := ParseRemoteURL("git@github.com:rexlabsio/wings-api.git")
	if err != nil {
		t.Fatal(err)
	}
	if owner != "rexlabsio" || repo != "wings-api" {
		t.Errorf("expected rexlabsio/wings-api, got %s/%s", owner, repo)
	}
}

func TestParseRemoteURL_Invalid(t *testing.T) {
	_, _, err := ParseRemoteURL("not-a-url")
	if err == nil {
		t.Error("expected error for invalid URL")
	}
}

// Helper to build a test graph: main -> feat-a -> feat-b
func testGraph() *graph.Graph {
	g := graph.NewGraph("main")
	g.AddBranch("feat-a", "main", "aaa", "bbb")
	g.AddBranch("feat-b", "feat-a", "ccc", "ddd")
	return g
}

// Helper to build a graph with two sibling stacks:
//
//	main -> feat-a -> feat-b (stack 1)
//	main -> unrelated-1      (stack 2)
func testGraphSiblingStacks() *graph.Graph {
	g := graph.NewGraph("main")
	g.AddBranch("feat-a", "main", "aaa", "bbb")
	g.AddBranch("feat-b", "feat-a", "ccc", "ddd")
	g.AddBranch("unrelated-1", "main", "eee", "fff")
	return g
}

func TestResolveBranches_ScopeAll_OnlyIncludesCurrentLineage(t *testing.T) {
	g := testGraphSiblingStacks()
	// On feat-b: should get feat-a and feat-b, NOT unrelated-1
	branches := ResolveBranches(g, "feat-b", ScopeAll)
	if len(branches) != 2 {
		t.Fatalf("expected 2 branches, got %d: %v", len(branches), branches)
	}
	for _, b := range branches {
		if b == "unrelated-1" {
			t.Errorf("should not include unrelated-1 in lineage of feat-b")
		}
	}
}

func TestResolveBranches_ScopeAll_OtherStack(t *testing.T) {
	g := testGraphSiblingStacks()
	// On unrelated-1: should get only unrelated-1, NOT feat-a or feat-b
	branches := ResolveBranches(g, "unrelated-1", ScopeAll)
	if len(branches) != 1 {
		t.Fatalf("expected 1 branch, got %d: %v", len(branches), branches)
	}
	if branches[0] != "unrelated-1" {
		t.Errorf("expected [unrelated-1], got %v", branches)
	}
}

func TestResolveBranches_ScopeAll_SingleLineage(t *testing.T) {
	g := testGraph()
	branches := ResolveBranches(g, "feat-b", ScopeAll)
	if len(branches) != 2 {
		t.Fatalf("expected 2 branches, got %d: %v", len(branches), branches)
	}
}

func TestResolveBranches_ScopeCurrent(t *testing.T) {
	g := testGraph()
	branches := ResolveBranches(g, "feat-a", ScopeCurrent)
	if len(branches) != 1 || branches[0] != "feat-a" {
		t.Fatalf("expected [feat-a], got %v", branches)
	}
}

func TestResolveBranches_ScopeToCurrent(t *testing.T) {
	g := testGraph()
	branches := ResolveBranches(g, "feat-b", ScopeToCurrent)
	// Should include feat-a and feat-b (not main, which is root/trunk)
	if len(branches) != 2 {
		t.Fatalf("expected 2 branches, got %d: %v", len(branches), branches)
	}
	// Should be in ancestor order
	if branches[0] != "feat-a" || branches[1] != "feat-b" {
		t.Errorf("expected [feat-a, feat-b], got %v", branches)
	}
}

func TestTruncateDiffHunk_ShortHunk(t *testing.T) {
	hunk := "@@ -10,6 +10,8 @@\n line1\n line2\n line3"
	result := truncateDiffHunk(hunk, 20)
	if result != hunk {
		t.Errorf("short hunk should be unchanged, got %q", result)
	}
}

func TestTruncateDiffHunk_LongHunk(t *testing.T) {
	var lines []string
	for i := range 50 {
		lines = append(lines, fmt.Sprintf("+line %d", i))
	}
	hunk := strings.Join(lines, "\n")
	result := truncateDiffHunk(hunk, 20)
	resultLines := strings.Split(result, "\n")
	if len(resultLines) != 20 {
		t.Errorf("expected 20 lines, got %d", len(resultLines))
	}
	// Should keep the tail
	if !strings.Contains(result, "+line 49") {
		t.Error("expected last line to be preserved")
	}
}

func TestCleanBotBody_StripsDetailsBlocks(t *testing.T) {
	body := "Main point.\n\n<details>\n<summary>Tools</summary>\nstuff\n</details>\n\nMore text."
	result := cleanBotBody(body)
	if strings.Contains(result, "<details>") {
		t.Error("should strip <details> blocks")
	}
	if !strings.Contains(result, "Main point.") {
		t.Error("should keep main content")
	}
	if !strings.Contains(result, "More text.") {
		t.Error("should keep content after details block")
	}
}

func TestCleanBotBody_StripsHTMLComments(t *testing.T) {
	body := "Good point.\n\n<!-- fingerprinting:phantom:medusa:hawk -->\n\n<!-- This is an auto-generated comment by CodeRabbit -->"
	result := cleanBotBody(body)
	if strings.Contains(result, "fingerprinting") {
		t.Error("should strip HTML comments")
	}
	if !strings.Contains(result, "Good point.") {
		t.Error("should keep main content")
	}
}

func TestCleanBotBody_StripsNestedDetails(t *testing.T) {
	body := `Review comment.

<details>
<summary>Tools</summary>

<details>
<summary>Inner</summary>
nested stuff
</details>

</details>

End.`
	result := cleanBotBody(body)
	if strings.Contains(result, "<details>") {
		t.Error("should strip nested details blocks")
	}
	if !strings.Contains(result, "Review comment.") {
		t.Error("should keep leading content")
	}
}

func TestCleanBotBody_StripsCodeRabbitSuggestionBlocks(t *testing.T) {
	body := `Good suggestion.

<!-- suggestion_start -->

<details>
<summary>Committable suggestion</summary>

` + "```suggestion\ncode here\n```" + `

</details>

<!-- suggestion_end -->`
	result := cleanBotBody(body)
	if strings.Contains(result, "suggestion_start") {
		t.Error("should strip suggestion blocks")
	}
	if !strings.Contains(result, "Good suggestion.") {
		t.Error("should keep main content")
	}
}

func TestCleanBotBody_StripsOrphanedClosingTags(t *testing.T) {
	body := "Good point.\n\n</blockquote></details>\n\n</blockquote></details>\n\n---"
	result := cleanBotBody(body)
	if strings.Contains(result, "</details>") || strings.Contains(result, "</blockquote>") {
		t.Errorf("should strip orphaned closing tags, got %q", result)
	}
	if !strings.Contains(result, "Good point.") {
		t.Error("should keep main content")
	}
}

func TestCleanBotBody_StripsHTMLHeadingsAndSub(t *testing.T) {
	body := "<h3>Greptile Summary</h3>\n\nThis PR does things.\n\n<sub>Last reviewed commit: abc123</sub>"
	result := cleanBotBody(body)
	if strings.Contains(result, "<h3>") || strings.Contains(result, "</h3>") {
		t.Error("should strip h3 tags")
	}
	if strings.Contains(result, "<sub>") {
		t.Error("should strip sub tags")
	}
	if !strings.Contains(result, "Greptile Summary") {
		t.Error("should keep text content from headings")
	}
}

func TestFilterNoise_RemovesBotInvocations(t *testing.T) {
	items := []FeedbackItem{
		{Author: "dev1", AuthorType: "Human", Body: "@coderabbitai review"},
		{Author: "dev1", AuthorType: "Human", Body: "@greptile please review"},
		{Author: "dev1", AuthorType: "Human", Body: "@Coderabbitai review again"},
		{Author: "dev1", AuthorType: "Human", Body: "actual feedback here"},
	}
	filtered := FilterNoise(items)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 item, got %d", len(filtered))
	}
	if filtered[0].Body != "actual feedback here" {
		t.Errorf("expected real feedback, got %q", filtered[0].Body)
	}
}

func TestFilterNoise_KeepsBotComments(t *testing.T) {
	items := []FeedbackItem{
		{Author: "coderabbitai[bot]", AuthorType: "Bot", Body: "Review comment."},
	}
	filtered := FilterNoise(items)
	if len(filtered) != 1 {
		t.Fatal("should keep bot review comments")
	}
}

func TestFilterNoise_RemovesBotTally(t *testing.T) {
	items := []FeedbackItem{
		{Author: "coderabbitai[bot]", AuthorType: "Bot", Body: "**Actionable comments posted: 4**\n\n---"},
	}
	filtered := FilterNoise(items)
	if len(filtered) != 0 {
		t.Fatalf("expected 0 items, got %d", len(filtered))
	}
}

func TestFilterNoise_RemovesReviewSkipped(t *testing.T) {
	items := []FeedbackItem{
		{Author: "coderabbitai[bot]", AuthorType: "Bot", Body: "> [!IMPORTANT]\n> ## Review skipped\n> Draft detected."},
	}
	filtered := FilterNoise(items)
	if len(filtered) != 0 {
		t.Fatalf("expected 0 items, got %d", len(filtered))
	}
}

func TestFilterNoise_StripsLastReviewedCommit(t *testing.T) {
	items := []FeedbackItem{
		{Author: "greptile-apps[bot]", AuthorType: "Bot", Body: "Good analysis here.\n\nLast reviewed commit: 554aa95"},
	}
	filtered := FilterNoise(items)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 item, got %d", len(filtered))
	}
	if strings.Contains(filtered[0].Body, "Last reviewed commit") {
		t.Error("should strip 'Last reviewed commit' line")
	}
	if !strings.Contains(filtered[0].Body, "Good analysis here.") {
		t.Error("should keep main content")
	}
}

func TestCleanBotBody_EmptyAfterCleaning(t *testing.T) {
	body := "<!-- just a comment -->"
	result := cleanBotBody(body)
	if strings.TrimSpace(result) != "" {
		t.Errorf("expected empty string, got %q", result)
	}
}

func TestFormatMarkdown_Structure(t *testing.T) {
	result := ReviewResult{
		Items: []FeedbackItem{
			{
				PR: 123, Author: "dev1", AuthorType: "Human",
				Type: "inline", File: "main.go", Line: 42,
				Body: "fix this null check", DiffHunk: "@@ -10,6 +10,8 @@",
			},
			{
				PR: 123, Author: "coderabbitai[bot]", AuthorType: "Bot",
				Type: "review", Body: "overall looks good",
			},
			{
				PR: 456, Author: "dev2", AuthorType: "Human",
				Type: "general", Body: "what about edge cases?",
			},
		},
		RepoOwner: "myorg",
		RepoName:  "myrepo",
	}
	md := FormatMarkdown(result)
	if !strings.Contains(md, "# PR Review Feedback") {
		t.Error("missing title")
	}
	if !strings.Contains(md, "PR #123") {
		t.Error("missing PR #123 section")
	}
	if !strings.Contains(md, "PR #456") {
		t.Error("missing PR #456 section")
	}
	if !strings.Contains(md, "dev1") {
		t.Error("missing author dev1")
	}
	if !strings.Contains(md, "`main.go:42`") {
		t.Error("missing file:line reference")
	}
	if !strings.Contains(md, "CLASSIFICATION") {
		t.Error("missing classification prompt section")
	}
}
