package reviews

import (
	"fmt"
	"sort"
	"strings"

	"github.com/cpave3/staccato/pkg/graph"
)

// Scope determines which branches to collect reviews from.
type Scope int

const (
	ScopeAll       Scope = iota // All branches in the stack
	ScopeCurrent                // Current branch only
	ScopeToCurrent              // Ancestors up to and including current branch
)

const botSuffix = "[bot]"

var reviewBots = map[string]bool{
	"coderabbitai[bot]":   true,
	"greptile-apps[bot]":  true,
}

// FeedbackItem represents a single review comment from a PR.
type FeedbackItem struct {
	PR         int
	Author     string
	AuthorType string // "Human" or "Bot"
	Type       string // "inline", "review", or "general"
	File       string
	Line       int
	Body       string
	DiffHunk   string
	CreatedAt  string
	InReplyTo  int
	ID         int
	Replies    []FeedbackItem
}

// ReviewResult holds the collected feedback and metadata.
type ReviewResult struct {
	Items     []FeedbackItem
	Scope     Scope
	RepoOwner string
	RepoName  string
}

// FilterBots removes bot comments except for known review bots,
// and sets AuthorType on remaining items.
func FilterBots(items []FeedbackItem) []FeedbackItem {
	var result []FeedbackItem
	for _, item := range items {
		isBot := strings.HasSuffix(item.Author, botSuffix)
		if isBot && !reviewBots[item.Author] {
			continue
		}
		if isBot {
			item.AuthorType = "Bot"
		} else {
			item.AuthorType = "Human"
		}
		result = append(result, item)
	}
	return result
}

// ThreadReplies groups inline comments into threads based on InReplyTo.
// Returns only root items with replies attached.
func ThreadReplies(items []FeedbackItem) []FeedbackItem {
	byID := make(map[int]*FeedbackItem)
	var roots []FeedbackItem

	// First pass: separate roots from replies
	for _, item := range items {
		if item.InReplyTo == 0 {
			roots = append(roots, item)
			byID[item.ID] = &roots[len(roots)-1]
		}
	}

	// Second pass: attach replies to parents
	for _, item := range items {
		if item.InReplyTo != 0 {
			if parent, ok := byID[item.InReplyTo]; ok {
				parent.Replies = append(parent.Replies, item)
			} else {
				// Orphan reply — treat as standalone
				roots = append(roots, item)
			}
		}
	}

	return roots
}

// ParseRemoteURL extracts owner and repo from a GitHub remote URL.
// Supports HTTPS (https://github.com/owner/repo.git) and SSH (git@github.com:owner/repo.git).
func ParseRemoteURL(url string) (owner, repo string, err error) {
	// SSH format: git@github.com:owner/repo.git
	if strings.HasPrefix(url, "git@") {
		idx := strings.Index(url, ":")
		if idx < 0 {
			return "", "", fmt.Errorf("invalid SSH remote URL: %s", url)
		}
		path := url[idx+1:]
		path = strings.TrimSuffix(path, ".git")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid SSH remote URL: %s", url)
		}
		return parts[0], parts[1], nil
	}

	// HTTPS format: https://github.com/owner/repo.git
	if strings.Contains(url, "github.com") {
		idx := strings.Index(url, "github.com/")
		if idx < 0 {
			return "", "", fmt.Errorf("invalid HTTPS remote URL: %s", url)
		}
		path := url[idx+len("github.com/"):]
		path = strings.TrimSuffix(path, ".git")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) != 2 {
			return "", "", fmt.Errorf("invalid HTTPS remote URL: %s", url)
		}
		return parts[0], parts[1], nil
	}

	return "", "", fmt.Errorf("unsupported remote URL format: %s", url)
}

// ResolveBranches returns the branch names to collect reviews from based on scope.
// Root/trunk branches are excluded since they don't have PRs in a stack context.
func ResolveBranches(g *graph.Graph, currentBranch string, scope Scope) []string {
	switch scope {
	case ScopeCurrent:
		return []string{currentBranch}
	case ScopeToCurrent:
		// Walk ancestors from current back to root, excluding root
		var ancestors []string
		current := currentBranch
		for current != "" && current != g.Root {
			ancestors = append([]string{current}, ancestors...)
			if b, exists := g.GetBranch(current); exists {
				current = b.Parent
			} else {
				break
			}
		}
		return ancestors
	default: // ScopeAll
		var branches []string
		for name := range g.Branches {
			branches = append(branches, name)
		}
		sort.Strings(branches)
		return branches
	}
}

// FormatMarkdown produces the unified markdown feedback document.
func FormatMarkdown(result ReviewResult) string {
	var b strings.Builder

	b.WriteString("# PR Review Feedback\n\n")
	fmt.Fprintf(&b, "**Repository:** %s/%s\n\n", result.RepoOwner, result.RepoName)

	// Group items by PR
	prItems := make(map[int][]FeedbackItem)
	var prNumbers []int
	for _, item := range result.Items {
		if _, seen := prItems[item.PR]; !seen {
			prNumbers = append(prNumbers, item.PR)
		}
		prItems[item.PR] = append(prItems[item.PR], item)
	}
	sort.Ints(prNumbers)

	for _, pr := range prNumbers {
		items := prItems[pr]
		fmt.Fprintf(&b, "## PR #%d\n\n", pr)

		for _, item := range items {
			fmt.Fprintf(&b, "### %s — %s [%s]\n\n", item.Type, item.Author, item.AuthorType)

			if item.File != "" {
				if item.Line > 0 {
					fmt.Fprintf(&b, "**File:** `%s:%d`\n\n", item.File, item.Line)
				} else {
					fmt.Fprintf(&b, "**File:** `%s`\n\n", item.File)
				}
			}

			if item.DiffHunk != "" {
				b.WriteString("**Diff context:**\n```diff\n")
				b.WriteString(item.DiffHunk)
				b.WriteString("\n```\n\n")
			}

			b.WriteString(item.Body)
			b.WriteString("\n\n")

			// Render replies
			for _, reply := range item.Replies {
				fmt.Fprintf(&b, "> **%s [%s]:** %s\n\n", reply.Author, reply.AuthorType, reply.Body)
			}
		}
	}

	// Classification prompt for AI consumers
	b.WriteString("---\n\n")
	b.WriteString("<!-- CLASSIFICATION PROMPT\n")
	b.WriteString("When processing this feedback, classify each item by severity:\n")
	b.WriteString("- CRITICAL: Runtime errors, data corruption, security issues, completely broken functionality\n")
	b.WriteString("- HIGH: Design flaws, data integrity risks, missing validation on external input\n")
	b.WriteString("- MEDIUM: Code quality, missing guards, non-deterministic behavior, missing feature wiring\n")
	b.WriteString("- LOW: Style, docs, redundant indexes, test brittleness, unused variables\n\n")
	b.WriteString("Deduplicate items that flag the same issue across PRs. Consolidate into a single item noting all sources.\n")
	b.WriteString("Prioritize Human reviewer feedback over Bot feedback.\n")
	b.WriteString("-->\n")

	return b.String()
}
