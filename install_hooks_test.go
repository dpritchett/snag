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
		t.Fatal("expected error when no lefthook config exists")
	}
	if !strings.Contains(err.Error(), "no lefthook config found") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestInstallHooks_FindsYamlExtension(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "lefthook.yaml"), []byte("pre-commit:\n  commands:\n    lint:\n      run: echo lint\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"install-hooks"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "lefthook.yaml"))
	content := string(data)
	if !strings.Contains(content, "github.com/dpritchett/snag") {
		t.Error("expected snag remote in lefthook.yaml")
	}
}

func TestInstallHooks_AddsRemote(t *testing.T) {
	dir := t.TempDir()
	initial := "# My hooks\npre-commit:\n  commands:\n    lint:\n      run: echo lint\n"
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
	if !strings.Contains(content, "github.com/dpritchett/snag") {
		t.Error("expected snag remote in lefthook.yml")
	}
	if !strings.Contains(content, "recipes/lefthook-blocklist.yml") {
		t.Error("expected blocklist recipe in lefthook.yml")
	}
	// Original content must be preserved verbatim.
	if !strings.HasPrefix(content, initial) {
		t.Error("original file content was mangled")
	}
	// Comment must survive.
	if !strings.Contains(content, "# My hooks") {
		t.Error("comment was stripped from file")
	}
}

func TestInstallHooks_UpdatesRef(t *testing.T) {
	dir := t.TempDir()
	initial := `# Important hooks
pre-commit:
  parallel: true
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
		t.Errorf("expected current version %s in lefthook.yml", Version)
	}
	// Everything except the ref line must be preserved.
	if !strings.Contains(content, "# Important hooks") {
		t.Error("comment was stripped during ref update")
	}
	if !strings.Contains(content, "parallel: true") {
		t.Error("parallel key was lost during ref update")
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

	data, _ := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	if string(data) != initial {
		t.Error("file should not have been modified when already at current version")
	}
}
