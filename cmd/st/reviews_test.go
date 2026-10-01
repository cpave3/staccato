package main

import (
	"strings"
	"testing"
)

func TestReviews_NoBranches(t *testing.T) {
	repo := setupRepo(t)
	defer repo.Cleanup()
	repo.InitStack()

	// With no branches in the stack, should exit cleanly
	out := runSt(t, "reviews")
	if !strings.Contains(out, "no branches in scope") {
		t.Errorf("expected 'no branches in scope', got: %s", out)
	}
}

func TestReviews_FlagsRegistered(t *testing.T) {
	repo := setupRepo(t)
	defer repo.Cleanup()
	repo.InitStack()

	// Verify help shows the expected flags
	out := runSt(t, "reviews", "--help")
	if !strings.Contains(out, "--current") {
		t.Error("missing --current flag in help")
	}
	if !strings.Contains(out, "--to-current") {
		t.Error("missing --to-current flag in help")
	}
	if !strings.Contains(out, "--out") {
		t.Error("missing --out flag in help")
	}
}
