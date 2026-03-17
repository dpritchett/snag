package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestStripMatchingTrailers_NoTrailers(t *testing.T) {
	lines := []string{"fix bug", "", "body"}
	got, removed := stripMatchingTrailers(lines, []string{"bot"})
	if removed != 0 {
		t.Errorf("expected 0 removed, got %d", removed)
	}
	if len(got) != len(lines) {
		t.Errorf("expected %d lines, got %d", len(lines), len(got))
	}
}

func TestStripMatchingTrailers_MatchingTrailerRemoved(t *testing.T) {
	lines := []string{"fix bug", "", "Signed-off-by: Bot"}
	got, removed := stripMatchingTrailers(lines, []string{"bot"})
	if removed != 1 {
		t.Errorf("expected 1 removed, got %d", removed)
	}
	if len(got) != 2 {
		t.Errorf("expected 2 lines, got %d", len(got))
	}
}

func TestStripMatchingTrailers_NonMatchingTrailerKept(t *testing.T) {
	lines := []string{"fix bug", "", "Signed-off-by: Human"}
	got, removed := stripMatchingTrailers(lines, []string{"bot"})
	if removed != 0 {
		t.Errorf("expected 0 removed, got %d", removed)
	}
	if len(got) != 3 {
		t.Errorf("expected 3 lines, got %d", len(got))
	}
}

func TestStripMatchingTrailers_PartialMatch(t *testing.T) {
	lines := []string{
		"fix bug",
		"",
		"Signed-off-by: Bot",
		"Reviewed-by: Human",
		"Co-authored-by: Bot Helper",
	}
	got, removed := stripMatchingTrailers(lines, []string{"bot"})
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

func TestRunMsg_CleanMessage(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\nmsg = [\"hack\"]\n"), 0644)

	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	os.WriteFile(msgFile, []byte("fix bug\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"check", "msg", msgFile})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected nil error for clean message, got: %v", err)
	}
}

func TestRunMsg_TrailerStripped(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\nmsg = [\"bot\"]\n"), 0644)

	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	os.WriteFile(msgFile, []byte("fix bug\n\nSigned-off-by: Bot\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"check", "msg", msgFile})
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

	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\nmsg = [\"bot\", \"fixme\"]\n"), 0644)

	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	os.WriteFile(msgFile, []byte("TODO fixme later\n\nSigned-off-by: Bot\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"check", "msg", msgFile})
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

func TestMsgContentLines(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  int
	}{
		{"empty", nil, 0},
		{"blank lines only", []string{"", "  ", "\t"}, 0},
		{"comments only", []string{"# comment", "# another"}, 0},
		{"mixed", []string{"subject", "", "# comment", "body line"}, 2},
		{"all content", []string{"one", "two", "three"}, 3},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := msgContentLines(tc.lines)
			if len(got) != tc.want {
				t.Errorf("msgContentLines() returned %d lines, want %d", len(got), tc.want)
			}
		})
	}
}

func TestRunMsg_MaxLenExceeded(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\nmsg_max_len = 20\n"), 0644)

	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	os.WriteFile(msgFile, []byte("this first line is way too long for the limit\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"check", "msg", msgFile})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for first line exceeding max_len")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error should mention 'exceeds', got: %v", err)
	}
}

func TestRunMsg_MaxLenOK(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\nmsg_max_len = 100\n"), 0644)

	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	os.WriteFile(msgFile, []byte("short subject\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"check", "msg", msgFile})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestRunMsg_MaxLenSkipsComments(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\nmsg_max_len = 20\n"), 0644)

	// First real line is "ok" (2 chars), the long line is a comment.
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	os.WriteFile(msgFile, []byte("# this comment is very long and should be ignored by the check\nok\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"check", "msg", msgFile})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error (comment skipped), got: %v", err)
	}
}

func TestRunMsg_MaxLinesExceeded(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\nmsg_max_lines = 3\n"), 0644)

	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	os.WriteFile(msgFile, []byte("line one\nline two\nline three\nline four\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"check", "msg", msgFile})
	err := rootCmd.Execute()
	if err == nil {
		t.Fatal("expected error for exceeding max_lines")
	}
	if !strings.Contains(err.Error(), "exceeds") {
		t.Errorf("error should mention 'exceeds', got: %v", err)
	}
}

func TestRunMsg_MaxLinesOK(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\nmsg_max_lines = 3\n"), 0644)

	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	os.WriteFile(msgFile, []byte("subject\n\nbody line\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"check", "msg", msgFile})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
}

func TestRunMsg_MaxLinesSkipsBlanksAndComments(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\nmsg_max_lines = 2\n"), 0644)

	// 2 content lines + blanks + comments = should pass
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	os.WriteFile(msgFile, []byte("subject\n\n# comment\n\nbody\n# another comment\n\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"check", "msg", msgFile})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error (blanks/comments don't count), got: %v", err)
	}
}

func TestRunMsg_ZeroMeansUnlimited(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\nmsg_max_len = 0\nmsg_max_lines = 0\n"), 0644)

	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	long := strings.Repeat("x", 500) + "\n" + strings.Repeat("line\n", 50)
	os.WriteFile(msgFile, []byte(long), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"check", "msg", msgFile})
	err := rootCmd.Execute()
	if err != nil {
		t.Fatalf("expected no error with zero limits, got: %v", err)
	}
}

func TestRunMsg_BodyMatch(t *testing.T) {
	dir := t.TempDir()

	os.WriteFile(filepath.Join(dir, "snag.toml"),
		[]byte("[block]\nmsg = [\"todo\"]\n"), 0644)

	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	os.WriteFile(msgFile, []byte("TODO fix this later\n"), 0644)

	oldDir, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(oldDir)

	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	rootCmd := buildRootCmd()
	rootCmd.SetArgs([]string{"check", "msg", msgFile})
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
