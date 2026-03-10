package forge

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// GitHub implements Forge using the gh CLI.
type GitHub struct{}

func checkGH() error {
	if _, err := exec.LookPath("gh"); err != nil {
		return fmt.Errorf("gh CLI not found — install it from https://cli.github.com")
	}
	return nil
}

func (g *GitHub) CreatePR(opts PRCreateOpts) error {
	if err := checkGH(); err != nil {
		return err
	}

	args := []string{"pr", "create", "--head", opts.Head, "--base", opts.Base}
	if opts.Web {
		args = append(args, "-w")
	}

	cmd := exec.Command("gh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

func (g *GitHub) ViewPR(opts PRViewOpts) error {
	if err := checkGH(); err != nil {
		return err
	}

	args := []string{"pr", "view"}
	if opts.Web {
		args = append(args, "-w")
	}

	cmd := exec.Command("gh", args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// ghPRItem represents a single PR from the gh CLI JSON output.
type ghPRItem struct {
	Number          int    `json:"number"`
	HeadRefName     string `json:"headRefName"`
	Title           string `json:"title"`
	State           string `json:"state"`
	IsDraft         bool   `json:"isDraft"`
	ReviewDecision  string `json:"reviewDecision"`
	URL             string `json:"url"`
	StatusCheckRollup []struct {
		State      string `json:"state"`
		Status     string `json:"status"`
		Conclusion string `json:"conclusion"`
	} `json:"statusCheckRollup"`
}

func (g *GitHub) StackStatus(branches []string) (map[string]*PRStatusInfo, error) {
	if err := checkGH(); err != nil {
		return nil, err
	}

	// Query per-branch to avoid fetching all PRs (which can 504 on large repos).
	type branchResult struct {
		branch string
		prs    []ghPRItem
		err    error
	}

	ch := make(chan branchResult, len(branches))
	for _, b := range branches {
		go func(branch string) {
			cmd := exec.Command("gh", "pr", "list", "--head", branch, "--state", "all", "--limit", "1",
				"--json", "number,headRefName,title,state,isDraft,reviewDecision,url,statusCheckRollup")
			var stderr bytes.Buffer
			cmd.Stderr = &stderr
			out, err := cmd.Output()
			if err != nil {
				msg := strings.TrimSpace(stderr.String())
				if msg != "" {
					ch <- branchResult{branch: branch, err: fmt.Errorf("failed to list PRs for %s: %s", branch, msg)}
				} else {
					ch <- branchResult{branch: branch, err: fmt.Errorf("failed to list PRs for %s: %w", branch, err)}
				}
				return
			}
			var items []ghPRItem
			if err := json.Unmarshal(out, &items); err != nil {
				ch <- branchResult{branch: branch, err: fmt.Errorf("failed to parse PR list for %s: %w", branch, err)}
				return
			}
			ch <- branchResult{branch: branch, prs: items}
		}(b)
	}

	// Collect results
	result := make(map[string]*PRStatusInfo, len(branches))
	for range branches {
		res := <-ch
		if res.err != nil {
			return nil, res.err
		}
		if len(res.prs) == 0 {
			result[res.branch] = &PRStatusInfo{Branch: res.branch}
			continue
		}
		// With --limit 1 and --state all, gh returns the most relevant PR (open preferred).
		// If multiple exist, pick best by state priority.
		statePriority := map[string]int{"OPEN": 3, "MERGED": 2, "CLOSED": 1}
		best := &res.prs[0]
		for i := 1; i < len(res.prs); i++ {
			if statePriority[res.prs[i].State] > statePriority[best.State] {
				best = &res.prs[i]
			}
		}
		result[res.branch] = &PRStatusInfo{
			Branch:       res.branch,
			HasPR:        true,
			Number:       best.Number,
			Title:        best.Title,
			State:        best.State,
			IsDraft:      best.IsDraft,
			ReviewStatus: best.ReviewDecision,
			URL:          best.URL,
			CheckStatus:  deriveCheckStatus(best.StatusCheckRollup),
		}
	}

	return result, nil
}

func deriveCheckStatus(checks []struct {
	State      string `json:"state"`
	Status     string `json:"status"`
	Conclusion string `json:"conclusion"`
}) string {
	if len(checks) == 0 {
		return ""
	}
	allPass := true
	for _, c := range checks {
		conclusion := c.Conclusion
		if conclusion == "" {
			conclusion = c.State
		}
		switch conclusion {
		case "SUCCESS", "NEUTRAL", "SKIPPED":
			// still passing
		case "FAILURE", "ERROR", "CANCELLED", "TIMED_OUT", "ACTION_REQUIRED":
			return "fail"
		default:
			allPass = false
		}
	}
	if allPass {
		return "pass"
	}
	return "pending"
}
