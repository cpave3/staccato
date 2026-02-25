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

// Forge abstracts PR operations across hosting providers.
type Forge interface {
	CreatePR(opts PRCreateOpts) error
	ViewPR(opts PRViewOpts) error
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
