package main

import (
	"os/exec"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// skill install
// ---------------------------------------------------------------------------

func TestSkillInstall_NpxNotFound(t *testing.T) {
	setupRepo(t)

	// Run with empty PATH so npx cannot be found.
	cmd := exec.Command(stBinary, "skill", "install")
	setCoverEnv(cmd)
	cmd.Env = append(cmd.Env, "PATH=")
	cmd.Env = append(cmd.Env, "HOME="+t.TempDir())
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatalf("expected error when npx not in PATH, got success\nOutput: %s", out)
	}
	if !strings.Contains(string(out), "npx") {
		t.Fatalf("expected error mentioning npx, got: %s", out)
	}
}
