package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

var defaultTicketPattern = `(\d+)-`

func ticketPattern() *regexp.Regexp {
	pat := os.Getenv("SNAG_TICKET_PATTERN")
	if pat == "" {
		pat = defaultTicketPattern
	}
	re, err := regexp.Compile(pat)
	if err != nil {
		return regexp.MustCompile(defaultTicketPattern)
	}
	return re
}

func currentBranch() (string, error) {
	out, err := exec.Command("git", "symbolic-ref", "--short", "HEAD").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("not on a branch (detached HEAD?): %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// extractTicket returns the first submatch group if present, otherwise the full match.
// With the default pattern `(\d+)-`, this extracts just the number from "123-".
func extractTicket(branch string) string {
	m := ticketPattern().FindStringSubmatch(branch)
	if m == nil {
		return ""
	}
	if len(m) > 1 && m[1] != "" {
		return m[1]
	}
	return m[0]
}

func runPrepare(cmd *cobra.Command, args []string) error {
	// args[0] = message file, args[1] = source (optional), args[2] = sha (optional)
	msgFile := args[0]

	// Skip merge, squash, and amend commits.
	if len(args) > 1 {
		switch args[1] {
		case "merge", "squash", "commit":
			return nil
		}
	}

	branch, err := currentBranch()
	if err != nil {
		return nil // detached HEAD — nothing to inject
	}

	ticket := extractTicket(branch)
	if ticket == "" {
		return nil // no ticket in branch name
	}

	data, err := os.ReadFile(msgFile)
	if err != nil {
		return fmt.Errorf("reading commit message: %w", err)
	}
	msg := string(data)

	prefix := "#" + ticket
	if strings.Contains(msg, prefix) {
		return nil // already present
	}

	// Prepend ticket to first line.
	lines := strings.SplitN(msg, "\n", 2)
	lines[0] = prefix + " " + lines[0]
	updated := strings.Join(lines, "\n")

	if err := os.WriteFile(msgFile, []byte(updated), 0644); err != nil {
		return fmt.Errorf("writing commit message: %w", err)
	}

	quiet, _ := cmd.Flags().GetBool("quiet")
	if !quiet {
		infof("prepended %s from branch %s", prefix, branch)
	}
	return nil
}

func testPrepare(cmd *cobra.Command, dir string, _ []string) bool {
	run := func(args ...string) error {
		c := exec.Command(args[0], args[1:]...)
		c.Dir = dir
		out, err := c.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s: %w\n%s", strings.Join(args, " "), err, out)
		}
		return nil
	}
	if err := run("git", "checkout", "-b", "feat/42-demo"); err != nil {
		return false
	}

	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte("Add new feature\n"), 0644); err != nil {
		return false
	}

	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	err := runPrepare(cmd, []string{msgFile})
	if err != nil {
		return false
	}

	data, err := os.ReadFile(msgFile)
	if err != nil {
		return false
	}
	return strings.HasPrefix(string(data), "#42 ")
}
