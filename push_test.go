package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// commitFile creates a file, stages it, and commits it in one step.
func commitFile(t *testing.T, dir, name, content, message string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{
		{"add", name},
		{"commit", "-m", message},
	} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func TestRunPush_CleanPush(t *testing.T) {
	dir := initGitRepo(t)
	initialCommit(t, dir)

	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\ndiff = [\"secret\"]\nmsg = [\"secret\"]\n"), 0644)

	commitFile(t, dir, "a.txt", "hello world\n", "add greeting")

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"check", "push"})
	err := rootCmd.Execute()

	w.Close()
	os.Stderr = oldStderr

	if err != nil {
		t.Fatalf("expected nil error for clean push, got: %v", err)
	}

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	stderr := string(buf[:n])
	if !strings.Contains(stderr, "patterns checked against") {
		t.Errorf("stderr should contain summary line, got: %q", stderr)
	}
}

func TestRunPush_MessageMatch(t *testing.T) {
	dir := initGitRepo(t)
	initialCommit(t, dir)

	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\ndiff = [\"todo\"]\nmsg = [\"todo\"]\n"), 0644)

	commitFile(t, dir, "a.txt", "clean content\n", "TODO fix this later")

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"check", "push"})
	err := rootCmd.Execute()

	w.Close()
	os.Stderr = oldStderr

	if err == nil {
		t.Fatal("expected non-nil error for message match")
	}
	if !strings.Contains(err.Error(), "todo") {
		t.Errorf("error should mention matched pattern, got: %v", err)
	}
	if !strings.Contains(err.Error(), "message") {
		t.Errorf("error should mention message context, got: %v", err)
	}

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	stderr := string(buf[:n])
	if !strings.Contains(stderr, `snag: match "todo" in message of`) {
		t.Errorf("stderr should contain match message, got: %q", stderr)
	}
}

func TestRunPush_DiffMatchInSecondCommit(t *testing.T) {
	dir := initGitRepo(t)
	initialCommit(t, dir)

	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\ndiff = [\"hack\"]\nmsg = [\"hack\"]\n"), 0644)

	// Commit 1: clean content, clean message
	commitFile(t, dir, "a.txt", "hello world\n", "add greeting")

	// Commit 2: content containing blocked pattern
	commitFile(t, dir, "b.txt", "this is a hack\n", "add file b")

	// Capture the short SHA of the second commit
	shaCmd := exec.Command("git", "rev-parse", "--short=7", "HEAD")
	shaCmd.Dir = dir
	shaOut, err := shaCmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git rev-parse: %v\n%s", err, shaOut)
	}
	shortSHA := strings.TrimSpace(string(shaOut))

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"check", "push"})
	err = rootCmd.Execute()

	w.Close()
	os.Stderr = oldStderr

	if err == nil {
		t.Fatal("expected non-nil error for diff match in second commit")
	}
	if !strings.Contains(err.Error(), "hack") {
		t.Errorf("error should mention matched pattern, got: %v", err)
	}
	if !strings.Contains(err.Error(), "diff") {
		t.Errorf("error should mention diff context, got: %v", err)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	stderr := string(buf[:n])
	if !strings.Contains(stderr, `snag: match "hack" in diff of`) {
		t.Errorf("stderr should contain match message, got: %q", stderr)
	}
	if !strings.Contains(stderr, shortSHA) {
		t.Errorf("stderr should contain SHA of second commit (%s), got: %q", shortSHA, stderr)
	}
}

func TestRunPush_DiffMatch(t *testing.T) {
	dir := initGitRepo(t)
	initialCommit(t, dir)

	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\ndiff = [\"hack\"]\nmsg = [\"hack\"]\n"), 0644)

	commitFile(t, dir, "a.txt", "this is a hack\n", "add file")

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"check", "push"})
	err := rootCmd.Execute()

	w.Close()
	os.Stderr = oldStderr

	if err == nil {
		t.Fatal("expected non-nil error for diff match")
	}
	if !strings.Contains(err.Error(), "hack") {
		t.Errorf("error should mention matched pattern, got: %v", err)
	}
	if !strings.Contains(err.Error(), "diff") {
		t.Errorf("error should mention diff context, got: %v", err)
	}

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	stderr := string(buf[:n])
	if !strings.Contains(stderr, `snag: match "hack" in diff of`) {
		t.Errorf("stderr should contain match message, got: %q", stderr)
	}
}

// initBareRemote creates a bare clone of dir and adds it as "origin".
func initBareRemote(t *testing.T, dir string) string {
	t.Helper()
	bare := t.TempDir()
	cmd := exec.Command("git", "clone", "--bare", dir, bare)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git clone --bare: %v\n%s", err, out)
	}
	add := exec.Command("git", "remote", "add", "origin", bare)
	add.Dir = dir
	if out, err := add.CombinedOutput(); err != nil {
		t.Fatalf("git remote add: %v\n%s", err, out)
	}
	// Fetch so remote tracking refs exist locally.
	fetch := exec.Command("git", "fetch", "origin")
	fetch.Dir = dir
	if out, err := fetch.CombinedOutput(); err != nil {
		t.Fatalf("git fetch: %v\n%s", err, out)
	}
	return bare
}

func TestRunPush_SkipsCommitsAlreadyOnRemote(t *testing.T) {
	dir := initGitRepo(t)
	initialCommit(t, dir)

	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\ndiff = [\"hack\"]\nmsg = [\"hack\"]\n"), 0644)

	// Create a commit with a violation — this will be "already pushed".
	commitFile(t, dir, "a.txt", "this is a hack\n", "add file with violation")

	// Set up a bare remote that includes the violation commit.
	initBareRemote(t, dir)

	// Now make a clean new commit that hasn't been pushed.
	commitFile(t, dir, "b.txt", "clean content\n", "add clean file")

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"check", "push"})
	err := rootCmd.Execute()

	w.Close()
	os.Stderr = oldStderr

	// The violation commit is already on the remote, so only the new
	// clean commit should be scanned — no error expected.
	if err != nil {
		t.Fatalf("expected no error (violation is on remote), got: %v", err)
	}
}

func TestRunPush_CatchesNewViolationWithRemote(t *testing.T) {
	dir := initGitRepo(t)
	initialCommit(t, dir)

	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\ndiff = [\"hack\"]\nmsg = [\"hack\"]\n"), 0644)

	// Clean commit that gets pushed to remote.
	commitFile(t, dir, "a.txt", "clean content\n", "add clean file")

	initBareRemote(t, dir)

	// New commit with a violation — not yet on remote.
	commitFile(t, dir, "b.txt", "this is a hack\n", "add violation")

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	oldStderr := os.Stderr
	_, w, _ := os.Pipe()
	os.Stderr = w

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"check", "push"})
	err := rootCmd.Execute()

	w.Close()
	os.Stderr = oldStderr

	if err == nil {
		t.Fatal("expected error for new violation not yet on remote")
	}
	if !strings.Contains(err.Error(), "hack") {
		t.Errorf("error should mention matched pattern, got: %v", err)
	}
}
