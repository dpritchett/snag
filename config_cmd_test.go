package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/spf13/cobra"
)

func TestCollectSources(t *testing.T) {
	makeCmd := func() *cobra.Command {
		cmd := &cobra.Command{}
		cmd.Flags().BoolP("quiet", "q", false, "")
		return cmd
	}
	t.Setenv("SNAG_IGNORE", "")

	t.Run("snag.toml source", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "snag.toml"), []byte(`
[block]
diff = ["HACK"]
msg  = ["WIP"]
branch = ["main"]
`), 0644)

		orig, _ := os.Getwd()
		os.Chdir(dir)
		defer os.Chdir(orig)
		t.Setenv("SNAG_PROTECTED_BRANCHES", "")

		sources, err := collectSources(makeCmd())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should have: 1 toml source (has branch, so no defaults)
		tomlCount := 0
		for _, s := range sources {
			if s.Kind == "toml" {
				tomlCount++
				if len(s.Diff) != 1 || s.Diff[0] != "HACK" {
					t.Errorf("diff: got %v, want [HACK]", s.Diff)
				}
			}
		}
		if tomlCount != 1 {
			t.Errorf("expected 1 toml source, got %d", tomlCount)
		}
	})

	t.Run("env var sources", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "snag.toml"), []byte(`
[block]
diff = ["HACK"]
`), 0644)

		orig, _ := os.Getwd()
		os.Chdir(dir)
		defer os.Chdir(orig)
		t.Setenv("SNAG_PROTECTED_BRANCHES", "staging")

		sources, err := collectSources(makeCmd())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		envCount := 0
		for _, s := range sources {
			if s.Kind == "env" {
				envCount++
			}
		}
		// Should have SNAG_PROTECTED_BRANCHES
		if envCount != 1 {
			t.Errorf("expected 1 env source, got %d", envCount)
		}
	})

	t.Run("default branches when none set", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "snag.toml"), []byte(`
[block]
diff = ["HACK"]
`), 0644)

		orig, _ := os.Getwd()
		os.Chdir(dir)
		defer os.Chdir(orig)
		t.Setenv("SNAG_PROTECTED_BRANCHES", "")

		sources, err := collectSources(makeCmd())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		hasDefault := false
		for _, s := range sources {
			if s.Kind == "default" {
				hasDefault = true
			}
		}
		if !hasDefault {
			t.Error("expected default branch source when no branches set")
		}
	})

	t.Run("no defaults when branch set in toml", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "snag.toml"), []byte(`
[block]
diff = ["HACK"]
branch = ["main"]
`), 0644)

		orig, _ := os.Getwd()
		os.Chdir(dir)
		defer os.Chdir(orig)
		t.Setenv("SNAG_PROTECTED_BRANCHES", "")

		sources, err := collectSources(makeCmd())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		for _, s := range sources {
			if s.Kind == "default" {
				t.Error("should not have default source when branch is set in toml")
			}
		}
	})

	t.Run("no config anywhere", func(t *testing.T) {
		dir := t.TempDir()

		orig, _ := os.Getwd()
		os.Chdir(dir)
		defer os.Chdir(orig)
		t.Setenv("SNAG_PROTECTED_BRANCHES", "")

		sources, err := collectSources(makeCmd())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should still have default branches
		if len(sources) != 1 || sources[0].Kind != "default" {
			t.Errorf("expected only default source, got %v", sources)
		}
	})

	t.Run("SNAG_IGNORE source", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "snag.toml"), []byte(`
[block]
diff = ["HACK", "FIXME"]
msg  = ["WIP"]
branch = ["main"]
`), 0644)

		orig, _ := os.Getwd()
		os.Chdir(dir)
		defer os.Chdir(orig)
		t.Setenv("SNAG_PROTECTED_BRANCHES", "")
		t.Setenv("SNAG_IGNORE", "diff:hack")

		sources, err := collectSources(makeCmd())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		ignoreCount := 0
		for _, s := range sources {
			if s.Kind == "ignore" {
				ignoreCount++
				if len(s.Diff) != 1 || s.Diff[0] != "hack" {
					t.Errorf("ignore diff: got %v, want [hack]", s.Diff)
				}
			}
		}
		if ignoreCount != 1 {
			t.Errorf("expected 1 ignore source, got %d", ignoreCount)
		}
	})
}
