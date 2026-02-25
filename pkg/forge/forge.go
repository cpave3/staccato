package forge

import (
	"fmt"
	"strings"

	"github.com/user/st/pkg/git"
)

// PRCreateOpts holds options for creating a pull request.
type PRCreateOpts struct {
	Head string // current branch
	Base string // parent branch in stack
	Web  bool   // open in browser with -w
}

// PRViewOpts holds options for viewing a pull request.
type PRViewOpts struct {
	Web bool // open in browser with -w
}

// Forge abstracts PR operations across different hosting providers.
type Forge interface {
	CreatePR(opts PRCreateOpts) error
	ViewPR(opts PRViewOpts) error
}

// DetectForge inspects the origin remote URL and returns the appropriate Forge.
func DetectForge(gitRunner *git.Runner) (Forge, error) {
	url, err := gitRunner.GetRemoteURL("origin")
	if err != nil {
		return nil, fmt.Errorf("failed to get remote URL: %w", err)
	}

	if strings.Contains(url, "github.com") {
		return &GitHubForge{}, nil
	}

	return nil, fmt.Errorf("forge not supported for remote %q — only GitHub is implemented", url)
}
