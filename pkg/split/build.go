package split

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/cpave3/staccato/pkg/git"
)

// builtSlice is a slice whose commits exist as objects but which no ref points
// at yet.
type builtSlice struct {
	name string
	sha  string
}

// builder creates the commits of each slice with git plumbing, in a scratch
// index, so the working tree and the real index are never touched.
type builder struct {
	git     *git.Runner
	base    string
	head    string
	changes map[string]change // base → head, by path
	commits map[string]commit // base..head, by sha
	history []commit          // base..head, oldest first
	placed  map[string]string // commit sha → slice that took it
	env     []string          // points git at the scratch index
	baseDir string            // relative override sources resolve against this
	// placedBy records which paths slice brought a file to its final content,
	// and pending which slice left a file partway there with an override.
	placedBy map[string]string
	pending  map[string]string
	// warnings are things the plan's author should know about a split that
	// is otherwise fine.
	warnings []string
	// sign is set when the repository signs commits. commit-tree does not
	// read commit.gpgsign itself.
	sign bool
}

// newBuilder prepares to build slices of t. The returned func removes the
// scratch index.
func newBuilder(g *git.Runner, t *target, baseDir string) (*builder, func(), error) {
	changes, err := diffChanges(g, t.base, t.head)
	if err != nil {
		return nil, nil, err
	}
	commits, err := listCommits(g, t.base, t.head)
	if err != nil {
		return nil, nil, err
	}
	dir, err := os.MkdirTemp("", "st-split-*")
	if err != nil {
		return nil, nil, fmt.Errorf("create scratch index: %w", err)
	}
	b := &builder{
		git:     g,
		base:    t.base,
		head:    t.head,
		changes: make(map[string]change, len(changes)),
		commits: make(map[string]commit, len(commits)),
		history: commits,
		placed:  map[string]string{},
		env:     []string{"GIT_INDEX_FILE=" + filepath.Join(dir, "index")},
		baseDir: baseDir,

		placedBy: map[string]string{},
		pending:  map[string]string{},
	}
	for _, ch := range changes {
		b.changes[ch.path] = ch
	}
	for _, c := range commits {
		b.commits[c.sha] = c
	}
	if sign, err := g.Run("config", "--type=bool", "--get", "commit.gpgsign"); err == nil {
		b.sign = sign == "true"
	}
	return b, func() { os.RemoveAll(dir) }, nil
}

// build creates every slice on top of the previous one, starting at the
// branch's base.
func (b *builder) build(slices []Slice) ([]builtSlice, error) {
	built := make([]builtSlice, 0, len(slices))
	prev := b.base
	for _, s := range slices {
		var sha string
		var err error
		switch {
		case len(s.Commits) > 0:
			sha, err = b.commitsSlice(prev, s)
		case s.Rest:
			sha, err = b.restSlice(prev, s)
		default:
			sha, err = b.pathsSlice(prev, s)
		}
		if err != nil {
			return nil, err
		}
		empty, err := b.sameTree(prev, sha)
		if err != nil {
			return nil, err
		}
		if empty {
			if s.Rest {
				return nil, fmt.Errorf("slice %q is empty: the earlier slices already contain the whole branch", s.Name)
			}
			return nil, fmt.Errorf("slice %q is empty: everything it lists is already in an earlier slice", s.Name)
		}
		built = append(built, builtSlice{name: s.Name, sha: sha})
		prev = sha
	}
	if err := b.checkComplete(prev); err != nil {
		return nil, err
	}
	return built, nil
}

// sameTree reports whether two commits hold identical content.
func (b *builder) sameTree(x, y string) (bool, error) {
	out, err := b.git.RunWith(nil, "", "rev-parse", x+"^{tree}", y+"^{tree}")
	if err != nil {
		return false, fmt.Errorf("compare trees: %w", err)
	}
	trees := strings.Fields(out)
	return len(trees) == 2 && trees[0] == trees[1], nil
}

// checkComplete enforces the invariant every split rests on: the top slice
// holds exactly the content the branch had before the split.
func (b *builder) checkComplete(top string) error {
	out, err := b.git.Output(nil, "", "diff-tree", "-r", "--no-renames", "--name-only", "-z", top, b.head)
	if err != nil {
		return fmt.Errorf("compare the top slice with the branch: %w", err)
	}
	var missing, unfinished []string
	for _, path := range splitNUL(out) {
		if slice, ok := b.pending[path]; ok {
			unfinished = append(unfinished, fmt.Sprintf("slice %q overrides %q but no later slice finishes it — add %q to a later slice's paths", slice, path, path))
		} else {
			missing = append(missing, path)
		}
	}
	if len(missing) == 0 && len(unfinished) == 0 {
		return nil
	}

	var msg []string
	if len(missing) > 0 {
		msg = append(msg, fmt.Sprintf("the slices do not add up to the branch: %d file(s) are missing or differ from its final content:\n  %s",
			len(missing), strings.Join(missing, "\n  ")))
	}
	msg = append(msg, unfinished...)
	if len(b.placed) > 0 {
		var unplaced []string
		for _, c := range b.history {
			if _, ok := b.placed[c.sha]; !ok {
				unplaced = append(unplaced, abbrev(c.sha)+" "+c.subject)
			}
		}
		if len(unplaced) > 0 {
			msg = append(msg, "commits not assigned to a slice:\n  "+strings.Join(unplaced, "\n  "))
		}
	}
	return fmt.Errorf("%s\nassign them to a slice, or end the plan with a {\"rest\": true} slice", strings.Join(msg, "\n"))
}

