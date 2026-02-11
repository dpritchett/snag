package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// unpushedRange returns the git revision range covering unpushed commits.
// If an upstream is configured it returns "@{upstream}..HEAD".
// Otherwise it falls back to "HEAD" (the single tip commit).
func unpushedRange() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--verify", "@{upstream}")
	if err := cmd.Run(); err == nil {
		return "@{upstream}..HEAD", nil
	}
	// No upstream tracked — check the tip commit only.
	return "HEAD", nil
}

// unpushedCommits returns the list of commit SHAs in the given revision range.
func unpushedCommits(revRange string) ([]string, error) {
	out, err := exec.Command("git", "rev-list", revRange).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git rev-list %s: %w\n%s", revRange, err, out)
	}
	text := strings.TrimSpace(string(out))
	if text == "" {
		return nil, nil
	}
	return strings.Split(text, "\n"), nil
}

func runPush(cmd *cobra.Command, args []string) error {
	bc, err := resolveBlockConfig(cmd)
	if err != nil {
		return err
	}
	patterns := bc.PushPatterns()
	if len(patterns) == 0 {
		return nil
	}

	revRange, err := unpushedRange()
	if err != nil {
		return err
	}

	shas, err := unpushedCommits(revRange)
	if err != nil {
		return err
	}
	if len(shas) == 0 {
		return nil
	}

	quiet, _ := cmd.Flags().GetBool("quiet")

	for _, sha := range shas {
		short := sha[:7]

		// Check commit message
		msgOut, err := exec.Command("git", "log", "-1", "--format=%B", sha).CombinedOutput()
		if err != nil {
			return fmt.Errorf("git log %s: %w\n%s", short, err, msgOut)
		}
		if pattern, found := matchesPattern(string(msgOut), patterns); found {
			if !quiet {
				errorf("match %q in message of %s", pattern, short)
				bell()
			}
			return fmt.Errorf("policy violation: %q found in message of %s", pattern, short)
		}

		// Check commit diff
		diffOut, err := exec.Command("git", "diff-tree", "-p", sha).CombinedOutput()
		if err != nil {
			return fmt.Errorf("git diff-tree %s: %w\n%s", short, err, diffOut)
		}
		if pattern, found := matchesPattern(stripDiffNoise(stripDiffMeta(string(diffOut))), patterns); found {
			if !quiet {
				errorf("match %q in diff of %s", pattern, short)
				bell()
			}
			return fmt.Errorf("policy violation: %q found in diff of %s", pattern, short)
		}
	}

	if !quiet {
		infof("%d patterns checked against %d commits", len(patterns), len(shas))
	}
	return nil
}
