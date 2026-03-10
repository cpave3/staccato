## 1. Package Structure and Data Types

- [x] 1.1 Create `pkg/reviews/` package with data types: `FeedbackItem` (PR, Author, AuthorType, Type, File, Line, Body, DiffHunk, CreatedAt, InReplyTo, Replies), `ReviewResult` (Items, Scope, RepoOwner, RepoName)
- [x] 1.2 Add `Scope` type with constants: `ScopeAll`, `ScopeCurrent`, `ScopeToCurrent`
- [x] 1.3 Add bot filtering config: `BotSuffix = "[bot]"`, `ReviewBots = []string{"coderabbitai[bot]", "greptile-apps[bot]"}`

## 2. GitHub API Fetching

- [x] 2.1 Implement `FetchInlineComments(owner, repo string, prNumber int) ([]FeedbackItem, error)` — shells out to `gh api repos/{owner}/{repo}/pulls/{n}/comments --paginate`, parses JSON
- [x] 2.2 Implement `FetchReviews(owner, repo string, prNumber int) ([]FeedbackItem, error)` — shells out to `gh api repos/{owner}/{repo}/pulls/{n}/reviews --paginate`, parses JSON, skips empty bodies and pure approvals
- [x] 2.3 Implement `FetchIssueComments(owner, repo string, prNumber int) ([]FeedbackItem, error)` — shells out to `gh api repos/{owner}/{repo}/issues/{n}/comments --paginate`, parses JSON, skips empty bodies
- [x] 2.4 Implement `FetchPRReviews(owner, repo string, prNumber int) ([]FeedbackItem, error)` — calls all three fetch functions concurrently, merges results, applies bot filtering
- [x] 2.5 Implement `FetchAll(owner, repo string, prs map[string]int, concurrency int) ([]FeedbackItem, error)` — fetches reviews for multiple PRs with semaphore-based concurrency limit of 5

## 3. Threading and Formatting

- [x] 3.1 Implement `ThreadReplies(items []FeedbackItem) []FeedbackItem` — groups items by `in_reply_to_id`, attaches replies to parent items, returns only root items with replies populated
- [x] 3.2 Implement `FormatMarkdown(result ReviewResult) string` — produces the unified markdown document with items grouped by PR, each showing author/type/file/line/body/diff, includes classification prompt section at the end

## 4. Scope Resolution

- [x] 4.1 Implement `ResolveBranches(g *graph.Graph, currentBranch string, scope Scope) []string` — returns branch names based on scope: all stack branches, current only, or ancestors-to-current
- [x] 4.2 Add helper to extract `owner/repo` from git remote URL (parse GitHub URL patterns: HTTPS and SSH)

## 5. CLI Command

- [x] 5.1 Create `cmd/st/reviews.go` with `reviewsCmd()` returning `*cobra.Command` for `st reviews`
- [x] 5.2 Add `--current`, `--to-current`, `--out` flags
- [x] 5.3 Wire up: `getContext()` → resolve scope → detect forge → get PR numbers via `StackStatus` → fetch reviews → format → output to stdout or file
- [x] 5.4 Register `reviewsCmd()` in `main.go`

## 6. MCP Tool

- [x] 6.1 Create `pkg/mcp/tools_reviews.go` with `registerReviewTools(s, sc)` function
- [x] 6.2 Register `st_reviews` tool with `scope` (string enum: all/current/to-current) and `out` (optional string) parameters
- [x] 6.3 Implement handler: resolve scope, detect forge, fetch, format, return as tool result text (or write to file if `out` specified)
- [x] 6.4 Set tool annotation: read-only, non-destructive, idempotent, open-world=true
- [x] 6.5 Register `registerReviewTools` in `server.go`'s `NewServer` function

## 7. Tests

- [x] 7.1 Write unit tests for bot filtering logic (generic bot filtered, review bot kept, human kept)
- [x] 7.2 Write unit tests for reply threading (replies attached to parents, standalone items preserved)
- [x] 7.3 Write unit tests for scope resolution (all branches, current only, to-current)
- [x] 7.4 Write unit tests for URL parsing (HTTPS and SSH remote URL patterns)
- [x] 7.5 Write unit tests for markdown formatting (correct structure, all item types represented)
- [x] 7.6 Write E2E test for `st reviews` command using mock/local test setup (verify command runs, flags work)
