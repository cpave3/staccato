package forge

import (
	"fmt"
	"strings"

	"github.com/user/st/pkg/git"
)

// PRCreateOpts holds options for creating a pull request.
type PRCreateOpts struct {
	Head string
	Base string
	Web  bool
}

// PRViewOpts holds options for viewing a pull request.
type PRViewOpts struct {
	Web bool
}

// PRStatusInfo holds the status of a PR for a branch.
type PRStatusInfo struct {
	Branch       string
	HasPR        bool
	Number       int
	Title        string
	State        string // OPEN, MERGED, CLOSED
	IsDraft      bool
	ReviewStatus string // APPROVED, CHANGES_REQUESTED, REVIEW_REQUIRED, ""
	URL          string
	CheckStatus  string // pass, fail, pending, ""
}

// Forge abstracts PR operations across hosting providers.
type Forge interface {
	CreatePR(opts PRCreateOpts) error
	ViewPR(opts PRViewOpts) error
	StackStatus(branches []string) (map[string]*PRStatusInfo, error)
}

// Detect inspects the origin remote URL and returns the appropriate Forge.
func Detect(gitRunner *git.Runner) (Forge, error) {
	url, err := gitRunner.GetRemoteURL("origin")
	if err != nil {
		return nil, fmt.Errorf("failed to get remote URL: %w", err)
	}

	if strings.Contains(url, "github.com") {
		return &GitHub{}, nil
	}

	return nil, fmt.Errorf("forge not supported for remote %q — only GitHub is implemented", url)
}
