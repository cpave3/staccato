package mcp

import (
	"os"
	"strings"
	"testing"
)

func TestRunToolRefusesStExec(t *testing.T) {
	// st_run starts the running binary, which here is this test binary. If
	// the refusal ever regresses, that would run the tests again from inside
	// this one; the variable stops it after one level.
	const guard = "ST_MCP_RUN_TOOL_TEST"
	if os.Getenv(guard) != "" {
		t.Skip("started by st_run from inside this test")
	}
	t.Setenv(guard, "1")

	_, sc, _ := setupSplitRepo(t)

	// 'st exec' runs an arbitrary shell command, which is more than approving
	// "run an st command" should grant.
	for _, command := range []string{
		"exec make",
		"exec --fail-fast -- make build",
		"-v exec make",
		"--verbose exec make",
	} {
		text, isErr := callTool(t, runHandler(sc), map[string]any{"command": command})
		if !isErr || !strings.Contains(text, "st exec") {
			t.Errorf("st_run(%q) = %q (error=%v), want st exec refused", command, text, isErr)
		}
	}
}
