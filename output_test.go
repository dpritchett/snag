package main

import (
	"os"
	"strings"
	"testing"
)

func TestErrorf_ContainsSnagPrefix(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	errorf("match %q in staged diff", "secret")

	w.Close()
	os.Stderr = old

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	got := string(buf[:n])

	if !strings.Contains(got, "snag:") {
		t.Errorf("expected snag: prefix, got: %q", got)
	}
	if !strings.Contains(got, `match "secret" in staged diff`) {
		t.Errorf("expected formatted message, got: %q", got)
	}
}

func TestWarnf_ContainsSnagPrefix(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	warnf("removed %d trailer line(s)", 2)

	w.Close()
	os.Stderr = old

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	got := string(buf[:n])

	if !strings.Contains(got, "snag:") {
		t.Errorf("expected snag: prefix, got: %q", got)
	}
	if !strings.Contains(got, "removed 2 trailer line(s)") {
		t.Errorf("expected formatted message, got: %q", got)
	}
}

func TestInfof_ContainsSnagPrefix(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	infof("%d patterns checked against %d commits", 3, 5)

	w.Close()
	os.Stderr = old

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	got := string(buf[:n])

	if !strings.Contains(got, "snag:") {
		t.Errorf("expected snag: prefix, got: %q", got)
	}
	if !strings.Contains(got, "3 patterns checked against 5 commits") {
		t.Errorf("expected formatted message, got: %q", got)
	}
}

func TestHintf_ContainsMessage(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	hintf("to recover: git commit -eF .git/COMMIT_EDITMSG")

	w.Close()
	os.Stderr = old

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	got := string(buf[:n])

	if !strings.Contains(got, "to recover: git commit -eF .git/COMMIT_EDITMSG") {
		t.Errorf("expected hint message, got: %q", got)
	}
}

func TestOutputNoANSI_WhenPiped(t *testing.T) {
	// stderr is a pipe in tests, so lipgloss should emit no ANSI codes
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	errorf("test message")

	w.Close()
	os.Stderr = old

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	got := string(buf[:n])

	if strings.Contains(got, "\x1b[") {
		t.Errorf("expected no ANSI escape codes in pipe output, got: %q", got)
	}
}

func TestBell_NoOutputWhenPiped(t *testing.T) {
	old := os.Stderr
	r, w, _ := os.Pipe()
	os.Stderr = w

	bell()

	w.Close()
	os.Stderr = old

	buf := make([]byte, 1024)
	n, _ := r.Read(buf)
	got := string(buf[:n])

	if strings.Contains(got, "\a") {
		t.Errorf("expected no bell in pipe output, got: %q", got)
	}
}
