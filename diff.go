package main

import (
	"fmt"
	"os"
	"os/exec"

	"github.com/spf13/cobra"
)

func runDiff(cmd *cobra.Command, args []string) error {
	path, err := cmd.Flags().GetString("blocklist")
	if err != nil {
		return err
	}

	patterns, err := loadBlocklist(path)
	if err != nil {
		return fmt.Errorf("loading blocklist: %w", err)
	}
	if patterns == nil {
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
