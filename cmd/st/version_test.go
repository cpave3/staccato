package main

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// stampedBinaries holds st built with a version stamped in, by version, built
// once per test run as a release build is.
var (
	stampedMu       sync.Mutex
	stampedBinaries = map[string]string{}
)

// stampedBinary returns the path of an st binary built with version stamped
// in. Callers that may change the file (st update) copy it first.
func stampedBinary(t *testing.T, version string) string {
	t.Helper()
	stampedMu.Lock()
	defer stampedMu.Unlock()
	if path, ok := stampedBinaries[version]; ok {
		return path
	}
	dir, err := os.MkdirTemp("", "st-stamped-*")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "st")
	build := exec.Command("go", "build",
		"-ldflags", "-X github.com/cpave3/staccato/pkg/version.stamped="+version,
		"-o", path, "./cmd/st/")
	build.Dir = filepath.Join(mustGetwd(), "..", "..")
	if out, err := build.CombinedOutput(); err != nil {
		t.Fatalf("building st %s: %v\n%s", version, err, out)
	}
	stampedBinaries[version] = path
	return path
}

// removeStampedBinaries deletes the binaries stampedBinary built.
func removeStampedBinaries() {
	stampedMu.Lock()
	defer stampedMu.Unlock()
	for _, path := range stampedBinaries {
		os.RemoveAll(filepath.Dir(path))
	}
}

// runBinary runs an st binary in dir and returns its output and error.
func runBinary(t *testing.T, binary, dir string, env []string, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(binary, args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestVersionOfABuildFromSource(t *testing.T) {
	out, err := runBinary(t, stBinary, t.TempDir(), nil, "version")
	if err != nil {
		t.Fatalf("st version: %v\n%s", err, out)
	}
	if strings.TrimSpace(out) != "st dev" {
		t.Errorf("st version = %q, want st dev", out)
	}

	out, err = runBinary(t, stBinary, t.TempDir(), nil, "version", "--json")
	if err != nil {
		t.Fatalf("st version --json: %v\n%s", err, out)
	}
	got := decodeVersion(t, out)
	if got.Version != "dev" || got.Release {
		t.Errorf("st version --json = %+v, want dev, not a release", got)
	}
}

func TestVersionOfAReleaseBuild(t *testing.T) {
	binary := stampedBinary(t, "v0.4.2")

	// Outside any git repository: st version reads no graph.
	out, err := runBinary(t, binary, t.TempDir(), nil, "version")
	if err != nil {
		t.Fatalf("st version: %v\n%s", err, out)
	}
	if strings.TrimSpace(out) != "st v0.4.2" {
		t.Errorf("st version = %q, want st v0.4.2", out)
	}

	out, err = runBinary(t, binary, t.TempDir(), nil, "version", "--json")
	if err != nil {
		t.Fatalf("st version --json: %v\n%s", err, out)
	}
	got := decodeVersion(t, out)
	if got.Version != "v0.4.2" || !got.Release {
		t.Errorf("st version --json = %+v, want v0.4.2, a release", got)
	}
}

type versionJSON struct {
	Version string `json:"version"`
	Release bool   `json:"release"`
}

func decodeVersion(t *testing.T, out string) versionJSON {
	t.Helper()
	var got versionJSON
	if err := json.Unmarshal([]byte(out), &got); err != nil {
		t.Fatalf("st version --json printed %q: %v", out, err)
	}
	return got
}
