package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// unifiedDiff generates a minimal unified diff between old and new content for filename.
func unifiedDiff(filename, oldText, newText string) string {
	oldLines := splitLines(oldText)
	newLines := splitLines(newText)

	var b strings.Builder
	if oldText == "" {
		// New file.
		fmt.Fprintf(&b, "--- /dev/null\n")
		fmt.Fprintf(&b, "+++ b/%s\n", filename)
		fmt.Fprintf(&b, "@@ -0,0 +1,%d @@\n", len(newLines))
		for _, line := range newLines {
			fmt.Fprintf(&b, "+%s\n", line)
		}
		return b.String()
	}

	// Find the first and last differing lines for a single hunk.
	start := 0
	for start < len(oldLines) && start < len(newLines) && oldLines[start] == newLines[start] {
		start++
	}
	endOld := len(oldLines)
	endNew := len(newLines)
	for endOld > start && endNew > start && oldLines[endOld-1] == newLines[endNew-1] {
		endOld--
		endNew--
	}

	// Context: up to 3 lines before and after.
	ctxBefore := 3
	if start < ctxBefore {
		ctxBefore = start
	}
	ctxAfterOld := 3
	if len(oldLines)-endOld < ctxAfterOld {
		ctxAfterOld = len(oldLines) - endOld
	}
	ctxAfterNew := 3
	if len(newLines)-endNew < ctxAfterNew {
		ctxAfterNew = len(newLines) - endNew
	}
	// Use the smaller of the two after-contexts (they should be equal for our diffs).
	ctxAfter := ctxAfterOld
	if ctxAfterNew < ctxAfter {
		ctxAfter = ctxAfterNew
	}

	hunkStartOld := start - ctxBefore
	hunkStartNew := start - ctxBefore
	hunkEndOld := endOld + ctxAfter
	hunkEndNew := endNew + ctxAfter

	fmt.Fprintf(&b, "--- a/%s\n", filename)
	fmt.Fprintf(&b, "+++ b/%s\n", filename)
	fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n",
		hunkStartOld+1, hunkEndOld-hunkStartOld,
		hunkStartNew+1, hunkEndNew-hunkStartNew)

	// Leading context.
	for i := hunkStartOld; i < start; i++ {
		fmt.Fprintf(&b, " %s\n", oldLines[i])
	}
	// Removed lines.
	for i := start; i < endOld; i++ {
		fmt.Fprintf(&b, "-%s\n", oldLines[i])
	}
	// Added lines.
	for i := start; i < endNew; i++ {
		fmt.Fprintf(&b, "+%s\n", newLines[i])
	}
	// Trailing context.
	for i := endOld; i < hunkEndOld; i++ {
		fmt.Fprintf(&b, " %s\n", oldLines[i])
	}

	return b.String()
}

// splitLines splits text into lines, handling the trailing newline correctly.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	text = strings.TrimRight(text, "\n")
	return strings.Split(text, "\n")
}

// findDiffPager returns the user's preferred diff pager command, checking
// GIT_PAGER, git config core.pager, PAGER, in that order. Returns "" if
// none configured or the binary isn't found on PATH.
var findDiffPager = func() string {
	// GIT_PAGER takes top priority.
	if p := os.Getenv("GIT_PAGER"); p != "" {
		if name := firstWord(p); name != "" {
			if _, err := exec.LookPath(name); err == nil {
				return p
			}
		}
	}

	// git config core.pager.
	if out, err := exec.Command("git", "config", "core.pager").Output(); err == nil {
		p := strings.TrimSpace(string(out))
		if p != "" {
			if name := firstWord(p); name != "" {
				if _, err := exec.LookPath(name); err == nil {
					return p
				}
			}
		}
	}

	// PAGER env var.
	if p := os.Getenv("PAGER"); p != "" {
		if name := firstWord(p); name != "" {
			if _, err := exec.LookPath(name); err == nil {
				return p
			}
		}
	}

	return ""
}

// firstWord returns the first whitespace-delimited token from s.
func firstWord(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, " \t"); i != -1 {
		return s[:i]
	}
	return s
}

// showDiffOutput writes diff text to stderr, piping through the user's pager
// when stderr is a TTY and a pager is available.
func showDiffOutput(diff string) {
	if diff == "" {
		return
	}

	if isTTY() {
		if pager := findDiffPager(); pager != "" {
			cmd := exec.Command("sh", "-c", pager)
			cmd.Stdin = strings.NewReader(diff)
			cmd.Stdout = os.Stderr // pager output goes to stderr like the rest of our output
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err == nil {
				return
			}
			// Fall through to plain output on pager error.
		}
	}

	fmt.Fprint(os.Stderr, diff)
}
