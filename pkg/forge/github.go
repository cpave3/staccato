package forge

import (
	"fmt"
	"os"
	"os/exec"
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
