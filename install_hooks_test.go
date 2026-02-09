package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallHooks_NoLefthookYml(t *testing.T) {
	dir := t.TempDir()
	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"install-hooks"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error when no lefthook.yml exists")
	}
	if !strings.Contains(err.Error(), "no lefthook.yml found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInstallHooks_AddsRemote(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte("pre-commit:\n  commands:\n    lint:\n      run: echo lint\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"install-hooks"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	content := string(data)
	if !strings.Contains(content, "github.com/dpritchett/snag") {
		t.Error("expected snag remote in lefthook.yml")
	}
	if !strings.Contains(content, "recipes/lefthook-blocklist.yml") {
		t.Error("expected blocklist recipe in lefthook.yml")
	}
}

func TestInstallHooks_UpdatesRef(t *testing.T) {
	dir := t.TempDir()
	initial := `pre-commit:
  commands:
    lint:
      run: echo lint
remotes:
  - git_url: https://github.com/dpritchett/snag.git
    ref: v0.1.0
    configs:
      - recipes/lefthook-blocklist.yml
`
	os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte(initial), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"install-hooks"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	content := string(data)
	if strings.Contains(content, "v0.1.0") {
		t.Error("old ref v0.1.0 should have been replaced")
	}
	if !strings.Contains(content, Version) {
		t.Errorf("expected current version %s in lefthook.yml, got:\n%s", Version, content)
	}
}

func TestInstallHooks_Idempotent(t *testing.T) {
	dir := t.TempDir()
	initial := `pre-commit:
  commands:
    lint:
      run: echo lint
remotes:
  - git_url: https://github.com/dpritchett/snag.git
    ref: ` + Version + `
    configs:
      - recipes/lefthook-blocklist.yml
`
	os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte(initial), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"install-hooks"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File should not have been rewritten — read it and check it's unchanged.
	data, _ := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	if string(data) != initial {
		t.Error("file should not have been modified when already at current version")
	}
}
