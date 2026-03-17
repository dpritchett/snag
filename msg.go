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
			if _, matched := matchesPattern(line, patterns); matched {
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
	if len(bc.Msg) == 0 && bc.MsgMaxLen == 0 && bc.MsgMaxLines == 0 {
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

	// Pass 1.5 — structural limits: check line length and line count.
	content := msgContentLines(cleaned)
	if bc.MsgMaxLen > 0 && len(content) > 0 {
		first := content[0]
		if len(first) > bc.MsgMaxLen {
			if !quiet {
				errorf("first line is %d chars (limit: %d)", len(first), bc.MsgMaxLen)
				bell()
				hintf("to recover: git commit -eF .git/COMMIT_EDITMSG")
			}
			return fmt.Errorf("policy violation: first line exceeds %d characters (%d)", bc.MsgMaxLen, len(first))
		}
	}
	if bc.MsgMaxLines > 0 && len(content) > bc.MsgMaxLines {
		if !quiet {
			errorf("commit message has %d lines (limit: %d)", len(content), bc.MsgMaxLines)
			bell()
			hintf("to recover: git commit -eF .git/COMMIT_EDITMSG")
		}
		return fmt.Errorf("policy violation: commit message exceeds %d lines (%d)", bc.MsgMaxLines, len(content))
	}

	// Pass 2 — hard reject: check the remaining message body. Unlike pass 1,
	// a match here blocks the commit entirely.
	body := strings.Join(cleaned, "\n")
	pattern, found := matchesPattern(body, bc.Msg)
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

// msgContentLines returns non-blank, non-comment lines from a commit message.
// Comment lines (# prefix) and blank lines are excluded from structural checks.
func msgContentLines(lines []string) []string {
	var out []string
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		out = append(out, line)
	}
	return out
}
