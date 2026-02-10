package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var cannedPatterns = []string{"todo", "fixme", "password"}

func buildTestCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "test [diff|msg|push]",
		Short:        "Dry-run hooks against a temp repo to verify output",
		SilenceUsage: true,
		Args:         cobra.MaximumNArgs(1),
		RunE:         runTest,
	}
	return cmd
}

func runTest(cmd *cobra.Command, args []string) error {
	// Decide which sub-tests to run.
	which := "all"
	if len(args) == 1 {
		which = args[0]
	}
	valid := map[string]bool{"all": true, "diff": true, "msg": true, "push": true}
	if !valid[which] {
		return fmt.Errorf("unknown test %q (choose diff, msg, or push)", which)
	}

	// Resolve patterns: real blocklist first, canned fallback.
	patterns, err := resolvePatterns(cmd)
	if err != nil {
		return err
	}
	if len(patterns) == 0 {
		patterns = cannedPatterns
		infof("no blocklist found, using demo patterns")
	}

	quiet, _ := cmd.Flags().GetBool("quiet")
	if !quiet {
		infof("testing with patterns: %v", patterns)
	}

	dir, err := os.MkdirTemp("", "snag-test-*")
	if err != nil {
		return fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(dir)

	if err := setupTestRepo(dir); err != nil {
		return fmt.Errorf("setting up temp repo: %w", err)
	}

	// Write a blocklist file in the temp repo so the real resolve logic finds it.
	blFile := filepath.Join(dir, ".blocklist")
	if err := os.WriteFile(blFile, []byte(strings.Join(patterns, "\n")+"\n"), 0644); err != nil {
		return fmt.Errorf("writing temp blocklist: %w", err)
	}

	type subtest struct {
		name string
		fn   func(*cobra.Command, string, []string) bool
	}
	all := []subtest{
		{"diff", testDiff},
		{"msg", testMsg},
		{"push", testPush},
	}

	passed := 0
	total := 0
	for _, st := range all {
		if which != "all" && which != st.name {
			continue
		}
		total++
		if !quiet {
			fmt.Fprintf(os.Stderr, "\n=== %s ===\n", st.name)
		}
		if st.fn(cmd, dir, patterns) {
			passed++
			if !quiet {
				fmt.Fprintln(os.Stderr, infoStyle.Render("PASS:")+" "+st.name+" correctly rejected violation")
			}
		} else {
			if !quiet {
				fmt.Fprintln(os.Stderr, errorStyle.Render("FAIL:")+" "+st.name+" did not detect violation")
			}
		}
	}

	if !quiet {
		fmt.Fprintf(os.Stderr, "\nsnag: %d/%d checks passed\n", passed, total)
	}
	if passed < total {
		return fmt.Errorf("%d/%d checks failed", total-passed, total)
	}
	return nil
}

func setupTestRepo(dir string) error {
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "test@snag.dev"},
		{"git", "config", "user.name", "snag-test"},
		{"git", "commit", "--allow-empty", "-m", "initial commit"},
	}
	for _, c := range cmds {
		cmd := exec.Command(c[0], c[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			return fmt.Errorf("%s: %w\n%s", strings.Join(c, " "), err, out)
		}
	}
	return nil
}

func testDiff(cmd *cobra.Command, dir string, patterns []string) bool {
	// Create a file with a violation and stage it.
	violation := fmt.Sprintf("this has a %s in it\n", patterns[0])
	fpath := filepath.Join(dir, "bad.txt")
	if err := os.WriteFile(fpath, []byte(violation), 0644); err != nil {
		return false
	}
	gitAdd := exec.Command("git", "add", "bad.txt")
	gitAdd.Dir = dir
	if out, err := gitAdd.CombinedOutput(); err != nil {
		fmt.Fprintf(os.Stderr, "git add: %s\n", out)
		return false
	}

	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	err := runDiff(cmd, nil)
	return err != nil // error means violation detected = pass
}

func testMsg(cmd *cobra.Command, dir string, patterns []string) bool {
	// Pick two patterns: one for a trailer, one for the body.
	bodyPat := patterns[0]
	trailerPat := bodyPat
	if len(patterns) > 1 {
		trailerPat = patterns[1]
	}

	msgContent := fmt.Sprintf("Add new feature\n\nThis has a %s in the body\n\nSigned-off-by: %s@example.com\n", bodyPat, trailerPat)
	msgFile := filepath.Join(dir, "COMMIT_EDITMSG")
	if err := os.WriteFile(msgFile, []byte(msgContent), 0644); err != nil {
		return false
	}

	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	err := runMsg(cmd, []string{msgFile})
	return err != nil // error means violation detected = pass
}

func testPush(cmd *cobra.Command, dir string, patterns []string) bool {
	// Create a clean commit, then a commit with a violation in the diff.
	cleanFile := filepath.Join(dir, "clean.txt")
	if err := os.WriteFile(cleanFile, []byte("nothing wrong here\n"), 0644); err != nil {
		return false
	}
	run := func(args ...string) error {
		c := exec.Command(args[0], args[1:]...)
		c.Dir = dir
		out, err := c.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s: %w\n%s", strings.Join(args, " "), err, out)
		}
		return nil
	}
	if err := run("git", "add", "clean.txt"); err != nil {
		return false
	}
	if err := run("git", "commit", "-m", "clean commit"); err != nil {
		return false
	}

	badFile := filepath.Join(dir, "bad.txt")
	violation := fmt.Sprintf("this contains %s\n", patterns[len(patterns)-1])
	if err := os.WriteFile(badFile, []byte(violation), 0644); err != nil {
		return false
	}
	if err := run("git", "add", "bad.txt"); err != nil {
		return false
	}
	if err := run("git", "commit", "-m", "add bad file"); err != nil {
		return false
	}

	orig, _ := os.Getwd()
	os.Chdir(dir)
	defer os.Chdir(orig)

	err := runPush(cmd, nil)
	return err != nil // error means violation detected = pass
}
