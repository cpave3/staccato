// Package transfer moves a stack's graph from one clone to another: Export
// writes it as a self-describing document, and Plan and Apply merge such a
// document into this clone's graph, one branch at a time, trusting git over
// either graph.
package transfer

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"

	"github.com/cpave3/staccato/pkg/graph"
)

const (
	// Format is the document version this st writes and the newest it reads.
	// Raise it only for a change an older st cannot read correctly.
	Format = 1
	// Kind names what the document is.
	Kind = "staccato.graph"
)

// Document is a stack's graph in transit. Unlike graph.json, which is
// storage, it has a format number and an ordered list of branches.
type Document struct {
	Format    int      `json:"format"`
	Kind      string   `json:"kind"`
	STVersion string   `json:"st_version,omitempty"`
	Root      string   `json:"root"`
	Branches  []Branch `json:"branches"`
}

// Branch is one branch of a Document. HeadSHA is the commit the git branch
// pointed to when it was exported.
type Branch struct {
	Name    string `json:"name"`
	Parent  string `json:"parent"`
	BaseSHA string `json:"base_sha"`
	HeadSHA string `json:"head_sha"`
}

var objectName = regexp.MustCompile(`^[0-9a-f]{40}([0-9a-f]{24})?$`)

// Encode renders doc as indented JSON.
func Encode(doc *Document) ([]byte, error) {
	data, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}

// Decode reads a document and checks its shape: the kind, a format this st
// reads, and each branch's name, parent and commits. Unknown fields are
// ignored, so a later st can add some without breaking this one. Branch names
// are checked against git's rules by Plan, which has a repository to ask.
func Decode(data []byte) (*Document, error) {
	var doc Document
	if err := json.Unmarshal(data, &doc); err != nil || doc.Kind != Kind {
		return nil, fmt.Errorf("not a staccato graph document (want \"kind\": %q, from 'st graph export')", Kind)
	}
	if doc.Format > Format {
		return nil, fmt.Errorf("the document is format %d, and this st reads up to format %d: update st ('st update') and import again", doc.Format, Format)
	}
	if doc.Format < 1 {
		return nil, fmt.Errorf("the document has no format number")
	}
	if doc.Root == "" {
		return nil, fmt.Errorf("the document names no root branch")
	}
	seen := map[string]bool{}
	for _, b := range doc.Branches {
		switch {
		case b.Name == "":
			return nil, fmt.Errorf("the document has a branch with no name")
		case b.Name == doc.Root:
			return nil, fmt.Errorf("the document lists %s as a branch, but %s is the root", b.Name, b.Name)
		case seen[b.Name]:
			return nil, fmt.Errorf("branch %s appears twice in the document", b.Name)
		case b.Parent == "":
			return nil, fmt.Errorf("branch %s has no parent in the document", b.Name)
		case !objectName.MatchString(b.BaseSHA):
			return nil, fmt.Errorf("branch %s: base_sha %q is not a full commit name", b.Name, b.BaseSHA)
		case !objectName.MatchString(b.HeadSHA):
			return nil, fmt.Errorf("branch %s: head_sha %q is not a full commit name", b.Name, b.HeadSHA)
		}
		seen[b.Name] = true
	}
	return &doc, nil
}

// Order puts names in document order: depth first from g's root, each parent
// before its children and siblings by name, so that every stack is
// contiguous. Names whose parent chain does not reach the root follow, in the
// same order from the first of their ancestors that is not among names.
func Order(g *graph.Graph, names []string) []string {
	wanted := map[string]bool{}
	for _, n := range names {
		wanted[n] = true
	}
	children := map[string][]string{}
	for _, n := range names {
		b, ok := g.GetBranch(n)
		if !ok {
			continue
		}
		children[b.Parent] = append(children[b.Parent], n)
	}
	for _, c := range children {
		sort.Strings(c)
	}

	var out []string
	visited := map[string]bool{}
	var walk func(parent string)
	walk = func(parent string) {
		for _, c := range children[parent] {
			if visited[c] {
				continue
			}
			visited[c] = true
			out = append(out, c)
			walk(c)
		}
	}
	walk(g.Root)

	// Branches cut off from the root: start from each one whose parent is
	// not among names.
	rest := []string{}
	for _, n := range names {
		if !visited[n] {
			rest = append(rest, n)
		}
	}
	sort.Strings(rest)
	for _, n := range rest {
		if visited[n] {
			continue
		}
		if b, ok := g.GetBranch(n); ok && wanted[b.Parent] && !visited[b.Parent] {
			continue // reached from its parent
		}
		visited[n] = true
		out = append(out, n)
		walk(n)
	}
	// Anything left is in a cycle among names; keep it rather than drop it.
	for _, n := range rest {
		if !visited[n] {
			visited[n] = true
			out = append(out, n)
		}
	}
	return out
}
