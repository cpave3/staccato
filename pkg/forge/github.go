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

	cmd := exec.Command("gh", "pr", "list", "--state", "all", "--limit", "100",
		"--json", "number,headRefName,title,state,isDraft,reviewDecision,url,statusCheckRollup")
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg != "" {
			return nil, fmt.Errorf("failed to list PRs: %s", msg)
		}
		return nil, fmt.Errorf("failed to list PRs: %w", err)
	}

	var prs []ghPRItem
	if err := json.Unmarshal(out, &prs); err != nil {
		return nil, fmt.Errorf("failed to parse PR list: %w", err)
	}

	// Build a set of requested branches for fast lookup
	branchSet := make(map[string]bool, len(branches))
	for _, b := range branches {
		branchSet[b] = true
	}

	// Match PRs to branches, preferring OPEN > MERGED > CLOSED
	statePriority := map[string]int{"OPEN": 3, "MERGED": 2, "CLOSED": 1}
	matched := make(map[string]*ghPRItem)

	for i := range prs {
		pr := &prs[i]
		if !branchSet[pr.HeadRefName] {
			continue
		}
		existing, ok := matched[pr.HeadRefName]
		if !ok || statePriority[pr.State] > statePriority[existing.State] {
			matched[pr.HeadRefName] = pr
		}
	}

	// Build result map
	result := make(map[string]*PRStatusInfo, len(branches))
	for _, b := range branches {
		pr, ok := matched[b]
		if !ok {
			result[b] = &PRStatusInfo{Branch: b}
			continue
		}
		result[b] = &PRStatusInfo{
			Branch:       b,
			HasPR:        true,
			Number:       pr.Number,
			Title:        pr.Title,
			State:        pr.State,
			IsDraft:      pr.IsDraft,
			ReviewStatus: pr.ReviewDecision,
			URL:          pr.URL,
			CheckStatus:  deriveCheckStatus(pr.StatusCheckRollup),
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
