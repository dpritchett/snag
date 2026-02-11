package main

import (
	"fmt"
	"os/exec"

	"github.com/spf13/cobra"
)

func runDiff(cmd *cobra.Command, args []string) error {
	bc, err := resolveBlockConfig(cmd)
	if err != nil {
		return err
	}
	if len(bc.Diff) == 0 {
		return nil
	}

	out, err := exec.Command("git", "diff", "--staged").CombinedOutput()
	if err != nil {
		return fmt.Errorf("git diff --staged: %w\n%s", err, out)
	}

	pattern, found := matchesPattern(stripDiffNoise(stripDiffMeta(string(out))), bc.Diff)
	if !found {
		return nil
	}

	quiet, _ := cmd.Flags().GetBool("quiet")
	if !quiet {
		errorf("match %q in staged diff", pattern)
		bell()
	}
	return fmt.Errorf("policy violation: %q found in staged diff", pattern)
}
