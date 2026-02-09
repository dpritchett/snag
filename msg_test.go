package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStripTrailers_NoTrailers(t *testing.T) {
	lines := []string{"fix bug", "", "body"}
	got, removed := stripTrailers(lines, []string{"bot"})
	if removed != 0 {
		t.Errorf("expected 0 removed, got %d", removed)
	}
	if len(got) != len(lines) {
		t.Errorf("expected %d lines, got %d", len(lines), len(got))
	}
}

func TestStripTrailers_MatchingTrailerRemoved(t *testing.T) {
	lines := []string{"fix bug", "", "Signed-off-by: Bot"}
	got, removed := stripTrailers(lines, []string{"bot"})
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 lines, got %d", len(got))
	}
}

func TestStripTrailers_NonMatchingTrailerKept(t *testing.T) {
	lines := []string{"fix bug", "", "Signed-off-by: Human"}
	got, removed := stripTrailers(lines, []string{"bot"})
	if removed != 0 {
		t.Errorf("expected 0 removed, got %d", removed)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 lines, got %d", len(got))
	}
}

func TestStripTrailers_PartialMatch(t *testing.T) {
	lines := []string{
		"fix bug",
		"",
		"Signed-off-by: Bot",
		"Reviewed-by: Human",
		"Co-authored-by: Bot Helper",
	}
	got, removed := stripTrailers(lines, []string{"bot"})
	if removed != 2 {
		t.Errorf("expected 2 removed, got %d", removed)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 lines, got %d", len(got))
	}
	if got[2] != "Reviewed-by: Human" {
		t.Errorf("expected kept trailer to be 'Reviewed-by: Human', got %q", got[2])
	}
}

func TestRunMsg_MissingBlocklist(t *testing.T) {
	dir := t.TempDir()

	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	os.WriteFile(msgFile, []byte("fix bug\n"), 0644)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"msg", "--blocklist", filepath.Join(dir, "no-such-file"), msgFile})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error for missing blocklist, got: %v", err)
	}

	got, _ := os.ReadFile(msgFile)
	if string(got) != "fix bug\n" {
		t.Errorf("file should be unchanged, got: %q", got)
	}
}

func TestRunMsg_CleanMessage(t *testing.T) {
	dir := t.TempDir()

	blPath := filepath.Join(dir, ".blocklist")
	os.WriteFile(blPath, []byte("hack\n"), 0644)

	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	os.WriteFile(msgFile, []byte("fix bug\n"), 0644)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"msg", "--blocklist", blPath, msgFile})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error for clean message, got: %v", err)
	}
}

func TestRunMsg_TrailerStripped(t *testing.T) {
	dir := t.TempDir()

	blPath := filepath.Join(dir, ".blocklist")
	os.WriteFile(blPath, []byte("bot\n"), 0644)

	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	os.WriteFile(msgFile, []byte("fix bug\n\nSigned-off-by: Bot\n"), 0644)

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"msg", "--blocklist", blPath, msgFile})
	err := rootCmd.Execute()

	w.Close()
	os.Stderr = oldStderr

	if err != nil {
		t.Fatalf("expected nil error after trailer strip, got: %v", err)
	}

	got, _ := os.ReadFile(msgFile)
	if strings.Contains(string(got), "Signed-off-by") {
		t.Errorf("trailer should have been removed, got: %q", got)
	}

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	stderr := string(buf[:n])
	if !strings.Contains(stderr, "removed") {
		t.Errorf("stderr should mention removed trailers, got: %q", stderr)
	}
}

func TestRunMsg_TrailerStrippedThenBodyMatch(t *testing.T) {
	dir := t.TempDir()

	blPath := filepath.Join(dir, ".blocklist")
	os.WriteFile(blPath, []byte("bot\nfixme\n"), 0644)

	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	os.WriteFile(msgFile, []byte("TODO fixme later\n\nSigned-off-by: Bot\n"), 0644)

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"msg", "--blocklist", blPath, msgFile})
	err := rootCmd.Execute()

	w.Close()
	os.Stderr = oldStderr

	if err == nil {
		t.Fatal("expected non-nil error for body match after trailer strip")
	}
	if !strings.Contains(err.Error(), "fixme") {
		t.Errorf("error should mention matched pattern 'fixme', got: %v", err)
	}

	// Trailer should have been stripped from the file on disk
	got, _ := os.ReadFile(msgFile)
	if strings.Contains(string(got), "Signed-off-by") {
		t.Errorf("trailer should have been removed from file, got: %q", got)
	}

	buf := make([]byte, 4096)
	n, _ := r.Read(buf)
	stderr := string(buf[:n])
	if !strings.Contains(stderr, "removed") {
		t.Errorf("stderr should mention removed trailers, got: %q", stderr)
	}
	if !strings.Contains(stderr, `snag: match "fixme"`) {
		t.Errorf("stderr should contain match message, got: %q", stderr)
	}
	if !strings.Contains(stderr, "git commit -eF") {
		t.Errorf("stderr should contain recovery hint, got: %q", stderr)
	}
}

func TestRunMsg_BodyMatch(t *testing.T) {
	dir := t.TempDir()

	blPath := filepath.Join(dir, ".blocklist")
	os.WriteFile(blPath, []byte("todo\n"), 0644)

	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	os.WriteFile(msgFile, []byte("TODO fix this later\n"), 0644)

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"msg", "--blocklist", blPath, msgFile})
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
	if !strings.Contains(stderr, "git commit -eF") {
		t.Errorf("stderr should contain recovery hint, got: %q", stderr)
	}
}
