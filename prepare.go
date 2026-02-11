package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

func runPrepare(cmd *cobra.Command, args []string) error {
	// args[0] = message file, args[1] = source (optional), args[2] = sha (optional)
	msgFile := args[0]

	// When source is "message" the user passed -m; commit-msg will check it later.
	if len(args) > 1 && args[1] == "message" {
		return nil
	}

	bc, err := resolveBlockConfig(cmd)
	if err != nil {
		return err
	}
	if len(bc.Msg) == 0 {
		return nil
	}

	data, err := os.ReadFile(msgFile)
	if err != nil {
		return fmt.Errorf("reading commit message: %w", err)
	}

	// Strip git's comment lines (# ...) before checking — they won't end up in the commit.
	var body []string
	for _, line := range strings.Split(string(data), "\n") {
		if !strings.HasPrefix(line, "#") {
			body = append(body, line)
		}
	}

	pattern, found := matchesPattern(strings.Join(body, "\n"), bc.Msg)
	if !found {
		return nil
	}

	quiet, _ := cmd.Flags().GetBool("quiet")
	if !quiet {
		errorf("match %q in auto-generated commit message", pattern)
		bell()
		hintf("git pre-populated this message (merge, template, or amend)")
		hintf("to commit with your own message: git commit -m \"your message here\"")
		hintf("to edit the message first: git commit -e")
	}
	return fmt.Errorf("policy violation: %q found in auto-generated commit message", pattern)
}

func testPrepare(cmd *cobra.Command, dir string, patterns []string) bool {
	// Write a commit message file that contains a blocklist violation,
	// as if git auto-populated it (e.g. merge or template).
	violation := fmt.Sprintf("Merge branch 'feature-%s-integration'\n", patterns[0])
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte(violation), 0644); err != nil {
		return false
	}

	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	err := runPrepare(cmd, []string{msgFile})
	return err != nil // error means violation detected = pass
}
