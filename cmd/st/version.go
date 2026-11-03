package main

import (
	"fmt"

	"github.com/cpave3/staccato/pkg/version"
	"github.com/spf13/cobra"
)

// versionInfo is what 'st version --json' prints.
type versionInfo struct {
	Version string `json:"version"`
	Release bool   `json:"release"`
}

func versionCmd() *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Print the version of st",
		Long: `Prints 'st <version>'. A release build reports the release it was built as,
such as v0.4.2. A build made with 'go install …@vX.Y.Z' reports that module
version, and a build from a checkout reports 'dev'.

With --json it prints {"version": "<version>", "release": <bool>}, where
release is true only for a release build, the only kind 'st update' updates.

It works anywhere, inside a git repository or not, and reads no stack.`,
		Example: `st version
st version --json`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if asJSON {
				return writeJSON(cmd.OutOrStdout(), versionInfo{Version: version.String(), Release: version.IsRelease()})
			}
			fmt.Fprintf(cmd.OutOrStdout(), "st %s\n", version.String())
			return nil
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Print the version as JSON")
	return cmd
}
