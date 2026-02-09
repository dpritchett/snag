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

func TestRunPush_MissingBlocklist(t *testing.T) {
	dir := initGitRepo(t)
	initialCommit(t, dir)
	commitFile(t, dir, "a.txt", "hello\n", "add a")

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"push", "--blocklist", filepath.Join(dir, "no-such-file")})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error for missing blocklist, got: %v", err)
	}
}

func TestRunPush_CleanPush(t *testing.T) {
	dir := initGitRepo(t)
	initialCommit(t, dir)

	blPath := filepath.Join(dir, ".blocklist")
	os.WriteFile(blPath, []byte("secret\n"), 0644)

	commitFile(t, dir, "a.txt", "hello world\n", "add greeting")

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"push", "--blocklist", blPath})
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

	blPath := filepath.Join(dir, ".blocklist")
	os.WriteFile(blPath, []byte("todo\n"), 0644)

	commitFile(t, dir, "a.txt", "clean content\n", "TODO fix this later")

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"push", "--blocklist", blPath})
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

func TestRunPush_DiffMatch(t *testing.T) {
	dir := initGitRepo(t)
	initialCommit(t, dir)

	blPath := filepath.Join(dir, ".blocklist")
	os.WriteFile(blPath, []byte("hack\n"), 0644)

	commitFile(t, dir, "a.txt", "this is a hack\n", "add file")

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"push", "--blocklist", blPath})
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
