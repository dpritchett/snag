package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestRunInit(t *testing.T) {
	makeCmd := func() *cobra.Command {
		cmd := buildInitCmd()
		cmd.PersistentFlags().BoolP("quiet", "q", true, "")
		return cmd
	}

	t.Run("creates default snag.toml", func(t *testing.T) {
		dir := t.TempDir()
		orig, _ := os.Getwd()
		os.Chdir(dir)
		defer os.Chdir(orig)

		cmd := makeCmd()
		if err := cmd.RunE(cmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(filepath.Join(dir, "snag.toml"))
		if err != nil {
			t.Fatalf("snag.toml not created: %v", err)
		}
		content := string(data)
		if !strings.Contains(content, "min_version") {
			t.Error("missing min_version field")
		}
		if !strings.Contains(content, "[block]") {
			t.Error("missing [block] section")
		}
		if !strings.Contains(content, "DO NOT MERGE") {
			t.Error("missing default pattern")
		}
	})

	t.Run("refuses to overwrite without --force", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "snag.toml"), []byte("existing"), 0644)

		orig, _ := os.Getwd()
		os.Chdir(dir)
		defer os.Chdir(orig)

		cmd := makeCmd()
		err := cmd.RunE(cmd, nil)
		if err == nil {
			t.Fatal("expected error when snag.toml exists")
		}
		if !strings.Contains(err.Error(), "already exists") {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("overwrites with --force", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "snag.toml"), []byte("old"), 0644)

		orig, _ := os.Getwd()
		os.Chdir(dir)
		defer os.Chdir(orig)

		cmd := makeCmd()
		cmd.Flags().Set("force", "true")
		if err := cmd.RunE(cmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		data, _ := os.ReadFile(filepath.Join(dir, "snag.toml"))
		if string(data) == "old" {
			t.Error("file was not overwritten")
		}
	})
}

func TestRunInitLocal(t *testing.T) {
	makeCmd := func() *cobra.Command {
		cmd := buildInitCmd()
		cmd.PersistentFlags().BoolP("quiet", "q", true, "")
		return cmd
	}

	t.Run("creates default snag-local.toml", func(t *testing.T) {
		dir := t.TempDir()
		orig, _ := os.Getwd()
		os.Chdir(dir)
		defer os.Chdir(orig)

		cmd := makeCmd()
		cmd.Flags().Set("local", "true")
		if err := cmd.RunE(cmd, nil); err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		data, err := os.ReadFile(filepath.Join(dir, "snag-local.toml"))
		if err != nil {
			t.Fatalf("snag-local.toml not created: %v", err)
		}
		content := string(data)
		if !strings.Contains(content, "gitignored") {
			t.Error("missing gitignore comment")
		}
	})

	t.Run("refuses to overwrite local without --force", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "snag-local.toml"), []byte("existing"), 0644)

		orig, _ := os.Getwd()
		os.Chdir(dir)
		defer os.Chdir(orig)

		cmd := makeCmd()
		cmd.Flags().Set("local", "true")
		err := cmd.RunE(cmd, nil)
		if err == nil {
			t.Fatal("expected error when snag-local.toml exists")
		}
	})
}

func TestCompareSemver(t *testing.T) {
	tests := []struct {
		a, b string
		want int
	}{
		{"0.10.0", "0.10.0", 0},
		{"0.10.1", "0.10.0", 1},
		{"0.10.0", "0.10.1", -1},
		{"1.0.0", "0.99.99", 1},
		{"0.9.0", "0.10.0", -1},
		{"1.0.0", "1.0.0", 0},
	}
	for _, tc := range tests {
		t.Run(tc.a+"_vs_"+tc.b, func(t *testing.T) {
			got := compareSemver(tc.a, tc.b)
			if got != tc.want {
				t.Errorf("compareSemver(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
			}
		})
	}
}

func TestCheckMinVersion(t *testing.T) {
	t.Run("dev always passes", func(t *testing.T) {
		old := Version
		Version = "dev"
		defer func() { Version = old }()

		if err := checkMinVersion("99.0.0", "snag.toml"); err != nil {
			t.Errorf("dev build should pass: %v", err)
		}
	})

	t.Run("dev+hash always passes", func(t *testing.T) {
		old := Version
		Version = "dev+abc1234"
		defer func() { Version = old }()

		if err := checkMinVersion("99.0.0", "snag.toml"); err != nil {
			t.Errorf("dev build should pass: %v", err)
		}
	})

	t.Run("sufficient version passes", func(t *testing.T) {
		old := Version
		Version = "0.10.0"
		defer func() { Version = old }()

		if err := checkMinVersion("0.10.0", "snag.toml"); err != nil {
			t.Errorf("equal version should pass: %v", err)
		}
	})

	t.Run("insufficient version fails", func(t *testing.T) {
		old := Version
		Version = "0.9.0"
		defer func() { Version = old }()

		err := checkMinVersion("0.10.0", "snag.toml")
		if err == nil {
			t.Fatal("expected error for old version")
		}
		if !strings.Contains(err.Error(), "requires snag >= 0.10.0") {
			t.Errorf("unexpected error: %v", err)
		}
	})
}
