package output

import (
	"io"
	"os"
	"strings"
	"testing"

	"github.com/cpave3/staccato/pkg/forge"
	"github.com/cpave3/staccato/pkg/graph"
)

// captureStdout runs fn and returns whatever it wrote to os.Stdout.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}

	orig := os.Stdout
	os.Stdout = w

	fn()

	w.Close()
	os.Stdout = orig

	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("ReadAll: %v", err)
	}
	return string(out)
}

// --- 1.2 NewPrinter and SetVerbose ---

func TestNewPrinter(t *testing.T) {
	p := NewPrinter(false)
	if p.verbose {
		t.Error("expected verbose=false")
	}
	p.SetVerbose(true)
	if !p.verbose {
		t.Error("expected verbose=true after SetVerbose(true)")
	}
}

// --- 2.1 Success ---

func TestSuccess(t *testing.T) {
	p := NewPrinter(false)
	out := captureStdout(t, func() {
		p.Success("branch '%s' created", "feat")
	})
	expected := "✔ branch 'feat' created\n"
	if out != expected {
		t.Errorf("got %q, want %q", out, expected)
	}
}

// --- 2.2 Warning ---

func TestWarning(t *testing.T) {
	p := NewPrinter(false)
	out := captureStdout(t, func() {
		p.Warning("conflicts detected")
	})
	expected := "⚠ conflicts detected\n"
	if out != expected {
		t.Errorf("got %q, want %q", out, expected)
	}
}

// --- 2.3 Error ---

func TestError(t *testing.T) {
	p := NewPrinter(false)
	out := captureStdout(t, func() {
		p.Error("failed to rebase")
	})
	expected := "✘ failed to rebase\n"
	if out != expected {
		t.Errorf("got %q, want %q", out, expected)
	}
}

// --- 2.4 Info verbose ---

func TestInfo_Verbose(t *testing.T) {
	p := NewPrinter(true)
	out := captureStdout(t, func() {
		p.Info("fetching...")
	})
	expected := "ℹ fetching...\n"
	if out != expected {
		t.Errorf("got %q, want %q", out, expected)
	}
}

// --- 2.5 Info suppressed ---

func TestInfo_Suppressed(t *testing.T) {
	p := NewPrinter(false)
	out := captureStdout(t, func() {
		p.Info("fetching...")
	})
	if out != "" {
		t.Errorf("expected no output, got %q", out)
	}
}

// --- 2.6 Print ---

func TestPrint(t *testing.T) {
	p := NewPrinter(false)
	out := captureStdout(t, func() {
		p.Print("hello %s", "world")
	})
	if out != "hello world" {
		t.Errorf("got %q, want %q", out, "hello world")
	}
}

// --- 2.7 Println ---

func TestPrintln(t *testing.T) {
	p := NewPrinter(false)
	out := captureStdout(t, func() {
		p.Println("count: %d", 42)
	})
	if out != "count: 42\n" {
		t.Errorf("got %q, want %q", out, "count: 42\n")
	}
}

// --- 3.1 BranchCreated ---

func TestBranchCreated(t *testing.T) {
	p := NewPrinter(false)
	out := captureStdout(t, func() {
		p.BranchCreated("feat-a", "main")
	})
	if !strings.Contains(out, "✔") {
		t.Error("expected success icon")
	}
	if !strings.Contains(out, "feat-a") || !strings.Contains(out, "main") {
		t.Errorf("expected branch names in output, got %q", out)
	}
}

// --- 3.2 RestackComplete ---

func TestRestackComplete(t *testing.T) {
	p := NewPrinter(false)
	out := captureStdout(t, func() {
		p.RestackComplete(3)
	})
	if !strings.Contains(out, "✔") {
		t.Error("expected success icon")
	}
	if !strings.Contains(out, "3") {
		t.Errorf("expected count in output, got %q", out)
	}
}

// --- 3.3 ConflictDetected ---

func TestConflictDetected(t *testing.T) {
	p := NewPrinter(false)
	out := captureStdout(t, func() {
		p.ConflictDetected("feat-b")
	})
	if !strings.Contains(out, "⚠") {
		t.Error("expected warning icon")
	}
	if !strings.Contains(out, "feat-b") {
		t.Error("expected branch name in output")
	}
	if !strings.Contains(out, "st continue") {
		t.Error("expected 'st continue' instruction")
	}
	if !strings.Contains(out, "st restore") {
		t.Error("expected 'st restore' instruction")
	}
}

// --- 4.1 StackLog tree rendering ---

