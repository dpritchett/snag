package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadBlocklist(t *testing.T) {
	t.Run("mixed content", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".blocklist")
		content := "# comment\n\nTODO\nfixme\n  HACK  \n# another comment\n"
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatal(err)
		}
		patterns, err := loadBlocklist(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := []string{"todo", "fixme", "hack"}
		if len(patterns) != len(want) {
			t.Fatalf("got %d patterns, want %d", len(patterns), len(want))
		}
		for i, p := range patterns {
			if p != want[i] {
				t.Errorf("patterns[%d] = %q, want %q", i, p, want[i])
			}
		}
	})

	t.Run("missing file", func(t *testing.T) {
		patterns, err := loadBlocklist("/no/such/file")
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if patterns != nil {
			t.Fatalf("expected nil patterns, got %v", patterns)
		}
	})

	t.Run("empty file", func(t *testing.T) {
		dir := t.TempDir()
		path := filepath.Join(dir, ".blocklist")
		if err := os.WriteFile(path, []byte(""), 0644); err != nil {
			t.Fatal(err)
		}
		patterns, err := loadBlocklist(path)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(patterns) != 0 {
			t.Fatalf("expected empty slice, got %v", patterns)
		}
	})
}

func TestWalkBlocklists(t *testing.T) {
	t.Run("merges parent and child", func(t *testing.T) {
		parent := t.TempDir()
		child := filepath.Join(parent, "child")
		grandchild := filepath.Join(child, "grandchild")
		os.MkdirAll(grandchild, 0755)

		os.WriteFile(filepath.Join(parent, ".blocklist"), []byte("parent-word\n"), 0644)
		os.WriteFile(filepath.Join(child, ".blocklist"), []byte("child-word\n"), 0644)
		// grandchild has no .blocklist

		patterns, err := walkBlocklists(grandchild)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		has := func(want string) bool {
			for _, p := range patterns {
				if p == want {
					return true
				}
			}
			return false
		}
		if !has("parent-word") {
			t.Error("missing parent-word from parent .blocklist")
		}
		if !has("child-word") {
			t.Error("missing child-word from child .blocklist")
		}
	})

	t.Run("no blocklists found", func(t *testing.T) {
		dir := t.TempDir()
		patterns, err := walkBlocklists(dir)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(patterns) != 0 {
			t.Fatalf("expected empty patterns, got %v", patterns)
		}
	})
}

func TestLoadEnvBlocklist(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		t.Setenv("SNAG_BLOCKLIST", "")
		if p := loadEnvBlocklist(); len(p) != 0 {
			t.Fatalf("expected nil, got %v", p)
		}
	})

	t.Run("single pattern", func(t *testing.T) {
		t.Setenv("SNAG_BLOCKLIST", "forbidden")
		p := loadEnvBlocklist()
		if len(p) != 1 || p[0] != "forbidden" {
			t.Fatalf("got %v, want [forbidden]", p)
		}
	})

	t.Run("multi-line with comments", func(t *testing.T) {
		t.Setenv("SNAG_BLOCKLIST", "# comment\nword1\n\nWORD2\n# trailing")
		p := loadEnvBlocklist()
		want := []string{"word1", "word2"}
		if len(p) != len(want) {
			t.Fatalf("got %d patterns, want %d", len(p), len(want))
		}
		for i := range want {
			if p[i] != want[i] {
				t.Errorf("patterns[%d] = %q, want %q", i, p[i], want[i])
			}
		}
	})

	t.Run("colon-separated", func(t *testing.T) {
		t.Setenv("SNAG_BLOCKLIST", "word1:WORD2:word3")
		p := loadEnvBlocklist()
		want := []string{"word1", "word2", "word3"}
		if len(p) != len(want) {
			t.Fatalf("got %d patterns, want %d", len(p), len(want))
		}
		for i := range want {
			if p[i] != want[i] {
				t.Errorf("patterns[%d] = %q, want %q", i, p[i], want[i])
			}
		}
	})

	t.Run("mixed colons and newlines", func(t *testing.T) {
		t.Setenv("SNAG_BLOCKLIST", "word1:word2\nword3")
		p := loadEnvBlocklist()
		if len(p) != 3 {
			t.Fatalf("got %d patterns, want 3", len(p))
		}
	})
}

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

func TestMatchesBlocklist(t *testing.T) {
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
			gotPattern, gotMatch := matchesBlocklist(tc.text, tc.patterns)
			if gotPattern != tc.wantPattern || gotMatch != tc.wantMatch {
				t.Errorf("matchesBlocklist(%q, ...) = (%q, %v), want (%q, %v)",
					tc.text, gotPattern, gotMatch, tc.wantPattern, tc.wantMatch)
			}
		})
	}
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
