package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// stripTrailers removes trailer lines that match any blocklist pattern.
// Returns the filtered lines and the count of removed lines.
func stripTrailers(lines []string, patterns []string) ([]string, int) {
	var kept []string
	removed := 0
	for _, line := range lines {
		if isTrailerLine(line) {
			if _, matched := matchesBlocklist(line, patterns); matched {
				removed++
				continue
			}
		}
		kept = append(kept, line)
	}
	return kept, removed
}

func runMsg(cmd *cobra.Command, args []string) error {
	patterns, err := resolvePatterns(cmd)
	if err != nil {
		return err
	}
	if len(patterns) == 0 {
		return nil
	}

	data, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("reading commit message: %w", err)
	}

	quiet, _ := cmd.Flags().GetBool("quiet")

	// Pass 1: strip matching trailers
	lines := strings.Split(string(data), "\n")
	filtered, removed := stripTrailers(lines, patterns)
	if removed > 0 {
		if err := os.WriteFile(args[0], []byte(strings.Join(filtered, "\n")), 0644); err != nil {
			return fmt.Errorf("rewriting commit message: %w", err)
		}
		if !quiet {
			fmt.Fprintf(os.Stderr, "snag: removed %d trailer line(s)\n", removed)
		}
	}

	// Pass 2: check remaining body for policy violations
	body := strings.Join(filtered, "\n")
	pattern, found := matchesBlocklist(body, patterns)
	if !found {
		return nil
	}

	if !quiet {
		fmt.Fprintf(os.Stderr, "snag: match %q in commit message\n", pattern)
		fmt.Fprintf(os.Stderr, "  to recover: git commit -eF .git/COMMIT_EDITMSG\n")
	}
	return fmt.Errorf("policy violation: %q found in commit message", pattern)
}
