package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// scripts/next-version.sh is how the release workflow names a release: the
// newest stable tag, bumped.
func TestNextReleaseVersion(t *testing.T) {
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "next-version.sh"))
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name, bump, want string
		tags             []string
	}{
		{"first release", "patch", "v0.0.1", nil},
		{"first minor", "minor", "v0.1.0", nil},
		{"patch", "patch", "v1.4.3", []string{"v1.4.2", "v1.3.9"}},
		{"minor resets patch", "minor", "v1.5.0", []string{"v1.4.2"}},
		{"major resets both", "major", "v2.0.0", []string{"v1.4.2"}},
		{"newest by number, not by name", "patch", "v0.10.1", []string{"v0.9.0", "v0.10.0", "v0.2.0"}},
		{"prereleases and other tags ignored", "patch", "v0.2.1", []string{"v0.2.0", "v0.3.0-rc1", "v9", "legacy"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			repo := t.TempDir()
			releaseGit(t, repo, "init", "-q")
			releaseGit(t, repo, "-c", "user.name=t", "-c", "user.email=t@t", "commit", "-q", "--allow-empty", "-m", "c")
			for _, tag := range c.tags {
				releaseGit(t, repo, "tag", tag)
			}
			cmd := exec.Command("bash", script, c.bump)
			cmd.Dir = repo
			out, err := cmd.CombinedOutput()
			if err != nil || strings.TrimSpace(string(out)) != c.want {
				t.Errorf("next-version.sh %s = %q (%v), want %s", c.bump, out, err, c.want)
			}
		})
	}
}

func TestNextReleaseVersionRefusesAnUnknownBump(t *testing.T) {
	script := filepath.Join("..", "..", "scripts", "next-version.sh")
	if out, err := exec.Command("bash", script, "huge").CombinedOutput(); err == nil {
		t.Errorf("accepted bump huge: %s", out)
	}
}

// releaseGit runs git in dir with no user or system configuration.
func releaseGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CONFIG_GLOBAL=/dev/null", "GIT_CONFIG_SYSTEM=/dev/null")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
