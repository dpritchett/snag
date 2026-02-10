package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

// snagHooksInstalled checks whether snag enforcement hooks are wired up,
// either via a lefthook config (remote or inline commands) or via direct .git/hooks/ scripts.
func snagHooksInstalled() bool {
	// Path 1: lefthook config — check for snag remote or inline snag commands
	for _, finder := range []func() (string, error){findLefthookConfig, findLefthookLocalConfig} {
		cfg, err := finder()
		if err != nil || cfg == "" {
			continue
		}
		data, err := os.ReadFile(cfg)
		if err != nil {
			continue
		}
		if ref, _ := findSnagRemote(data); ref != "" {
			return true
		}
		// Also detect inline usage like "run: snag check diff"
		if strings.Contains(string(data), "snag check") {
			return true
		}
	}
	// Path 2: direct .git/hooks/ files containing "snag"
	for _, name := range []string{"pre-commit", "commit-msg", "pre-push"} {
		data, err := os.ReadFile(filepath.Join(".git", "hooks", name))
		if err == nil && strings.Contains(string(data), "snag") {
			return true
		}
	}
	return false
}

func runCheckout(cmd *cobra.Command, args []string) error {
	// If no patterns exist, nothing to protect — skip silently.
	bc, err := resolveBlockConfig(cmd)
	if err != nil {
		return err
	}
	if !bc.HasAnyPatterns() {
		return nil
	}

	if snagHooksInstalled() {
		return nil
	}

	quiet, _ := cmd.Flags().GetBool("quiet")
	if !quiet {
		warnf("this repo has a snag config but snag hooks aren't installed")
		hintf("run: snag install && lefthook install")
	}
	return fmt.Errorf("snag hooks not installed")
}

func testCheckout(cmd *cobra.Command, dir string, patterns []string) bool {
	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	// The temp repo has a .blocklist but no lefthook config or .git/hooks with snag,
	// so runCheckout should return an error (correctly detecting missing hooks).
	err := runCheckout(cmd, nil)
	return err != nil
}
