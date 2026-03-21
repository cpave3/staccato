package hooks

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Event represents a hook lifecycle event.
type Event string

const (
	PostPRCreate        Event = "post-pr-create"
	PostPRView          Event = "post-pr-view"
	PostBranchCreate    Event = "post-branch-create"
	PostBranchDelete    Event = "post-branch-delete"
	PostRestack         Event = "post-restack"
	PostRestackConflict Event = "post-restack-conflict"
	PostSync            Event = "post-sync"
	PostAttach          Event = "post-attach"
	PreSync             Event = "pre-sync"
	PreRestack          Event = "pre-restack"
)

// DefaultTimeout is the maximum duration a hook script can run.
const DefaultTimeout = 30 * time.Second

// Context is the JSON payload passed to hook scripts via stdin.
type Context struct {
	Event    Event          `json:"event"`
	RepoPath string        `json:"repo_path"`
	Branch   string        `json:"branch,omitempty"`
	Data     map[string]any `json:"data,omitempty"`
}

// Runner discovers and executes hooks.
type Runner struct {
	repoPath string
}

// NewRunner creates a hook runner for the given repository root.
func NewRunner(repoPath string) *Runner {
	return &Runner{repoPath: repoPath}
}

// Fire discovers and runs all hooks for the given event.
// For pre-* events, returns an error if any hook exits with code 2 (blocking).
// For post-* events, exit code 2 is treated as a warning (printed, not returned).
func (r *Runner) Fire(ctx Context) error {
	scripts := r.discover(ctx.Event)
	if len(scripts) == 0 {
		return nil
	}

	isPre := strings.HasPrefix(string(ctx.Event), "pre-")

	payload, err := json.Marshal(ctx)
	if err != nil {
		return fmt.Errorf("failed to marshal hook context: %w", err)
	}

	env := append(os.Environ(),
		"ST_EVENT="+string(ctx.Event),
		"ST_REPO_PATH="+ctx.RepoPath,
		"ST_BRANCH="+ctx.Branch,
	)

	for _, script := range scripts {
		if err := r.run(script, payload, env, isPre); err != nil {
			return err
		}
	}

	return nil
}

// discover finds all executable scripts for an event across both hook directories
// (global first, then project).
func (r *Runner) discover(event Event) []string {
	var scripts []string

	dirs := r.hookDirs(event)
	for _, dir := range dirs {
		found := discoverInDir(dir)
		scripts = append(scripts, found...)
	}

	return scripts
}

// hookDirs returns the ordered list of directories to scan for an event.
func (r *Runner) hookDirs(event Event) []string {
	var dirs []string

	// Global hooks: ~/.config/staccato/hooks/<event>/
	if home, err := os.UserHomeDir(); err == nil {
		globalDir := filepath.Join(home, ".config", "staccato", "hooks", string(event))
		dirs = append(dirs, globalDir)
	}

	// Project hooks: <repo>/.staccato/hooks/<event>/
	if r.repoPath != "" {
		projectDir := filepath.Join(r.repoPath, ".staccato", "hooks", string(event))
		dirs = append(dirs, projectDir)
	}

	return dirs
}

// discoverInDir returns executable files in a directory.
func discoverInDir(dir string) []string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil // directory doesn't exist or can't be read — not an error
	}

	var scripts []string
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		if !isExecutable(info) {
			continue
		}
		scripts = append(scripts, filepath.Join(dir, entry.Name()))
	}

	return scripts
}

// isExecutable checks if a file has any execute permission bit set.
func isExecutable(info fs.FileInfo) bool {
	return info.Mode()&0111 != 0
}

// run executes a single hook script. Returns an error only if isPre and the script exits 2.
func (r *Runner) run(script string, payload []byte, env []string, isPre bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), DefaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, script)
	cmd.Stdin = bytes.NewReader(payload)
	cmd.Env = env
	cmd.Dir = r.repoPath

	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	cmd.Stdout = os.Stdout

	err := cmd.Run()

	if ctx.Err() == context.DeadlineExceeded {
		fmt.Fprintf(os.Stderr, "hook timeout: %s (killed after %s)\n", filepath.Base(script), DefaultTimeout)
		return nil
	}

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			code := exitErr.ExitCode()
			if code == 2 {
				msg := strings.TrimSpace(stderr.String())
				if msg == "" {
					msg = fmt.Sprintf("blocked by hook: %s", filepath.Base(script))
				}
				if isPre {
					return fmt.Errorf("%s", msg)
				}
				// Post-hook exit 2: warn only
				fmt.Fprintf(os.Stderr, "hook warning: %s: %s\n", filepath.Base(script), msg)
				return nil
			}
			// Other non-zero exit: warn and continue
			if msg := strings.TrimSpace(stderr.String()); msg != "" {
				fmt.Fprintf(os.Stderr, "hook warning: %s: %s\n", filepath.Base(script), msg)
			}
			return nil
		}
		// exec error (e.g., permission denied after discovery race)
		fmt.Fprintf(os.Stderr, "hook error: %s: %v\n", filepath.Base(script), err)
	}

	return nil
}
