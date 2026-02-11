package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestShellFish_OutputContainsHook(t *testing.T) {
	cmd := buildShellCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"fish"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "function __snag_check") {
		t.Error("output should contain the __snag_check function")
	}
	if !strings.Contains(out, "--on-variable PWD") {
		t.Error("output should contain the PWD hook trigger")
	}
	if !strings.Contains(out, "set_color") {
		t.Error("output should contain colorized warning")
	}
	if !strings.Contains(out, "SNAG_QUIET") {
		t.Error("output should reference SNAG_QUIET")
	}
}

func TestShellFish_UnknownShell(t *testing.T) {
	cmd := buildShellCmd()
	cmd.SetArgs([]string{"nushell"})
	cmd.SilenceUsage = true

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error for unsupported shell")
	}
	if !strings.Contains(err.Error(), "unsupported shell") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestShellCmd_NoArgs(t *testing.T) {
	cmd := buildShellCmd()
	cmd.SetArgs([]string{})
	cmd.SilenceUsage = true

	err := cmd.Execute()
	if err == nil {
		t.Fatal("expected error when no shell argument given")
	}
}
