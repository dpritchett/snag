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
	rootCmd.SetArgs([]string{"install"})
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
	rootCmd.SetArgs([]string{"install"})
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
	rootCmd.SetArgs([]string{"install"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	content := string(data)
	if !strings.Contains(content, "github.com/dpritchett/snag") {
		t.Error("expected snag remote in lefthook.yml")
	}
	if !strings.Contains(content, "recipes/lefthook-snag-filter.yml") {
		t.Error("expected snag-filter recipe in lefthook.yml")
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
      - recipes/lefthook-snag-filter.yml
`
	os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte(initial), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"install"})
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
    ref: ` + versionRef() + `
    configs:
      - recipes/lefthook-snag-filter.yml

# Hook stubs — lefthook needs these to install hooks for remote recipe types.
commit-msg:
post-checkout:
pre-push:
pre-rebase:
prepare-commit-msg:
`
	os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte(initial), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"install"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	if string(data) != initial {
		t.Errorf("file should not have been modified when already at current version\ngot:\n%s", string(data))
	}
}

func TestInstallHooks_DetectsExistingInLocal(t *testing.T) {
	dir := t.TempDir()
	// Shared config exists but has no snag remote.
	os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte("pre-commit:\n  commands:\n    lint:\n      run: echo lint\n"), 0644)
	// Local config has an old snag remote.
	localContent := `remotes:
  - git_url: https://github.com/dpritchett/snag.git
    ref: v0.1.0
    configs:
      - recipes/lefthook-snag-filter.yml
`
	os.WriteFile(filepath.Join(dir, "lefthook-local.yml"), []byte(localContent), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"install"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Local config should be updated.
	data, _ := os.ReadFile(filepath.Join(dir, "lefthook-local.yml"))
	content := string(data)
	if strings.Contains(content, "v0.1.0") {
		t.Error("old ref v0.1.0 should have been replaced in local config")
	}
	if !strings.Contains(content, Version) {
		t.Errorf("expected current version %s in local config", Version)
	}

	// Shared config should NOT have snag remote added.
	sharedData, _ := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	if strings.Contains(string(sharedData), "github.com/dpritchett/snag") {
		t.Error("shared config should not have been modified when snag was only in local")
	}
}

func TestInstallHooks_DetectsExistingInBoth(t *testing.T) {
	dir := t.TempDir()
	sharedContent := `pre-commit:
  commands:
    lint:
      run: echo lint
remotes:
  - git_url: https://github.com/dpritchett/snag.git
    ref: v0.1.0
    configs:
      - recipes/lefthook-snag-filter.yml
`
	localContent := `remotes:
  - git_url: https://github.com/dpritchett/snag.git
    ref: v0.2.0
    configs:
      - recipes/lefthook-snag-filter.yml
`
	os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte(sharedContent), 0644)
	os.WriteFile(filepath.Join(dir, "lefthook-local.yml"), []byte(localContent), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"install"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Both should be updated.
	sharedData, _ := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	if !strings.Contains(string(sharedData), Version) {
		t.Errorf("expected current version %s in shared config", Version)
	}
	if strings.Contains(string(sharedData), "v0.1.0") {
		t.Error("old ref should have been replaced in shared config")
	}

	localData, _ := os.ReadFile(filepath.Join(dir, "lefthook-local.yml"))
	if !strings.Contains(string(localData), Version) {
		t.Errorf("expected current version %s in local config", Version)
	}
	if strings.Contains(string(localData), "v0.2.0") {
		t.Error("old ref should have been replaced in local config")
	}
}

func TestInstallHooks_LocalFlag(t *testing.T) {
	dir := t.TempDir()
	// Shared config exists.
	os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte("pre-commit:\n  commands:\n    lint:\n      run: echo lint\n"), 0644)
	// Local config exists with some content but no snag.
	os.WriteFile(filepath.Join(dir, "lefthook-local.yml"), []byte("pre-push:\n  commands:\n    test:\n      run: echo test\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"install", "--local"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Local config should have snag remote.
	data, _ := os.ReadFile(filepath.Join(dir, "lefthook-local.yml"))
	content := string(data)
	if !strings.Contains(content, "github.com/dpritchett/snag") {
		t.Error("expected snag remote in local config")
	}
	// Original local content preserved.
	if !strings.Contains(content, "echo test") {
		t.Error("original local content was lost")
	}

	// Shared config should NOT be modified.
	sharedData, _ := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	if strings.Contains(string(sharedData), "github.com/dpritchett/snag") {
		t.Error("shared config should not have been modified with --local flag")
	}
}

func TestInstallHooks_SharedFlag(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte("pre-commit:\n  commands:\n    lint:\n      run: echo lint\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"install", "--shared"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	content := string(data)
	if !strings.Contains(content, "github.com/dpritchett/snag") {
		t.Error("expected snag remote in shared config")
	}
}

func TestInstallHooks_LocalFlagCreatesFile(t *testing.T) {
	dir := t.TempDir()
	// Shared config exists, but no local config.
	os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte("pre-commit:\n  commands:\n    lint:\n      run: echo lint\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"install", "--local"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// lefthook-local.yml should have been created.
	data, err := os.ReadFile(filepath.Join(dir, "lefthook-local.yml"))
	if err != nil {
		t.Fatalf("expected lefthook-local.yml to be created: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "github.com/dpritchett/snag") {
		t.Error("expected snag remote in newly created local config")
	}
	// Should not have a leading newline (it's a fresh file).
	if strings.HasPrefix(content, "\n") {
		t.Error("newly created local config should not start with a blank line")
	}
}

func TestInstallHooks_NonTTYDefaultsToShared(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte("pre-commit:\n  commands:\n    lint:\n      run: echo lint\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	// Override isTTY to simulate non-TTY.
	origIsTTY := isTTY
	isTTY = func() bool { return false }
	defer func() { isTTY = origIsTTY }()

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"install"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have written to shared config.
	data, _ := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	if !strings.Contains(string(data), "github.com/dpritchett/snag") {
		t.Error("expected snag remote in shared config when non-TTY")
	}

	// Local config should NOT exist.
	if _, err := os.Stat(filepath.Join(dir, "lefthook-local.yml")); err == nil {
		t.Error("local config should not have been created in non-TTY mode")
	}
}

func TestInstallHooks_DryRunDoesNotWrite(t *testing.T) {
	dir := t.TempDir()
	initial := "pre-commit:\n  commands:\n    lint:\n      run: echo lint\n"
	os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte(initial), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"install", "--dry-run"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File should be unchanged.
	data, _ := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	if string(data) != initial {
		t.Error("--dry-run should not modify the file")
	}
}

func TestInstallHooks_DryRunLocalDoesNotCreate(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte("pre-commit:\n  commands:\n    lint:\n      run: echo lint\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"install", "--local", "--dry-run"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// lefthook-local.yml should NOT have been created.
	if _, err := os.Stat(filepath.Join(dir, "lefthook-local.yml")); err == nil {
		t.Error("--dry-run --local should not create the file")
	}
}

func TestInstallHooks_DryRunUpdate(t *testing.T) {
	dir := t.TempDir()
	initial := `pre-commit:
  commands:
    lint:
      run: echo lint
remotes:
  - git_url: https://github.com/dpritchett/snag.git
    ref: v0.1.0
    configs:
      - recipes/lefthook-snag-filter.yml
`
	os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte(initial), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"install", "--dry-run"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// File should still have the old ref.
	data, _ := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	if !strings.Contains(string(data), "v0.1.0") {
		t.Error("--dry-run should not update the ref")
	}
}

func TestMissingHookStubs_AllMissing(t *testing.T) {
	stubs := missingHookStubs("")
	for _, ht := range snagRecipeHookTypes {
		if !strings.Contains(stubs, ht+":") {
			t.Errorf("expected stub for %s in output", ht)
		}
	}
}

func TestMissingHookStubs_SkipsExisting(t *testing.T) {
	content := "pre-commit:\n  commands:\n    lint:\n      run: echo lint\ncommit-msg:\n"
	stubs := missingHookStubs(content)
	if strings.Contains(stubs, "\npre-commit:\n") {
		t.Error("should not include stub for pre-commit (already defined)")
	}
	if strings.Contains(stubs, "\ncommit-msg:\n") {
		t.Error("should not include stub for commit-msg (already defined)")
	}
	if !strings.Contains(stubs, "pre-push:") {
		t.Error("expected stub for pre-push")
	}
}

func TestMissingHookStubs_NoneNeeded(t *testing.T) {
	// All hook types present.
	content := "commit-msg:\npost-checkout:\npre-commit:\npre-push:\npre-rebase:\nprepare-commit-msg:\n"
	stubs := missingHookStubs(content)
	if stubs != "" {
		t.Errorf("expected no stubs when all types present, got: %q", stubs)
	}
}

func TestInstallHooks_AddsStubsOnFreshInstall(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte("pre-commit:\n  commands:\n    lint:\n      run: echo lint\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"install"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	content := string(data)
	for _, ht := range []string{"commit-msg:", "pre-push:", "post-checkout:", "pre-rebase:", "prepare-commit-msg:"} {
		if !strings.Contains(content, ht) {
			t.Errorf("expected hook stub %s in config", ht)
		}
	}
	// pre-commit was already there, should not appear in stubs section.
	stubSection := content[strings.Index(content, "# Hook stubs"):]
	if strings.Contains(stubSection, "pre-commit:") {
		t.Error("should not add stub for pre-commit when already defined")
	}
}

func TestInstallHooks_AddsStubsOnRefUpdate(t *testing.T) {
	dir := t.TempDir()
	initial := `pre-commit:
  commands:
    lint:
      run: echo lint
remotes:
  - git_url: https://github.com/dpritchett/snag.git
    ref: v0.1.0
    configs:
      - recipes/lefthook-snag-filter.yml
`
	os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte(initial), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"install"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "lefthook.yml"))
	content := string(data)
	if !strings.Contains(content, "commit-msg:") {
		t.Error("expected commit-msg stub after ref update")
	}
	if !strings.Contains(content, "pre-push:") {
		t.Error("expected pre-push stub after ref update")
	}
}

func TestInstallHooks_LocalCreatesFileWithStubs(t *testing.T) {
	dir := t.TempDir()
	os.WriteFile(filepath.Join(dir, "lefthook.yml"), []byte("pre-commit:\n  commands:\n    lint:\n      run: echo lint\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"install", "--local"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	data, _ := os.ReadFile(filepath.Join(dir, "lefthook-local.yml"))
	content := string(data)
	// New local file should have all hook stubs (including pre-commit since
	// the local file itself doesn't define it).
	for _, ht := range snagRecipeHookTypes {
		if !strings.Contains(content, ht+":") {
			t.Errorf("expected hook stub %s in newly created local config", ht)
		}
	}
}
