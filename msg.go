package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
)

// stripMatchingTrailers silently removes git trailer lines (Key: Value) whose
// content matches a block pattern. This rewrites the commit message file in
// place — the commit proceeds without the offending trailers rather than being
// rejected. Useful for auto-injected trailers like Generated-by that you want
// gone without interrupting the developer's flow.
//
// Non-trailer lines are never touched here; those are checked separately in
// pass 2 of runMsg, which *does* reject the commit on a match.
func stripMatchingTrailers(lines []string, patterns []string) ([]string, int) {
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
	bc, err := resolveBlockConfig(cmd)
	if err != nil {
		return err
	}
	if len(bc.Msg) == 0 {
		return nil
	}

	data, err := os.ReadFile(args[0])
	if err != nil {
		return fmt.Errorf("reading commit message: %w", err)
	}

	quiet, _ := cmd.Flags().GetBool("quiet")

	// Pass 1 — silent removal: strip trailer lines (like Generated-by) that
	// match block patterns. The commit message file is rewritten in place so
	// the commit proceeds cleanly without the matched trailers.
	lines := strings.Split(string(data), "\n")
	cleaned, removed := stripMatchingTrailers(lines, bc.Msg)
	if removed > 0 {
		if err := os.WriteFile(args[0], []byte(strings.Join(cleaned, "\n")), 0644); err != nil {
			return fmt.Errorf("rewriting commit message: %w", err)
		}
		if !quiet {
			warnf("removed %d trailer line(s)", removed)
		}
	}

	// Pass 2 — hard reject: check the remaining message body. Unlike pass 1,
	// a match here blocks the commit entirely.
	body := strings.Join(cleaned, "\n")
	pattern, found := matchesBlocklist(body, bc.Msg)
	if !found {
		return nil
	}

	if !quiet {
		errorf("match %q in commit message", pattern)
		bell()
		hintf("to recover: git commit -eF .git/COMMIT_EDITMSG")
	}
	return fmt.Errorf("policy violation: %q found in commit message", pattern)
}
