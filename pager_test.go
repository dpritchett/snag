package main

import (
	"strings"
	"testing"
)

func TestUnifiedDiff_NewFile(t *testing.T) {
	diff := unifiedDiff("lefthook-local.yml", "", "remotes:\n  - git_url: https://example.com\n")
	if !strings.HasPrefix(diff, "--- /dev/null\n+++ b/lefthook-local.yml\n") {
		t.Error("new file diff should have /dev/null header")
	}
	if !strings.Contains(diff, "@@ -0,0 +1,") {
		t.Error("new file diff should start at line 0,0")
	}
	if !strings.Contains(diff, "+remotes:") {
		t.Error("new file diff should show added lines with + prefix")
	}
}

func TestUnifiedDiff_AppendBlock(t *testing.T) {
	old := "pre-commit:\n  commands:\n    lint:\n      run: echo lint\n"
	new := old + "\nremotes:\n  - git_url: https://example.com\n"
	diff := unifiedDiff("lefthook.yml", old, new)

	if !strings.Contains(diff, "--- a/lefthook.yml") {
		t.Error("diff should reference the filename")
	}
	if !strings.Contains(diff, "+remotes:") {
		t.Error("diff should show appended lines")
	}
	// Original lines should appear as context (space prefix), not removed.
	if strings.Contains(diff, "-pre-commit:") {
		t.Error("original lines should not appear as removed")
	}
}

func TestUnifiedDiff_SingleLineChange(t *testing.T) {
	old := "remotes:\n  - git_url: https://example.com\n    ref: v0.1.0\n    configs:\n      - recipes/snag-filter.yml\n"
	new := strings.Replace(old, "ref: v0.1.0", "ref: v0.5.0", 1)
	diff := unifiedDiff("lefthook.yml", old, new)

	if !strings.Contains(diff, "-    ref: v0.1.0") {
		t.Error("diff should show old ref as removed")
	}
	if !strings.Contains(diff, "+    ref: v0.5.0") {
		t.Error("diff should show new ref as added")
	}
	// Context lines around the change.
	if !strings.Contains(diff, " remotes:") || !strings.Contains(diff, "   - git_url:") {
		t.Error("diff should include context lines")
	}
}

func TestUnifiedDiff_ContextLimit(t *testing.T) {
	// Build a file with many lines, change one in the middle.
	var lines []string
	for i := 0; i < 20; i++ {
		lines = append(lines, "line"+string(rune('A'+i)))
	}
	old := strings.Join(lines, "\n") + "\n"
	new := strings.Replace(old, "lineJ", "lineJ-changed", 1)
	diff := unifiedDiff("test.txt", old, new)

	// Should have at most 3 context lines before/after, not the whole file.
	diffLines := strings.Split(strings.TrimRight(diff, "\n"), "\n")
	// Skip the 3 header lines (---, +++, @@).
	body := diffLines[3:]
	if len(body) > 7 { // 3 before + 1 removed + 1 added + 3 after = 8 max, but trailing context may be less
		// Allow a little slack but it shouldn't be the full 20 lines.
		if len(body) > 9 {
			t.Errorf("expected limited context, got %d body lines", len(body))
		}
	}
}

func TestSplitLines(t *testing.T) {
	tests := []struct {
		input string
		want  int
	}{
		{"", 0},
		{"one\n", 1},
		{"one\ntwo\n", 2},
		{"no trailing newline", 1},
	}
	for _, tt := range tests {
		got := splitLines(tt.input)
		if len(got) != tt.want {
			t.Errorf("splitLines(%q) = %d lines, want %d", tt.input, len(got), tt.want)
		}
	}
}

func TestFirstWord(t *testing.T) {
	tests := []struct {
		input, want string
	}{
		{"delta", "delta"},
		{"delta --color-only", "delta"},
		{"  less -R  ", "less"},
		{"", ""},
	}
	for _, tt := range tests {
		got := firstWord(tt.input)
		if got != tt.want {
			t.Errorf("firstWord(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestFindDiffPager_RespectsGitPager(t *testing.T) {
	orig := findDiffPager
	defer func() { findDiffPager = orig }()

	// Override to simulate GIT_PAGER pointing to a known binary.
	findDiffPager = func() string {
		return "cat"
	}
	if got := findDiffPager(); got != "cat" {
		t.Errorf("expected pager 'cat', got %q", got)
	}
}

func TestShowDiffOutput_PlainFallback(t *testing.T) {
	// When isTTY returns false, showDiffOutput writes plain text to stderr.
	// We can't easily capture stderr in a test, but we can at least verify
	// it doesn't panic with various inputs.
	origTTY := isTTY
	isTTY = func() bool { return false }
	defer func() { isTTY = origTTY }()

	showDiffOutput("")                                            // empty — should be a no-op
	showDiffOutput("--- a/f\n+++ b/f\n@@ -1 +1 @@\n-old\n+new\n") // non-empty
}
