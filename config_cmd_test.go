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
		cmd.Flags().String("blocklist", ".blocklist", "")
		cmd.Flags().BoolP("quiet", "q", false, "")
		return cmd
	}

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
		t.Setenv("SNAG_BLOCKLIST", "")
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

	t.Run("legacy blocklist source", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, ".blocklist"), []byte("secret-word\n"), 0644)

		orig, _ := os.Getwd()
		os.Chdir(dir)
		defer os.Chdir(orig)
		t.Setenv("SNAG_BLOCKLIST", "")
		t.Setenv("SNAG_PROTECTED_BRANCHES", "")

		sources, err := collectSources(makeCmd())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		blCount := 0
		for _, s := range sources {
			if s.Kind == "blocklist" {
				blCount++
			}
		}
		if blCount != 1 {
			t.Errorf("expected 1 blocklist source, got %d", blCount)
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
		t.Setenv("SNAG_BLOCKLIST", "env-pattern")
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
		// Should have SNAG_BLOCKLIST + SNAG_PROTECTED_BRANCHES
		if envCount != 2 {
			t.Errorf("expected 2 env sources, got %d", envCount)
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
		t.Setenv("SNAG_BLOCKLIST", "")
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
		t.Setenv("SNAG_BLOCKLIST", "")
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

	t.Run("blocklist flag override", func(t *testing.T) {
		dir := t.TempDir()
		blFile := filepath.Join(dir, "custom.blocklist")
		os.WriteFile(blFile, []byte("custom-word\n"), 0644)

		orig, _ := os.Getwd()
		os.Chdir(dir)
		defer os.Chdir(orig)
		t.Setenv("SNAG_BLOCKLIST", "")
		t.Setenv("SNAG_PROTECTED_BRANCHES", "")

		cmd := makeCmd()
		cmd.Flags().Set("blocklist", blFile)

		sources, err := collectSources(cmd)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(sources) == 0 {
			t.Fatal("expected at least one source")
		}
		if sources[0].Kind != "blocklist" {
			t.Errorf("first source should be blocklist, got %s", sources[0].Kind)
		}
	})

	t.Run("no config anywhere", func(t *testing.T) {
		dir := t.TempDir()

		orig, _ := os.Getwd()
		os.Chdir(dir)
		defer os.Chdir(orig)
		t.Setenv("SNAG_BLOCKLIST", "")
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
}
