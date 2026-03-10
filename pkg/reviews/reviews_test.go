package reviews

import (
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

func TestResolveBranches_ScopeAll(t *testing.T) {
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
