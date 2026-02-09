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
