package reviews

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"
	"sync"
)

// ghInlineComment represents a single inline review comment from the GitHub API.
type ghInlineComment struct {
	ID        int    `json:"id"`
	User      ghUser `json:"user"`
	Body      string `json:"body"`
	Path      string `json:"path"`
	Line      *int   `json:"line"`
	OrigLine  *int   `json:"original_line"`
	DiffHunk  string `json:"diff_hunk"`
	CreatedAt string `json:"created_at"`
	InReplyTo *int   `json:"in_reply_to_id"`
}

// ghReview represents a formal review submission from the GitHub API.
type ghReview struct {
	ID          int    `json:"id"`
	User        ghUser `json:"user"`
	Body        string `json:"body"`
	State       string `json:"state"`
	SubmittedAt string `json:"submitted_at"`
}

// ghIssueComment represents a general PR/issue comment from the GitHub API.
type ghIssueComment struct {
	ID        int    `json:"id"`
	User      ghUser `json:"user"`
	Body      string `json:"body"`
	CreatedAt string `json:"created_at"`
}

type ghUser struct {
	Login string `json:"login"`
}

// FetchInlineComments fetches inline code review comments for a PR.
func FetchInlineComments(owner, repo string, prNumber int) ([]FeedbackItem, error) {
	endpoint := fmt.Sprintf("repos/%s/%s/pulls/%d/comments", owner, repo, prNumber)
	data, err := ghAPI(endpoint)
	if err != nil {
		return nil, fmt.Errorf("fetch inline comments for PR #%d: %w", prNumber, err)
	}

	var comments []ghInlineComment
	if err := json.Unmarshal(data, &comments); err != nil {
		return nil, fmt.Errorf("parse inline comments for PR #%d: %w", prNumber, err)
	}

	var items []FeedbackItem
	for _, c := range comments {
		line := 0
		if c.Line != nil {
			line = *c.Line
		} else if c.OrigLine != nil {
			line = *c.OrigLine
		}
		replyTo := 0
		if c.InReplyTo != nil {
			replyTo = *c.InReplyTo
		}
		items = append(items, FeedbackItem{
			PR:        prNumber,
			Author:    c.User.Login,
			Type:      "inline",
			File:      c.Path,
			Line:      line,
			Body:      c.Body,
			DiffHunk:  c.DiffHunk,
			CreatedAt: c.CreatedAt,
			InReplyTo: replyTo,
			ID:        c.ID,
		})
	}
	return items, nil
}

// FetchReviews fetches formal review submissions for a PR.
func FetchReviews(owner, repo string, prNumber int) ([]FeedbackItem, error) {
	endpoint := fmt.Sprintf("repos/%s/%s/pulls/%d/reviews", owner, repo, prNumber)
	data, err := ghAPI(endpoint)
	if err != nil {
		return nil, fmt.Errorf("fetch reviews for PR #%d: %w", prNumber, err)
	}

	var reviews []ghReview
	if err := json.Unmarshal(data, &reviews); err != nil {
		return nil, fmt.Errorf("parse reviews for PR #%d: %w", prNumber, err)
	}

	var items []FeedbackItem
	for _, r := range reviews {
		body := strings.TrimSpace(r.Body)
		if body == "" {
			continue
		}
		items = append(items, FeedbackItem{
			PR:        prNumber,
			Author:    r.User.Login,
			Type:      "review",
			Body:      body,
			CreatedAt: r.SubmittedAt,
			ID:        r.ID,
		})
	}
	return items, nil
}

// FetchIssueComments fetches general conversation comments for a PR.
func FetchIssueComments(owner, repo string, prNumber int) ([]FeedbackItem, error) {
	endpoint := fmt.Sprintf("repos/%s/%s/issues/%d/comments", owner, repo, prNumber)
	data, err := ghAPI(endpoint)
	if err != nil {
		return nil, fmt.Errorf("fetch issue comments for PR #%d: %w", prNumber, err)
	}

	var comments []ghIssueComment
	if err := json.Unmarshal(data, &comments); err != nil {
		return nil, fmt.Errorf("parse issue comments for PR #%d: %w", prNumber, err)
	}

	var items []FeedbackItem
	for _, c := range comments {
		body := strings.TrimSpace(c.Body)
		if body == "" {
			continue
		}
		items = append(items, FeedbackItem{
			PR:        prNumber,
			Author:    c.User.Login,
			Type:      "general",
			Body:      body,
			CreatedAt: c.CreatedAt,
			ID:        c.ID,
		})
	}
	return items, nil
}

// FetchPRReviews fetches all three types of comments for a single PR concurrently,
// merges them, and applies bot filtering.
func FetchPRReviews(owner, repo string, prNumber int) ([]FeedbackItem, error) {
	type result struct {
		items []FeedbackItem
		err   error
	}

	ch := make(chan result, 3)

	go func() {
		items, err := FetchInlineComments(owner, repo, prNumber)
		ch <- result{items, err}
	}()
	go func() {
		items, err := FetchReviews(owner, repo, prNumber)
		ch <- result{items, err}
	}()
	go func() {
		items, err := FetchIssueComments(owner, repo, prNumber)
		ch <- result{items, err}
	}()

	var allItems []FeedbackItem
	for range 3 {
		r := <-ch
		if r.err != nil {
			return nil, r.err
		}
		allItems = append(allItems, r.items...)
	}

	return FilterBots(allItems), nil
}

// FetchAll fetches reviews for multiple PRs with a concurrency limit.
func FetchAll(owner, repo string, prs map[string]int, concurrency int) ([]FeedbackItem, error) {
	type result struct {
		items []FeedbackItem
		err   error
	}

	sem := make(chan struct{}, concurrency)
	results := make(chan result, len(prs))
	var wg sync.WaitGroup

	for _, prNum := range prs {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			items, err := FetchPRReviews(owner, repo, n)
			results <- result{items, err}
		}(prNum)
	}

	go func() {
		wg.Wait()
		close(results)
	}()

	var allItems []FeedbackItem
	for r := range results {
		if r.err != nil {
			return nil, r.err
		}
		allItems = append(allItems, r.items...)
	}

	return allItems, nil
}

// ghAPI calls gh api with pagination and returns the raw JSON output.
func ghAPI(endpoint string) ([]byte, error) {
	cmd := exec.Command("gh", "api", endpoint, "--paginate")
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("gh api %s: %w", endpoint, err)
	}
	return out, nil
}