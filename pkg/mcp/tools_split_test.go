package mcp

import (
	"context"
	"encoding/json"
	"regexp"
	"strings"
	"testing"

	"github.com/cpave3/staccato/internal/testutil"
	stcontext "github.com/cpave3/staccato/pkg/context"
	"github.com/cpave3/staccato/pkg/split"
	"github.com/mark3labs/mcp-go/mcp"
)

// setupSplitRepo builds a repo whose tracked branch "feature" adds core/ and
// api/ files in one commit, and returns the context an MCP server would hold.
func setupSplitRepo(t *testing.T) (*testutil.GitRepo, *stcontext.StaccatoContext, string) {
	t.Helper()
	repo, err := testutil.NewGitRepo()
	if err != nil {
		t.Fatalf("NewGitRepo: %v", err)
	}
	t.Cleanup(repo.Cleanup)
	if err := repo.InitStack(); err != nil {
		t.Fatalf("InitStack: %v", err)
	}
	root := git(t, repo, "rev-parse", "--abbrev-ref", "HEAD")

	// The server starts before the branch exists in the graph.
	server, err := stcontext.Load(repo.Dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}

	base := repo.HeadSHA()
	if err := repo.CreateBranch("feature"); err != nil {
		t.Fatal(err)
	}
	repo.WriteFile("core/types.txt", "types\n")
	repo.WriteFile("api/handler.txt", "handler\n")
	if err := repo.AddAndCommit("add core and api"); err != nil {
		t.Fatal(err)
	}
	// Another process (the CLI) tracks the branch.
	cli, err := stcontext.Load(repo.Dir)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	cli.Graph.Root = root
	cli.Graph.AddBranch("feature", root, base, repo.HeadSHA())
	if err := cli.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	return repo, server, root
}

func git(t *testing.T, repo *testutil.GitRepo, args ...string) string {
	t.Helper()
	out, err := repo.RunGit(args...)
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
	return out
}

func callTool(t *testing.T, handler func(context.Context, mcp.CallToolRequest) (*mcp.CallToolResult, error), args map[string]any) (string, bool) {
	t.Helper()
	req := mcp.CallToolRequest{}
	req.Params.Arguments = args
	result, err := handler(context.Background(), req)
	if err != nil {
		t.Fatalf("handler returned a Go error: %v", err)
	}
	text, ok := result.Content[0].(mcp.TextContent)
	if !ok {
		t.Fatalf("result content is %T, want text", result.Content[0])
	}
	return text.Text, result.IsError
}

func splitPlan() map[string]any {
	return map[string]any{
		"branch": "feature",
		"slices": []any{
			map[string]any{"name": "feature-core", "message": "core", "paths": []any{"core/"}},
			map[string]any{"name": "feature", "message": "api", "rest": true},
		},
	}
}

func TestSplitDescribeTool(t *testing.T) {
	repo, sc, root := setupSplitRepo(t)

	// The branch was tracked after the server loaded its graph, so this also
	// proves the handler reloads it.
	text, isErr := callTool(t, splitDescribeHandler(sc), map[string]any{"branch": "feature"})
	if isErr {
		t.Fatalf("describe failed: %s", text)
	}
	var d split.Description
	if err := json.Unmarshal([]byte(text), &d); err != nil {
		t.Fatalf("describe output is not JSON: %v\n%s", err, text)
	}
	if d.Branch != "feature" || d.Parent != root || d.Head != repo.HeadSHA() || len(d.Files) != 2 {
		t.Errorf("description = %+v", d)
	}
}

