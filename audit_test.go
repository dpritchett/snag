package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAudit_CleanHistory(t *testing.T) {
	dir := initGitRepo(t)
	initialCommit(t, dir)
	commitFile(t, dir, "a.txt", "hello\n", "add greeting")

	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\ndiff = [\"secret\"]\nmsg = [\"secret\"]\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"audit"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error for clean history, got: %v", err)
	}
}

func TestAudit_MessageViolation(t *testing.T) {
	dir := initGitRepo(t)
	initialCommit(t, dir)
	commitFile(t, dir, "a.txt", "clean content\n", "fixup! this is bad")

	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\nmsg = [\"fixup!\"]\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"audit"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for message violation")
	}
	if !strings.Contains(err.Error(), "violation") {
		t.Errorf("error should mention violation, got: %v", err)
	}
}

func TestAudit_DiffViolation(t *testing.T) {
	dir := initGitRepo(t)
	initialCommit(t, dir)
	commitFile(t, dir, "a.txt", "this is a HACK\n", "add file")

	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\ndiff = [\"hack\"]\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"audit"})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for diff violation")
	}
	if !strings.Contains(err.Error(), "violation") {
		t.Errorf("error should mention violation, got: %v", err)
	}
}

func TestAudit_BothMsgAndDiff(t *testing.T) {
	dir := initGitRepo(t)
	initialCommit(t, dir)
	commitFile(t, dir, "a.txt", "contains HACK here\n", "fixup! bad commit")

	// Use snag.toml so we get per-hook separation.
	tomlPath := filepath.Join(dir, "snag.toml")
	os.WriteFile(tomlPath, []byte("[block]\ndiff = [\"hack\"]\nmsg = [\"fixup!\"]\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"audit"})
	err := rootCmd.Execute()

	w.Close()
	os.Stderr = oldStderr

	if err == nil {
		t.Fatal("expected error when both msg and diff violations exist")
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	stderr := string(buf[:n])
	if !strings.Contains(stderr, "2 violations") {
		t.Errorf("should report 2 violations, got: %q", stderr)
	}
}

func TestAudit_LimitFlag(t *testing.T) {
	dir := initGitRepo(t)
	initialCommit(t, dir)
	// Create 5 commits, the first one has a violation.
	commitFile(t, dir, "a.txt", "this is a HACK\n", "add file a")
	commitFile(t, dir, "b.txt", "clean\n", "add file b")
	commitFile(t, dir, "c.txt", "clean\n", "add file c")
	commitFile(t, dir, "d.txt", "clean\n", "add file d")
	commitFile(t, dir, "e.txt", "clean\n", "add file e")

	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\ndiff = [\"hack\"]\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	// Limit to 3 — should only scan the 3 most recent, all clean.
	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"audit", "--limit", "3"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error with --limit 3 (violation is older), got: %v", err)
	}

	// Limit to 0 (unlimited) — should find the violation.
	rootCmd2 := buildRootCmd()
	rootCmd2.SetArgs([]string{"audit", "--limit", "0"})
	err = rootCmd2.Execute()
	if err == nil {
		t.Fatal("expected error with --limit 0 (scan all, violation exists)")
	}
}

func TestAudit_EmptyRepo(t *testing.T) {
	dir := initGitRepo(t)
	// No commits at all — just git init.

	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\ndiff = [\"hack\"]\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"audit"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error for empty repo, got: %v", err)
	}
}

func TestAudit_PerHookSeparation(t *testing.T) {
	dir := initGitRepo(t)
	initialCommit(t, dir)
	// Commit with "hack" in the message but clean diff.
	commitFile(t, dir, "a.txt", "clean content\n", "hack around with stuff")

	// snag.toml: "hack" only in diff patterns, not msg.
	tomlPath := filepath.Join(dir, "snag.toml")
	os.WriteFile(tomlPath, []byte("[block]\ndiff = [\"hack\"]\nmsg = [\"fixup!\"]\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"audit"})
	err := rootCmd.Execute()
	// "hack" is a diff pattern — it should NOT match in the message check.
	// The diff itself is clean ("clean content"), so no violation expected.
	if err != nil {
		t.Fatalf("expected no error (diff pattern shouldn't match in msg), got: %v", err)
	}
}

func TestAudit_QuietMode(t *testing.T) {
	dir := initGitRepo(t)
	initialCommit(t, dir)
	commitFile(t, dir, "a.txt", "this is a HACK\n", "add file")

	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\ndiff = [\"hack\"]\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"audit", "-q"})
	err := rootCmd.Execute()

	w.Close()
	os.Stderr = oldStderr

	if err == nil {
		t.Fatal("expected error for diff violation in quiet mode")
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	stderr := string(buf[:n])
	// Quiet mode: should still show summary but not per-commit details.
	if strings.Contains(stderr, "scanning") {
		t.Errorf("quiet mode should suppress scanning message, got: %q", stderr)
	}
}

func TestAudit_ExplicitRange(t *testing.T) {
	dir := initGitRepo(t)
	initialCommit(t, dir)
	commitFile(t, dir, "a.txt", "this is a HACK\n", "add file a")
	commitFile(t, dir, "b.txt", "clean\n", "add file b")

	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\ndiff = [\"hack\"]\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	// Range covering only the last commit (which is clean).
	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"audit", "HEAD~1..HEAD"})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error for range with only clean commit, got: %v", err)
	}

	// Range covering both commits (violation in older one).
	rootCmd2 := buildRootCmd()
	rootCmd2.SetArgs([]string{"audit", "HEAD~2..HEAD"})
	err = rootCmd2.Execute()
	if err == nil {
		t.Fatal("expected error for range including violation commit")
	}
}
