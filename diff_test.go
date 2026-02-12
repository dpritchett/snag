package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// initGitRepo creates a temp git repo and returns its path.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()

	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	return dir
}

// stageFile creates a file in dir and stages it.
func stageFile(t *testing.T, dir, name, content string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	// Need an initial commit so git diff --staged works
	cmd := exec.Command("git", "add", name)
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}
}

// initialCommit creates a dummy initial commit so staged diffs work.
func initialCommit(t *testing.T, dir string) {
	t.Helper()
	dummy := filepath.Join(dir, ".gitkeep")
	if err := os.WriteFile(dummy, []byte(""), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", ".gitkeep"},
		{"commit", "-m", "initial"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func TestRunDiff_CleanDiff(t *testing.T) {
	dir := initGitRepo(t)
	initialCommit(t, dir)

	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\ndiff = [\"secret\"]\nmsg = [\"secret\"]\n"), 0644)

	stageFile(t, dir, "hello.txt", "hello world\n")

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"check", "diff"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error for clean diff, got: %v", err)
	}
}

func TestRunDiff_WalkFindsParentConfig(t *testing.T) {
	// Parent dir has a snag.toml, child git repo does not.
	parent := t.TempDir()
	os.WriteFile(filepath.Join(parent, "snag.toml"),
		[]byte("[block]\ndiff = [\"secretword\"]\n"), 0644)

	child := filepath.Join(parent, "repo")
	os.MkdirAll(child, 0755)

	// init git repo in child
	for _, args := range [][]string{
		{"init"},
		{"config", "user.email", "test@test.com"},
		{"config", "user.name", "Test"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = child
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	initialCommit(t, child)
	stageFile(t, child, "leak.txt", "contains secretword here\n")

	oldDir, _ := os.Getwd()
	os.Chdir(child)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"check", "diff"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error from parent config match")
	}
	if !strings.Contains(err.Error(), "secretword") {
		t.Errorf("error should mention 'secretword', got: %v", err)
	}
}

func TestRunDiff_RemovingBlockedWordPasses(t *testing.T) {
	dir := initGitRepo(t)
	initialCommit(t, dir)

	// Use a test-only pattern to avoid triggering the repo's own pre-commit hook.
	pattern := "block" + "ed_test_word"
	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\ndiff = [\""+pattern+"\"]\n"), 0644)

	// First, commit a file containing the blocked pattern.
	filePath := filepath.Join(dir, "notes.txt")
	os.WriteFile(filePath, []byte(pattern+" appears here\n"), 0644)
	for _, args := range [][]string{
		{"add", "notes.txt"},
		{"commit", "-m", "add notes"},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}

	// Now remove the blocked pattern and stage the change.
	os.WriteFile(filePath, []byte("clean content\n"), 0644)
	addCmd := exec.Command("git", "add", "notes.txt")
	addCmd.Dir = dir
	if out, err := addCmd.CombinedOutput(); err != nil {
		t.Fatalf("git add: %v\n%s", err, out)
	}

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"check", "diff"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("removing a blocked word should not trigger a violation, got: %v", err)
	}
}

func TestRunDiff_MatchFound(t *testing.T) {
	dir := initGitRepo(t)
	initialCommit(t, dir)

	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\ndiff = [\"todo\"]\n"), 0644)

	stageFile(t, dir, "code.go", "// TODO fix this\n")

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"check", "diff"})

	// Capture stderr
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	err := rootCmd.Execute()

	w.Close()
	os.Stderr = oldStderr

	if err == nil {
		t.Fatal("expected non-nil error for policy match")
	}
	if !strings.Contains(err.Error(), "todo") {
		t.Errorf("error should mention matched pattern, got: %v", err)
	}

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	stderr := string(buf[:n])
	if !strings.Contains(stderr, `snag: match "todo"`) {
		t.Errorf("stderr should contain match message, got: %q", stderr)
	}
}