func TestSplitApplyTool(t *testing.T) {
	t.Run("applies_a_plan_given_as_an_object", func(t *testing.T) {
		repo, sc, root := setupSplitRepo(t)

		text, isErr := callTool(t, splitApplyHandler(sc), map[string]any{"plan": splitPlan()})
		if isErr {
			t.Fatalf("apply failed: %s", text)
		}
		var r split.Result
		if err := json.Unmarshal([]byte(text), &r); err != nil {
			t.Fatalf("apply output is not JSON: %v\n%s", err, text)
		}
		if !r.Applied || len(r.Slices) != 2 {
			t.Errorf("result = %+v", r)
		}
		if !repo.BranchExists("feature-core") {
			t.Error("feature-core was not created")
		}
		if files := git(t, repo, "diff", "--name-only", root, "feature-core"); files != "core/types.txt" {
			t.Errorf("feature-core changes %q, want core/types.txt", files)
		}
		// The server's own graph follows, so st_log and friends stay right.
		if b, ok := sc.Graph.GetBranch("feature"); !ok || b.Parent != "feature-core" {
			t.Errorf("server graph has feature = %+v, want it on feature-core", b)
		}
	})

	t.Run("accepts_the_plan_as_a_json_string", func(t *testing.T) {
		repo, sc, _ := setupSplitRepo(t)
		encoded, _ := json.Marshal(splitPlan())

		text, isErr := callTool(t, splitApplyHandler(sc), map[string]any{"plan": string(encoded)})
		if isErr {
			t.Fatalf("apply failed: %s", text)
		}
		if !repo.BranchExists("feature-core") {
			t.Error("feature-core was not created")
		}
	})

	t.Run("dry_run_moves_nothing", func(t *testing.T) {
		repo, sc, _ := setupSplitRepo(t)
		before := repo.HeadSHA()

		text, isErr := callTool(t, splitApplyHandler(sc), map[string]any{"plan": splitPlan(), "dry_run": true})
		if isErr {
			t.Fatalf("dry run failed: %s", text)
		}
		if !strings.Contains(text, `"dry_run": true`) {
			t.Errorf("result does not report a dry run:\n%s", text)
		}
		if repo.BranchExists("feature-core") || git(t, repo, "rev-parse", "feature") != before {
			t.Error("dry run moved branches")
		}
	})

	t.Run("rejects_a_misspelt_key", func(t *testing.T) {
		repo, sc, _ := setupSplitRepo(t)
		plan := splitPlan()
		plan["slices"].([]any)[0].(map[string]any)["path"] = []any{"core/"}

		text, isErr := callTool(t, splitApplyHandler(sc), map[string]any{"plan": plan})
		if !isErr || !strings.Contains(text, `"path"`) {
			t.Errorf("result = %q (error=%v), want an error naming the unknown field", text, isErr)
		}
		if repo.BranchExists("feature-core") {
			t.Error("a rejected plan created a branch")
		}
	})

	t.Run("requires_the_branch_to_be_named", func(t *testing.T) {
		_, sc, _ := setupSplitRepo(t)
		plan := splitPlan()
		delete(plan, "branch")

		// The server may sit in a different worktree from the agent, so
		// "the current branch" is not safe to assume.
		text, isErr := callTool(t, splitApplyHandler(sc), map[string]any{"plan": plan})
		if !isErr || !strings.Contains(text, `"branch"`) {
			t.Errorf("result = %q (error=%v), want an error asking for the branch", text, isErr)
		}
	})

	t.Run("reports_a_failed_plan_as_a_tool_error", func(t *testing.T) {
		_, sc, _ := setupSplitRepo(t)
		plan := splitPlan()
		plan["slices"].([]any)[1] = map[string]any{"name": "feature", "message": "api", "paths": []any{"nope/"}}

		text, isErr := callTool(t, splitApplyHandler(sc), map[string]any{"plan": plan})
		if !isErr || !strings.Contains(text, "matches no file changed") {
			t.Errorf("result = %q (error=%v), want the plan's error", text, isErr)
		}
	})
}

func TestRunToolRefusesExec(t *testing.T) {
	_, sc, _ := setupSplitRepo(t)

	// --exec runs an arbitrary shell command, which is more than approving
	// "run an st command" should grant.
	text, isErr := callTool(t, runHandler(sc), map[string]any{"command": "split apply plan.json --exec make"})
	if !isErr || !strings.Contains(text, "--exec") {
		t.Errorf("result = %q (error=%v), want --exec refused", text, isErr)
	}
	text, isErr = callTool(t, runHandler(sc), map[string]any{"command": "split apply plan.json --exec=make"})
	if !isErr || !strings.Contains(text, "--exec") {
		t.Errorf("result = %q (error=%v), want --exec= refused", text, isErr)
	}
}

func TestSplitToolsAreRegistered(t *testing.T) {
	_, sc, _ := setupSplitRepo(t)
	tools := NewServer(sc).ListTools()

	describe, apply := tools["st_split_describe"], tools["st_split_apply"]
	if describe == nil || apply == nil {
		t.Fatalf("split tools missing from the server: describe=%v apply=%v", describe != nil, apply != nil)
	}
	// The schema is what tells a client how to shape the nested plan.
	schema, err := json.Marshal(apply.Tool.InputSchema)
	if err != nil {
		t.Fatalf("marshal schema: %v", err)
	}
	for _, want := range []string{`"plan"`, `"slices"`, `"paths"`, `"commits"`, `"overrides"`, `"drop_original"`, `"dry_run"`} {
		if !strings.Contains(string(schema), want) {
			t.Errorf("st_split_apply schema does not mention %s:\n%s", want, schema)
		}
	}
	if strings.Contains(string(schema), `"exec"`) {
		t.Error("st_split_apply exposes exec, which would let an approved tool run arbitrary commands")
	}
}

// The prompts are what an agent follows, so a tool they name has to exist.
func TestPromptsOnlyNameRegisteredTools(t *testing.T) {
	_, sc, _ := setupSplitRepo(t)
	tools := NewServer(sc).ListTools()

	named := regexp.MustCompile(`\bst_[a-z_]+\b`)
	for name, text := range map[string]string{
		"split-monolithic-pr": splitPromptTemplate,
		"learn-staccato":      learnPromptContent,
	} {
		for _, tool := range named.FindAllString(text, -1) {
			if tools[tool] == nil {
				t.Errorf("prompt %s names %s, which the server does not expose", name, tool)
			}
		}
	}
}
