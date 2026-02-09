package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func runDiff(cmd *cobra.Command, args []string) error {
	patterns, err := resolvePatterns(cmd)
	if err != nil {
		return err
	}
	if len(patterns) == 0 {
		return nil
	}

	out, err := exec.Command("git", "diff", "--staged").CombinedOutput()
	if err != nil {
		return fmt.Errorf("git diff --staged: %w\n%s", err, out)
	}

	pattern, found := matchesBlocklist(string(out), patterns)
	if !found {
		return nil
	}

	quiet, _ := cmd.Flags().GetBool("quiet")
	if !quiet {
		fmt.Fprintf(os.Stderr, "snag: match %q in staged diff\n", pattern)
	}
	return fmt.Errorf("policy violation: %q found in staged diff", pattern)
}
