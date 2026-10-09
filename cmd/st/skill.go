package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func skillCmd() *cobra.Command {
	parent := &cobra.Command{
		Use:     "skill",
		Short:   "Install the Staccato skill for AI coding agents",
		Example: `st skill install`,
	}

	parent.AddCommand(skillInstallCmd())
	return parent
}

func skillInstallCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "install",
		Short: "Install the Staccato skill with the skills CLI",
		Long: `Runs 'npx -y skills install cpave3/staccato/skill', passing its prompts and
output through. Needs Node.js (npx) on the PATH.`,
		Example: `st skill install`,
		RunE: func(cmd *cobra.Command, args []string) error {
			npx, err := exec.LookPath("npx")
			if err != nil {
				return fmt.Errorf("npx not found in PATH — install Node.js first")
			}

			c := exec.Command(npx, "-y", "skills", "install", "cpave3/staccato/skill")
			c.Stdout = os.Stdout
			c.Stderr = os.Stderr
			c.Stdin = os.Stdin
			return c.Run()
		},
	}
}