func TestStackLog_Tree(t *testing.T) {
	g := graph.NewGraph("main")
	g.AddBranch("child", "main", "aaa", "bbb")
	g.AddBranch("grandchild", "child", "ccc", "ddd")

	p := NewPrinter(false)
	out := captureStdout(t, func() {
		p.StackLog(g, "other")
	})

	lines := strings.Split(out, "\n")
	// Find lines with branch names
	var branchLines []string
	for _, l := range lines {
		if strings.Contains(l, "main") || strings.Contains(l, "child") {
			branchLines = append(branchLines, l)
		}
	}
	if len(branchLines) < 3 {
		t.Fatalf("expected 3 branch lines, got %d: %q", len(branchLines), out)
	}
	// Root should have no indentation, child one level, grandchild two levels
	if strings.HasPrefix(branchLines[0], "  ") {
		t.Error("root should not be indented")
	}
	if !strings.HasPrefix(branchLines[1], "  ") {
		t.Error("child should be indented one level")
	}
	if !strings.HasPrefix(branchLines[2], "    ") {
		t.Error("grandchild should be indented two levels")
	}
}

// --- 4.2 StackLog highlights current branch ---

func TestStackLog_CurrentBranch(t *testing.T) {
	g := graph.NewGraph("main")
	g.AddBranch("feat", "main", "aaa", "bbb")

	p := NewPrinter(false)
	out := captureStdout(t, func() {
		p.StackLog(g, "feat")
	})

	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "feat") && !strings.Contains(line, "main") {
			if !strings.Contains(line, "●") {
				t.Errorf("expected ● for current branch, got: %q", line)
			}
		}
		if strings.Contains(line, "main") && !strings.Contains(line, "feat") {
			if !strings.Contains(line, "○") {
				t.Errorf("expected ○ for non-current branch, got: %q", line)
			}
		}
	}
}

// --- 5.1 formatPRStatus MERGED ---

func TestFormatPRStatus_Merged(t *testing.T) {
	info := &forge.PRStatusInfo{
		HasPR:  true,
		Number: 42,
		State:  "MERGED",
	}
	got := formatPRStatus(info)
	expected := "#42 ✔ Merged"
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

// --- 5.2 formatPRStatus CLOSED ---

func TestFormatPRStatus_Closed(t *testing.T) {
	info := &forge.PRStatusInfo{
		HasPR:  true,
		Number: 7,
		State:  "CLOSED",
	}
	got := formatPRStatus(info)
	expected := "#7 ✘ Closed"
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

// --- 5.3 formatPRStatus OPEN variants ---

func TestFormatPRStatus_OpenDraft(t *testing.T) {
	info := &forge.PRStatusInfo{
		HasPR:   true,
		Number:  5,
		State:   "OPEN",
		IsDraft: true,
	}
	got := formatPRStatus(info)
	expected := "#5 Draft"
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestFormatPRStatus_OpenApproved(t *testing.T) {
	info := &forge.PRStatusInfo{
		HasPR:        true,
		Number:       10,
		State:        "OPEN",
		ReviewStatus: "APPROVED",
	}
	got := formatPRStatus(info)
	expected := "#10 ✔ Approved"
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestFormatPRStatus_OpenApprovedCIFail(t *testing.T) {
	info := &forge.PRStatusInfo{
		HasPR:        true,
		Number:       10,
		State:        "OPEN",
		ReviewStatus: "APPROVED",
		CheckStatus:  "fail",
	}
	got := formatPRStatus(info)
	expected := "#10 ✔ Approved | CI ✘"
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestFormatPRStatus_OpenChangesRequested(t *testing.T) {
	info := &forge.PRStatusInfo{
		HasPR:        true,
		Number:       15,
		State:        "OPEN",
		ReviewStatus: "CHANGES_REQUESTED",
	}
	got := formatPRStatus(info)
	expected := "#15 ⚠ Changes requested"
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestFormatPRStatus_OpenPending(t *testing.T) {
	info := &forge.PRStatusInfo{
		HasPR:  true,
		Number: 20,
		State:  "OPEN",
	}
	got := formatPRStatus(info)
	expected := "#20 Review pending"
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}

func TestFormatPRStatus_OpenCIPending(t *testing.T) {
	info := &forge.PRStatusInfo{
		HasPR:       true,
		Number:      25,
		State:       "OPEN",
		CheckStatus: "pending",
	}
	got := formatPRStatus(info)
	expected := "#25 Review pending | CI pending"
	if got != expected {
		t.Errorf("got %q, want %q", got, expected)
	}
}