// commitsSlice stacks existing commits of the branch on top of prev. A commit
// whose parent is already in place is reused as it is, so cutting a branch at
// its existing commit boundaries rewrites nothing. Any other commit is
// cherry-picked without a working tree.
func (b *builder) commitsSlice(prev string, s Slice) (string, error) {
	cur := prev
	for _, rev := range s.Commits {
		sha, err := b.git.RunWith(nil, "", "rev-parse", "--verify", "--quiet", "--end-of-options", rev+"^{commit}")
		if err != nil {
			return "", fmt.Errorf("slice %q: commit %q not found", s.Name, rev)
		}
		c, ok := b.commits[sha]
		if !ok {
			return "", fmt.Errorf("slice %q: commit %q is not one of this branch's commits — run 'st split describe' to list them", s.Name, rev)
		}
		if other, dup := b.placed[sha]; dup {
			return "", fmt.Errorf("commit %q is listed twice (slices %q and %q)", rev, other, s.Name)
		}
		b.placed[sha] = s.Name
		if len(c.parents) != 1 {
			return "", fmt.Errorf("slice %q: commit %q is a merge commit and cannot be placed on its own — assign its files with paths instead", s.Name, rev)
		}

		if c.parents[0] == cur {
			cur = sha
			continue
		}
		pick, err := b.git.CherryPickTree(cur, sha)
		if err != nil {
			return "", fmt.Errorf("slice %q: apply commit %q: %w", s.Name, rev, err)
		}
		if pick.Conflicted {
			return "", fmt.Errorf("slice %q: commit %q (%s) does not apply at this position — it conflicts in:\n  %s\nplace the commits it builds on in the same or an earlier slice, or assign these files with paths instead",
				s.Name, rev, c.subject, strings.Join(pick.Paths, "\n  "))
		}
		// Everything this commit does is already in place, as happens when an
		// earlier slice took its files by path.
		if curTree, err := b.git.GetCommitSHA(cur + "^{tree}"); err != nil {
			return "", fmt.Errorf("slice %q: %w", s.Name, err)
		} else if curTree == pick.Tree {
			b.warnings = append(b.warnings, fmt.Sprintf("commit %s (%s) adds nothing on top of the earlier slices and was left out of slice %q", abbrev(sha), c.subject, s.Name))
			continue
		}
		if cur, err = b.copyCommit(sha, pick.Tree, cur); err != nil {
			return "", fmt.Errorf("slice %q: %w", s.Name, err)
		}
	}
	return cur, nil
}

// copyCommit creates a commit of tree on the given parents that keeps the
// message and author of an existing commit.
func (b *builder) copyCommit(from, tree string, parents ...string) (string, error) {
	out, err := b.git.RunWith(nil, "", "log", "-1", "--format=%an%x00%ae%x00%aI%x00%B", from)
	if err != nil {
		return "", fmt.Errorf("read commit %s: %w", from, err)
	}
	parts := strings.SplitN(out, "\x00", 4)
	if len(parts) != 4 {
		return "", fmt.Errorf("read commit %s: unexpected output", from)
	}
	author := []string{
		"GIT_AUTHOR_NAME=" + parts[0],
		"GIT_AUTHOR_EMAIL=" + parts[1],
		"GIT_AUTHOR_DATE=" + parts[2],
	}
	return b.commitTree(tree, parts[3], author, parents...)
}

