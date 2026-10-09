package main

import (
	"encoding/json"
	"testing"

	"github.com/cpave3/staccato/pkg/forge"
	"github.com/cpave3/staccato/pkg/graph"
)

// ---------------------------------------------------------------------------
// TestStatus
// ---------------------------------------------------------------------------

func TestStatus(t *testing.T) {
	t.Run("errors_when_no_remote", func(t *testing.T) {
		setupRepoWithStack(t)
		runSt(t, "new", "f1")
		out := runStExpectError(t, "status")
		assertContains(t, out, "failed to get remote URL")
	})

	t.Run("errors_for_unsupported_forge", func(t *testing.T) {
		repo, _ := setupRepoWithStack(t)
		runSt(t, "new", "f1")
		repo.RunGit("remote", "add", "origin", "https://gitlab.com/user/repo.git")
		out := runStExpectError(t, "status")
		assertContains(t, out, "forge not supported")
	})
}

// 'st status --json' needs GitHub, so its tree is checked here without it.
func TestStatusTree(t *testing.T) {
	g := graph.NewGraph("main")
	g.AddBranch("api", "main", "", "")
	g.AddBranch("ui", "api", "", "")
	prs := map[string]*forge.PRStatusInfo{
		"api": {Branch: "api", HasPR: true, Number: 12, State: "OPEN", ReviewStatus: "APPROVED", CheckStatus: "pass", URL: "https://example.test/pull/12"},
		"ui":  {Branch: "ui"},
	}

	data, err := json.Marshal(statusTree(g, "main", "ui", prs))
	if err != nil {
		t.Fatal(err)
	}

	want := `{"branch":"main","children":[{"branch":"api","pr_number":12,"pr_state":"OPEN","review_status":"APPROVED","check_status":"pass","pr_url":"https://example.test/pull/12",` +
		`"children":[{"branch":"ui","current":true}]}]}`
	if string(data) != want {
		t.Errorf("status tree =\n%s\nwant\n%s", data, want)
	}
}
