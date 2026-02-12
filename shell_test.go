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

func TestShellBash_OutputContainsHook(t *testing.T) {
	cmd := buildShellCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"bash"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "__snag_check()") {
		t.Error("output should contain the __snag_check function")
	}
	if !strings.Contains(out, "PROMPT_COMMAND") {
		t.Error("output should register via PROMPT_COMMAND")
	}
	if !strings.Contains(out, "__snag_last_pwd") {
		t.Error("output should contain PWD-change guard")
	}
	if !strings.Contains(out, `\033[`) {
		t.Error("output should contain ANSI color escapes")
	}
	if !strings.Contains(out, "SNAG_QUIET") {
		t.Error("output should reference SNAG_QUIET")
	}
	if !strings.Contains(out, ".git") {
		t.Error("output should check for .git directory")
	}
	if !strings.Contains(out, "lefthook") {
		t.Error("output should check for lefthook")
	}
	if !strings.Contains(out, "snag config") {
		t.Error("output should check snag config")
	}
}

func TestShellZsh_OutputContainsHook(t *testing.T) {
	cmd := buildShellCmd()
	var buf bytes.Buffer
	cmd.SetOut(&buf)
	cmd.SetArgs([]string{"zsh"})

	if err := cmd.Execute(); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "__snag_check()") {
		t.Error("output should contain the __snag_check function")
	}
	if !strings.Contains(out, "chpwd_functions") {
		t.Error("output should register via chpwd_functions")
	}
	if !strings.Contains(out, `\033[`) {
		t.Error("output should contain ANSI color escapes")
	}
	if !strings.Contains(out, "SNAG_QUIET") {
		t.Error("output should reference SNAG_QUIET")
	}
	if !strings.Contains(out, ".git") {
		t.Error("output should check for .git directory")
	}
	if !strings.Contains(out, "lefthook") {
		t.Error("output should check for lefthook")
	}
	if !strings.Contains(out, "snag config") {
		t.Error("output should check snag config")
	}
}

func TestShellHook_AllStagesNonEmpty(t *testing.T) {
	shells := []shellHook{fishShell{}, bashShell{}, zshShell{}}
	for _, h := range shells {
		t.Run(h.name(), func(t *testing.T) {
			stages := map[string]string{
				"name":                h.name(),
				"preamble":            h.preamble(),
				"checkGitDir":         h.checkGitDir(),
				"checkHooksInstalled": h.checkHooksInstalled(),
				"checkSnagConfig":     h.checkSnagConfig(),
				"checkQuiet":          h.checkQuiet(),
				"getRepoName":         h.getRepoName(),
				"warn":                h.warn(),
				"bell":                h.bell(),
				"postamble":           h.postamble(),
			}
			for stage, val := range stages {
				if strings.TrimSpace(val) == "" {
					t.Errorf("%s.%s() returned empty string", h.name(), stage)
				}
			}
		})
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
	if !strings.Contains(err.Error(), "supported: bash, fish, zsh") {
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
