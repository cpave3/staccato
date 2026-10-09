package split

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
)

// Plan says how to cut a branch into stacked slices, listed from the bottom of
// the stack to the top.
type Plan struct {
	// Branch is the branch to split. Empty means the current branch.
	Branch string `json:"branch,omitempty"`
	// Head, when set, is the commit the branch was at when the plan was
	// written (the "head" of 'st split describe'). The split is refused if the
	// branch has moved since.
	Head string `json:"head,omitempty"`
	// DropOriginal has to be set when no slice keeps the name of the branch
	// being split, which deletes that branch. Without it such a plan is
	// refused, so a branch cannot disappear because a name was forgotten.
	DropOriginal bool    `json:"drop_original,omitempty"`
	Slices       []Slice `json:"slices"`
}

// Slice is one branch of the resulting stack. Its content comes from exactly
// one source: Commits, Paths and/or Overrides, or Rest.
type Slice struct {
	// Name is the branch name. One slice may reuse the name of the branch
	// being split.
	Name string `json:"name"`
	// Message is the commit message for a slice built from Paths, Overrides
	// or Rest.
	Message string `json:"message,omitempty"`
	// Commits are existing commits of the branch, kept as they are (message
	// and author included) and applied in the order given.
	Commits []string `json:"commits,omitempty"`
	// Paths are pathspecs, relative to the repository root. Every changed file
	// they match is brought to its final content in this slice.
	Paths []string `json:"paths,omitempty"`
	// Overrides give a changed file content that is only partway to its final
	// state, for a file whose changes belong to more than one slice. Each key
	// is a repository path and each value a file holding the content that path
	// has in this slice. A later slice must list the path to finish it.
	Overrides map[string]string `json:"overrides,omitempty"`
	// Rest takes everything the earlier slices left out. Last slice only.
	Rest bool `json:"rest,omitempty"`
}

// Validate checks the shape of the plan: everything that can be judged without
// looking at the repository.
func (p *Plan) Validate() error {
	if len(p.Slices) < 2 {
		return fmt.Errorf("a split needs at least two slices, got %d", len(p.Slices))
	}
	seen := map[string]bool{}
	for i, s := range p.Slices {
		if s.Name == "" {
			return fmt.Errorf("slice %d has no name", i+1)
		}
		if seen[s.Name] {
			return fmt.Errorf("branch name %q is used by more than one slice", s.Name)
		}
		seen[s.Name] = true

		byCommits := len(s.Commits) > 0
		byPaths := len(s.Paths) > 0 || len(s.Overrides) > 0
		sources := 0
		for _, used := range []bool{byCommits, byPaths, s.Rest} {
			if used {
				sources++
			}
		}
		switch {
		case sources == 0:
			return fmt.Errorf(`slice %q has no content: give it "commits", "paths" (with optional "overrides"), or "rest": true`, s.Name)
		case sources > 1:
			return fmt.Errorf(`slice %q mixes content sources: use only one of "commits", "paths" (with optional "overrides"), or "rest"`, s.Name)
		case byCommits && s.Message != "":
			return fmt.Errorf(`slice %q has a message, but "commits" keeps each commit's own message — remove it`, s.Name)
		case !byCommits && s.Message == "":
			return fmt.Errorf("slice %q needs a message for the commit it creates", s.Name)
		case s.Rest && i != len(p.Slices)-1:
			return fmt.Errorf(`only the last slice can use "rest", but slice %q is not last`, s.Name)
		}
		for _, path := range s.Paths {
			if path == "" {
				return fmt.Errorf("slice %q has an empty path", s.Name)
			}
		}
	}
	return nil
}

// ParsePlan decodes a JSON plan. Unknown fields are rejected so a misspelt key
// is reported instead of silently ignored.
func ParsePlan(data []byte) (*Plan, error) {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	var plan Plan
	if err := dec.Decode(&plan); err != nil {
		return nil, planError(data, err)
	}
	if _, err := dec.Token(); err != io.EOF {
		return nil, fmt.Errorf("invalid plan: unexpected content after the plan — it must be a single JSON object")
	}
	return &plan, nil
}

// planError points a JSON error at the line it occurred on.
func planError(data []byte, err error) error {
	var offset int64
	var syntaxErr *json.SyntaxError
	var typeErr *json.UnmarshalTypeError
	switch {
	case errors.As(err, &syntaxErr):
		offset = syntaxErr.Offset
	case errors.As(err, &typeErr):
		offset = typeErr.Offset
	default:
		return fmt.Errorf("invalid plan: %w", err)
	}
	if offset > int64(len(data)) {
		offset = int64(len(data))
	}
	read := data[:offset]
	line := 1 + bytes.Count(read, []byte("\n"))
	column := len(read) - bytes.LastIndexByte(read, '\n') - 1
	hint := ""
	if syntaxErr != nil && offset > 0 && data[offset-1] == '/' {
		hint = " — comments are not allowed in JSON"
	}
	return fmt.Errorf("invalid plan (line %d, column %d): %w%s", line, column, err, hint)
}
