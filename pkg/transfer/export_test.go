package transfer

import (
	"strings"
	"testing"
)

func TestExportTheCurrentStack(t *testing.T) {
	_, c := newOrigin(t)
	c.stack("a=main", "b=a", "z=main")
	c.push("a", "b", "z")
	c.git("checkout", "-q", "b")

	doc, err := Export(c.context(), ExportOptions{STVersion: "v0.5.0"})
	if err != nil {
		t.Fatal(err)
	}

	if names(doc) != "a,b" || doc.Root != "main" || doc.Format != Format || doc.Kind != Kind || doc.STVersion != "v0.5.0" {
		t.Errorf("document = %+v", doc)
	}
	a := c.graph().Branches["a"]
	if doc.Branches[0] != (Branch{Name: "a", Parent: "main", BaseSHA: a.BaseSHA, HeadSHA: c.sha("a")}) {
		t.Errorf("a = %+v", doc.Branches[0])
	}
}

func TestExportAnotherStackOrAll(t *testing.T) {
	_, c := newOrigin(t)
	c.stack("a=main", "b=a", "z=main")
	c.push("a", "b", "z")

	one, err := Export(c.context(), ExportOptions{Branch: "z"})
	if err != nil || names(one) != "z" {
		t.Errorf("--branch z = %v, %v", one, err)
	}
	all, err := Export(c.context(), ExportOptions{All: true})
	if err != nil || names(all) != "a,b,z" {
		t.Errorf("--all = %v, %v", all, err)
	}
}

func TestExportTakesTheHeadFromGit(t *testing.T) {
	_, c := newOrigin(t)
	c.stack("a=main")
	c.git("checkout", "-q", "a")
	tip := c.commit("more.txt", "more") // the graph still has the old head
	c.push("a")

	doc, err := Export(c.context(), ExportOptions{})
	if err != nil {
		t.Fatal(err)
	}

	if doc.Branches[0].HeadSHA != tip {
		t.Errorf("head = %s, want git's tip %s", doc.Branches[0].HeadSHA, tip)
	}
}

func TestExportRefusesBranchesTheRemoteDoesNotMatch(t *testing.T) {
	_, c := newOrigin(t)
	c.stack("a=main", "b=a", "c=b", "d=c")
	c.push("a", "b", "d")
	c.git("checkout", "-q", "a")
	c.commit("unpushed.txt", "x")     // a: ahead of origin/a
	c.git("branch", "-f", "d", "d~1") // d: behind origin/d
	// c: never pushed

	_, err := Export(c.context(), ExportOptions{})

	if err == nil {
		t.Fatal("exported branches the remote does not have")
	}
	for _, want := range []string{"a has commits origin/a lacks", "c is not on origin", "d is behind origin/d", "--allow-unpushed"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err = %v, want it to say %q", err, want)
		}
	}
	if strings.Contains(err.Error(), "b ") {
		t.Errorf("err names b, which is pushed: %v", err)
	}

	doc, err := Export(c.context(), ExportOptions{AllowUnpushed: true})
	if err != nil || doc.Branches[0].HeadSHA != c.sha("a") {
		t.Errorf("--allow-unpushed = %+v, %v", doc, err)
	}
}

func TestExportChecksAnotherRemote(t *testing.T) {
	origin, c := newOrigin(t)
	c.stack("a=main")
	c.git("remote", "add", "upstream", origin)

	_, err := Export(c.context(), ExportOptions{Branch: "a", Remote: "upstream"})

	if err == nil || !strings.Contains(err.Error(), "not on upstream") {
		t.Errorf("err = %v, want a not on upstream", err)
	}
}

func TestExportNeedsABranchOfAStack(t *testing.T) {
	_, c := newOrigin(t)
	c.stack("a=main")
	c.git("checkout", "-q", "-b", "loose")

	for _, branch := range []string{"main", "loose"} {
		c.git("checkout", "-q", branch)
		_, err := Export(c.context(), ExportOptions{})
		if err == nil || !strings.Contains(err.Error(), branch) || !strings.Contains(err.Error(), "--branch") {
			t.Errorf("on %s: err = %v, want it to name %s and suggest --branch", branch, err, branch)
		}
	}
}

func TestExportRefusesAGraphEntryWithNoBranch(t *testing.T) {
	_, c := newOrigin(t)
	c.stack("a=main", "b=a")
	c.push("a", "b")
	c.git("branch", "-D", "b")

	_, err := Export(c.context(), ExportOptions{Branch: "a"})

	if err == nil || !strings.Contains(err.Error(), "b") || !strings.Contains(err.Error(), "st detach b") {
		t.Errorf("err = %v, want it to name b and st detach b", err)
	}
}
