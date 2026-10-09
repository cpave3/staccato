package main

import "testing"

func TestStat(t *testing.T) {
	repo, _ := setupRepoWithStack(t)
	runSt(t, "new", "feature")

	repo.CreateFile("feature.txt", "hello\n")
	repo.AddAndCommit("add feature")

	out := runSt(t, "stat")
	assertContains(t, out, "1 file changed, 1 insertion(+)")
}

func TestStatIncludeUncommitted(t *testing.T) {
	repo, _ := setupRepoWithStack(t)
	runSt(t, "new", "feature")

	repo.CreateFile("feature.txt", "hello\n")
	repo.AddAndCommit("add feature")
	repo.WriteFile("feature.txt", "hello\nmore\n")

	defaultOut := runSt(t, "stat")
	assertContains(t, defaultOut, "1 file changed, 1 insertion(+)")

	includedOut := runSt(t, "stat", "-i")
	assertContains(t, includedOut, "1 file changed, 2 insertions(+)")
}