// pathsSlice commits the final content of every changed file matched by the
// slice's pathspecs, then applies the slice's overrides on top.
func (b *builder) pathsSlice(prev string, s Slice) (string, error) {
	if _, err := b.git.RunWith(b.env, "", "-c", "core.sparseCheckout=false", "read-tree", prev); err != nil {
		return "", fmt.Errorf("read tree of %s: %w", prev, err)
	}

	// Files that are not yet at their final content below this slice.
	out, err := b.git.Output(nil, "", "diff-tree", "-r", "--no-renames", "--name-only", "-z", prev, b.head)
	if err != nil {
		return "", fmt.Errorf("slice %q: compare with the branch: %w", s.Name, err)
	}
	remaining := map[string]bool{}
	for _, path := range splitNUL(out) {
		remaining[path] = true
	}

	var entries strings.Builder
	for _, spec := range s.Paths {
		out, err := b.git.Output(nil, "", "diff-tree", "-r", "--no-renames", "--name-only", "-z", b.base, b.head, "--", spec)
		if err != nil {
			return "", fmt.Errorf("slice %q: resolve path %q: %w", s.Name, spec, err)
		}
		matched := splitNUL(out)
		if len(matched) == 0 {
			return "", fmt.Errorf("slice %q: path %q matches no file changed on this branch — run 'st split describe' to list them", s.Name, spec)
		}
		// A path only claims files that are still open. Claiming nothing means
		// the plan's author believes a file is somewhere it is not.
		claimed := 0
		for _, path := range matched {
			if !remaining[path] {
				continue
			}
			claimed++
			entries.WriteString(b.finalEntry(path))
			b.placedBy[path] = s.Name
			delete(b.pending, path)
		}
		if claimed == 0 {
			if earlier, ok := b.placedBy[matched[0]]; ok && earlier != s.Name {
				return "", fmt.Errorf("slice %q: path %q matches only files already placed by slice %q — narrow that slice's paths or remove this one", s.Name, spec, earlier)
			}
			return "", fmt.Errorf("slice %q: path %q matches only files that are already at their final content in an earlier slice — remove it", s.Name, spec)
		}
	}
	overridden := make([]string, 0, len(s.Overrides))
	for path := range s.Overrides {
		overridden = append(overridden, path)
	}
	sort.Strings(overridden)
	for _, path := range overridden {
		entry, err := b.overrideEntry(path, s.Overrides[path])
		if err != nil {
			return "", fmt.Errorf("slice %q: %w", s.Name, err)
		}
		entries.WriteString(entry)
		b.pending[path] = s.Name
		delete(b.placedBy, path)
	}
	if _, err := b.git.RunWith(b.env, entries.String(), "update-index", "-z", "--index-info"); err != nil {
		return "", fmt.Errorf("slice %q: stage paths: %w", s.Name, err)
	}

	tree, err := b.git.RunWith(b.env, "", "write-tree")
	if err != nil {
		return "", fmt.Errorf("slice %q: write tree: %w", s.Name, err)
	}
	return b.commitTree(tree, s.Message, nil, prev)
}

// restSlice commits everything the earlier slices left out.
func (b *builder) restSlice(prev string, s Slice) (string, error) {
	tree, err := b.git.GetCommitSHA(b.head + "^{tree}")
	if err != nil {
		return "", fmt.Errorf("resolve tree of %s: %w", b.head, err)
	}
	b.pending = map[string]string{}
	return b.commitTree(tree, s.Message, nil, prev)
}

// finalEntry is the `update-index --index-info` record that brings path to its
// state at the branch's head.
func (b *builder) finalEntry(path string) string {
	ch := b.changes[path]
	if ch.status == "deleted" {
		return "0 " + strings.Repeat("0", len(b.head)) + "\t" + path + "\x00"
	}
	return ch.mode + " " + ch.sha + "\t" + path + "\x00"
}

// commitTree creates a commit of tree on the given parents. env can carry
// GIT_AUTHOR_* variables to keep another commit's author.
func (b *builder) commitTree(tree, message string, env []string, parents ...string) (string, error) {
	if !strings.HasSuffix(message, "\n") {
		message += "\n"
	}
	args := []string{"commit-tree"}
	for _, parent := range parents {
		args = append(args, "-p", parent)
	}
	if b.sign {
		args = append(args, "-S")
	}
	args = append(args, "-F", "-", tree)
	sha, err := b.git.RunWith(env, message, args...)
	if err != nil {
		return "", fmt.Errorf("create commit: %w", err)
	}
	return sha, nil
}

// overrideEntry is the index record that gives path the content of the file at
// source instead of its final content.
func (b *builder) overrideEntry(path, source string) (string, error) {
	ch, ok := b.changes[path]
	if !ok {
		return "", fmt.Errorf("override %q is not changed on this branch — an override gives a changed file its content partway through the split", path)
	}
	if !filepath.IsAbs(source) {
		source = filepath.Join(b.baseDir, source)
	}
	if info, err := os.Stat(source); err != nil || info.IsDir() {
		return "", fmt.Errorf("override %q: cannot read content file %s", path, source)
	}
	mode := ch.mode
	if ch.status == "deleted" {
		mode = ch.srcMode
	}
	if mode != "100644" && mode != "100755" {
		return "", fmt.Errorf("override %q: only a regular file can be given intermediate content, and this path is a symlink or submodule", path)
	}
	blob, err := b.git.RunWith(nil, "", "hash-object", "-w", "--path="+path, "--", source)
	if err != nil {
		return "", fmt.Errorf("override %q: %w", path, err)
	}
	return mode + " " + blob + "\t" + path + "\x00", nil
}
