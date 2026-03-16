package main

import (
	"fmt"
	"os/exec"
	"strings"

	"github.com/spf13/cobra"
)

// unpushedCommits returns commit SHAs not yet on any remote.
// With an upstream configured it uses @{upstream}..HEAD.
// Without one it uses HEAD --not --remotes to exclude commits
// already reachable from any remote tracking ref.
func unpushedCommits() ([]string, error) {
	var args []string
	if exec.Command("git", "rev-parse", "--verify", "@{upstream}").Run() == nil {
		args = []string{"rev-list", "@{upstream}..HEAD"}
	} else {
		args = []string{"rev-list", "HEAD", "--not", "--remotes"}
	}
	out, err := exec.Command("git", args...).CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git rev-list: %w\n%s", err, out)
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

	shas, err := unpushedCommits()
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
