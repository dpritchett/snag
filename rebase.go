package main

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/spf13/cobra"
)

var defaultProtectedBranches = []string{"main", "master"}

// currentBranch returns the short name of HEAD via git symbolic-ref.
func currentBranch() (string, error) {
	out, err := exec.Command("git", "symbolic-ref", "--short", "HEAD").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git symbolic-ref: %w\n%s", err, out)
	}
	return strings.TrimSpace(string(out)), nil
}

// isProtected reports whether branch matches any of the given patterns.
// Patterns are checked as exact matches first, then as path.Match globs.
func isProtected(branch string, patterns []string) bool {
	for _, p := range patterns {
		if branch == p {
			return true
		}
		if matched, _ := path.Match(p, branch); matched {
			return true
		}
	}
	return false
}

func runRebase(cmd *cobra.Command, args []string) error {
	if os.Getenv("SNAG_ALLOW_REBASE") == "1" {
		return nil
	}

	var branch string
	if len(args) >= 2 && args[1] != "" {
		branch = args[1]
	} else {
		b, err := currentBranch()
		if err != nil {
			return err
		}
		branch = b
	}

	bc, err := resolveBlockConfig(cmd)
	if err != nil {
		return err
	}
	patterns := bc.Branch

	if !isProtected(branch, patterns) {
		return nil
	}

	quiet, _ := cmd.Flags().GetBool("quiet")
	if !quiet {
		warnf("rebase of protected branch %q blocked", branch)
		hintf("protected branches: %s", strings.Join(patterns, ", "))
		hintf("to override: SNAG_ALLOW_REBASE=1 git rebase ...")
	}
	return fmt.Errorf("rebase blocked: %q is a protected branch", branch)
}

func testRebase(cmd *cobra.Command, dir string, _ []string) bool {
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	// The temp repo's default branch is main (or master) — already protected.
	// runRebase with no branch arg resolves current branch = main → should block.
	err := runRebase(cmd, nil)
	return err != nil
}
