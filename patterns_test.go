package main

import (
	"strings"
	"testing"
)

func TestDeduplicatePatterns(t *testing.T) {
	t.Run("nil input", func(t *testing.T) {
		if p := deduplicatePatterns(nil); p != nil {
			t.Fatalf("expected nil, got %v", p)
		}
	})

	t.Run("no duplicates", func(t *testing.T) {
		p := deduplicatePatterns([]string{"a", "b", "c"})
		if len(p) != 3 {
			t.Fatalf("expected 3, got %d", len(p))
		}
	})

	t.Run("with duplicates preserves order", func(t *testing.T) {
		p := deduplicatePatterns([]string{"a", "b", "a", "c", "b"})
		want := []string{"a", "b", "c"}
		if len(p) != len(want) {
			t.Fatalf("got %d patterns, want %d", len(p), len(want))
		}
		for i := range want {
			if p[i] != want[i] {
				t.Errorf("patterns[%d] = %q, want %q", i, p[i], want[i])
			}
		}
	})
}

func TestMatchesPattern(t *testing.T) {
	patterns := []string{"todo", "fixme", "hack"}

	tests := []struct {
		name        string
		text        string
		patterns    []string
		wantPattern string
		wantMatch   bool
	}{
		{"match found", "// TODO: fix this", patterns, "todo", true},
		{"no match", "clean code here", patterns, "", false},
		{"empty patterns", "TODO", nil, "", false},
		{"empty text", "", patterns, "", false},
		{"case insensitive", "FIXME LATER", patterns, "fixme", true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotPattern, gotMatch := matchesPattern(tc.text, tc.patterns)
			if gotPattern != tc.wantPattern || gotMatch != tc.wantMatch {
				t.Errorf("matchesPattern(%q, ...) = (%q, %v), want (%q, %v)",
					tc.text, gotPattern, gotMatch, tc.wantPattern, tc.wantMatch)
			}
		})
	}
}

func TestStripDiffMeta(t *testing.T) {
	t.Run("strips all metadata, keeps content", func(t *testing.T) {
		diff := `diff --git a/secret.env b/secret.env
new file mode 100644
index 0000000..abc1234
--- /dev/null
+++ b/secret.env
@@ -0,0 +1,2 @@
+API_KEY=hunter2
+DB_HOST=localhost`

		got := stripDiffMeta(diff)
		if _, found := matchesPattern(got, []string{"secret.env"}); found {
			t.Error("filename should have been stripped from diff metadata")
		}
		if _, found := matchesPattern(got, []string{"hunter2"}); !found {
			t.Error("content line should still be present")
		}
	})

	t.Run("strips rename headers", func(t *testing.T) {
		diff := `diff --git a/old.env b/new.env
similarity index 100%
rename from old.env
rename to new.env`

		got := stripDiffMeta(diff)
		if _, found := matchesPattern(got, []string{"old.env"}); found {
			t.Error("rename from filename should be stripped")
		}
		if _, found := matchesPattern(got, []string{"new.env"}); found {
			t.Error("rename to filename should be stripped")
		}
	})

	t.Run("preserves added and removed lines", func(t *testing.T) {
		diff := `diff --git a/f b/f
--- a/f
+++ b/f
@@ -1 +1 @@
-old password here
+new password here`

		got := stripDiffMeta(diff)
		if _, found := matchesPattern(got, []string{"password"}); !found {
			t.Error("content with 'password' should be preserved")
		}
	})

	t.Run("empty input", func(t *testing.T) {
		if got := stripDiffMeta(""); got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}

func TestStripDiffNoise(t *testing.T) {
	t.Run("keeps only added lines", func(t *testing.T) {
		// After stripDiffMeta, a diff has content lines like:
		// +added line
		// -removed line
		//  context line (space prefix)
		input := "+added line\n-removed line\n context line\n+another add"
		got := stripDiffNoise(input)
		if !strings.Contains(got, "added line") {
			t.Error("should keep added lines")
		}
		if !strings.Contains(got, "another add") {
			t.Error("should keep second added line")
		}
		if strings.Contains(got, "removed line") {
			t.Error("should exclude removed lines")
		}
		if strings.Contains(got, "context line") {
			t.Error("should exclude context lines")
		}
	})

	t.Run("strips leading plus", func(t *testing.T) {
		got := stripDiffNoise("+hello world")
		if got != "hello world" {
			t.Errorf("expected %q, got %q", "hello world", got)
		}
	})

	t.Run("empty input", func(t *testing.T) {
		got := stripDiffNoise("")
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})

	t.Run("no added lines", func(t *testing.T) {
		got := stripDiffNoise("-removed\n context")
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}

func TestIsTrailerLine(t *testing.T) {
	tests := []struct {
		name string
		line string
		want bool
	}{
		{"signed-off-by", "Signed-off-by: Name", true},
		{"co-authored-by", "Co-authored-by: Name", true},
		{"not a trailer", "not a trailer", false},
		{"space in key", "Has Space: value", false},
		{"no colon", "NoColon", false},
		{"leading whitespace", "  Leading-Whitespace: val", false},
		{"empty", "", false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := isTrailerLine(tc.line)
			if got != tc.want {
				t.Errorf("isTrailerLine(%q) = %v, want %v", tc.line, got, tc.want)
			}
		})
	}
}
