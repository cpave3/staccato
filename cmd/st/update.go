package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"time"

	"github.com/cpave3/staccato/pkg/selfupdate"
	"github.com/cpave3/staccato/pkg/version"
	"github.com/spf13/cobra"
)

// releaseRepo is where st's releases are published.
const releaseRepo = "cpave3/staccato"

// releasesAPIEnv names another address for GitHub's API, for tests.
const releasesAPIEnv = "ST_RELEASES_API"

// updateReport is what 'st update --json' prints.
type updateReport struct {
	Current   string `json:"current"`
	Latest    string `json:"latest"`
	Available bool   `json:"available"`
	URL       string `json:"url"`
	// Updated is set by an install, and left out by --check.
	Updated *bool `json:"updated,omitempty"`
}

func updateCmd() *cobra.Command {
	var check, asJSON bool
	cmd := &cobra.Command{
		Use:   "update",
		Short: "Install the newest release of st over this one",
		Long: `Asks GitHub for the newest stable release of st and, when it is newer than
this one, installs it in place of the running binary. A link to the binary,
such as ~/.local/bin/st, is followed and left as it is; the file it leads to
is replaced.

The release's binary for this OS and CPU is downloaded beside the running
one, checked against the release's SHA256SUMS, and run with 'version --json',
which has to report the release's version. Only then is it moved into place.
The previous binary is kept as st.prev beside it: rename it back to undo an
update. When any step fails, the running binary is left as it was.

Only a release build updates itself ('st version --json' says "release":
true). A build from a checkout is refused: rebuild it from the checkout. A
'go install' build is refused too: run
'go install github.com/cpave3/staccato/cmd/st@latest' instead.

--check only reports the running version, the newest release and whether it
is newer, and works in any build. With --json, both print
{"current", "latest", "available", "url"}, and an install adds "updated".

st update is the only command that contacts GitHub's releases; no other
command checks for updates. It makes unauthenticated requests, which GitHub
limits to 60 an hour.`,
		Example: `st update
st update --check
st update --check --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			current := version.String()
			if !check && !version.IsRelease() {
				return notUpdatable(current)
			}

			ctx, stop := signal.NotifyContext(cmd.Context(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			client := selfupdate.Client{
				API:   os.Getenv(releasesAPIEnv),
				Repo:  releaseRepo,
				Asset: "st",
				HTTP:  &http.Client{Timeout: 10 * time.Minute},
			}
			rel, err := client.Latest(ctx)
			if errors.Is(err, selfupdate.ErrNoRelease) {
				return fmt.Errorf("no release of st has been published yet")
			}
			if err != nil {
				return fmt.Errorf("finding the newest release: %w", err)
			}
			report := updateReport{
				Current:   current,
				Latest:    rel.Version,
				Available: version.Newer(rel.Version, current),
				URL:       rel.URL,
			}
			out := cmd.OutOrStdout()

			if check {
				if asJSON {
					return writeJSON(out, report)
				}
				switch {
				case report.Available:
					fmt.Fprintf(out, "st %s is installed; %s is available: %s\nRun 'st update' to install it.\n", current, rel.Version, rel.URL)
				case !version.IsRelease():
					fmt.Fprintf(out, "st %s is not a release build; the newest release is %s: %s\n", current, rel.Version, rel.URL)
				default:
					fmt.Fprintf(out, "st %s is up to date (newest release: %s)\n", current, rel.Version)
				}
				return nil
			}

			updated := false
			report.Updated = &updated
			if !report.Available {
				if asJSON {
					return writeJSON(out, report)
				}
				fmt.Fprintf(out, "st %s is up to date (newest release: %s)\n", current, rel.Version)
				return nil
			}

			exe, err := os.Executable()
			if err != nil {
				return fmt.Errorf("finding the running binary: %w", err)
			}
			if err := client.Install(ctx, rel, exe, reportsVersion(rel.Version)); err != nil {
				return fmt.Errorf("installing st %s: %w", rel.Version, err)
			}
			updated = true
			if asJSON {
				return writeJSON(out, report)
			}
			fmt.Fprintf(out, "Updated st %s to %s\nThe previous binary is kept as %s.prev.\n", current, rel.Version, exe)
			return nil
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "Only report whether a newer release exists")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print the result as JSON")
	return cmd
}

// notUpdatable is the refusal for a build that is not a release, saying how
// to update it instead.
func notUpdatable(current string) error {
	if version.FromModule() {
		return fmt.Errorf("this st (%s) was built with 'go install', so st update leaves it to Go: run 'go install github.com/cpave3/staccato/cmd/st@latest'", current)
	}
	return fmt.Errorf("this st (%s) was built from a checkout, not downloaded from a release, so st update leaves it alone: rebuild it from the checkout ('task build')", current)
}

// reportsVersion checks a downloaded binary by running it with
// 'version --json': it has to report want.
func reportsVersion(want string) func(ctx context.Context, binary string) error {
	return func(ctx context.Context, binary string) error {
		ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		var stdout, stderr bytes.Buffer
		run := exec.CommandContext(ctx, binary, "version", "--json")
		run.Stdout, run.Stderr = &stdout, &stderr
		if err := run.Run(); err != nil {
			return fmt.Errorf("running 'version --json' failed: %w: %s", err, bytes.TrimSpace(stderr.Bytes()))
		}
		var got versionInfo
		if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
			return fmt.Errorf("'version --json' printed %q: %w", bytes.TrimSpace(stdout.Bytes()), err)
		}
		if got.Version != want {
			return fmt.Errorf("the downloaded binary reports %s, not %s", got.Version, want)
		}
		return nil
	}
}
