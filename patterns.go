package main

import "strings"

// matchesPattern checks whether text contains any of the given patterns.
// Comparison is case-insensitive. Returns the matched pattern and true on
// the first hit, or ("", false) if nothing matches.
func matchesPattern(text string, patterns []string) (string, bool) {
	lower := strings.ToLower(text)
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return p, true
		}
	}
	return "", false
}

// deduplicatePatterns removes duplicate patterns, preserving first-occurrence order.
func deduplicatePatterns(patterns []string) []string {
	if len(patterns) == 0 {
		return nil
	}
	seen := make(map[string]bool)
	var result []string
	for _, p := range patterns {
		if !seen[p] {
			seen[p] = true
			result = append(result, p)
		}
	}
	return result
}

// stripDiffNoise keeps only added lines from a unified diff.
// After stripDiffMeta removes headers, this filters out removed lines
// (- prefix) and context lines (no prefix), keeping only additions
// (+ prefix) with the leading + stripped.
func stripDiffNoise(diff string) string {
	var added []string
	for _, line := range strings.Split(diff, "\n") {
		if strings.HasPrefix(line, "+") {
			added = append(added, line[1:])
		}
	}
	return strings.Join(added, "\n")
}

// stripDiffMeta removes unified diff metadata lines (headers, index,
// hunk markers) so only actual content is checked for policy violations.
// This prevents filenames in diff headers from triggering false positives.
func stripDiffMeta(diff string) string {
	var content []string
	for _, line := range strings.Split(diff, "\n") {
		if isDiffMeta(line) {
			continue
		}
		content = append(content, line)
	}
	return strings.Join(content, "\n")
}

func isDiffMeta(line string) bool {
	switch {
	case strings.HasPrefix(line, "diff --git "):
		return true
	case strings.HasPrefix(line, "--- a/"), line == "--- /dev/null":
		return true
	case strings.HasPrefix(line, "+++ b/"), line == "+++ /dev/null":
		return true
	case strings.HasPrefix(line, "rename from "),
		strings.HasPrefix(line, "rename to "),
		strings.HasPrefix(line, "copy from "),
		strings.HasPrefix(line, "copy to "):
		return true
	case strings.HasPrefix(line, "index "):
		return true
	case strings.HasPrefix(line, "@@ "):
		return true
	case strings.HasPrefix(line, "old mode "),
		strings.HasPrefix(line, "new mode "),
		strings.HasPrefix(line, "new file mode "),
		strings.HasPrefix(line, "deleted file mode "):
		return true
	case strings.HasPrefix(line, "similarity index "),
		strings.HasPrefix(line, "dissimilarity index "):
		return true
	case strings.HasPrefix(line, "Binary files "):
		return true
	}
	return false
}

// isTrailerLine reports whether line is a valid Git trailer (Key: Value).
// The key must have no spaces, no leading whitespace, and be followed by ": ".
func isTrailerLine(line string) bool {
	if line == "" {
		return false
	}
	if strings.TrimLeft(line, " \t") != line {
		return false
	}
	idx := strings.Index(line, ": ")
	if idx < 1 {
		return false
	}
	key := line[:idx]
	return !strings.Contains(key, " ")
}
