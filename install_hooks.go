package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

const snagRemoteURL = "https://github.com/dpritchett/snag.git"

// lefthookCandidates lists filenames lefthook accepts, in priority order.
var lefthookCandidates = []string{
	"lefthook.yml",
	"lefthook.yaml",
	".lefthook.yml",
	".lefthook.yaml",
}

// lefthookLocalCandidates lists local config filenames lefthook merges.
var lefthookLocalCandidates = []string{
	"lefthook-local.yml",
	"lefthook-local.yaml",
	".lefthook-local.yml",
	".lefthook-local.yaml",
}

// findLefthookConfig returns the first existing lefthook config filename.
func findLefthookConfig() (string, error) {
	for _, name := range lefthookCandidates {
		if _, err := os.Stat(name); err == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("no lefthook config found (tried %v) — run `lefthook init` first", lefthookCandidates)
}

// findLefthookLocalConfig returns the first existing local config, or ("", nil) if none found.
func findLefthookLocalConfig() (string, error) {
	for _, name := range lefthookLocalCandidates {
		if _, err := os.Stat(name); err == nil {
			return name, nil
		}
	}
	return "", nil
}

// snagRemoteBlock returns a formatted remotes block to append to a lefthook config.
func snagRemoteBlock(ref string) string {
	return fmt.Sprintf(`
remotes:
  - git_url: %s
    ref: %s
    configs:
      - recipes/lefthook-blocklist.yml
`, snagRemoteURL, ref)
}

// snagRemoteBlockTrimmed returns the remotes block without a leading newline (for new files).
func snagRemoteBlockTrimmed(ref string) string {
	return strings.TrimLeft(snagRemoteBlock(ref), "\n")
}

// findSnagRemote parses the YAML and returns the existing snag remote's ref, or "" if not found.
func findSnagRemote(data []byte) (string, error) {
	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return "", err
	}

	remotes, _ := raw["remotes"].([]interface{})
	for _, r := range remotes {
		entry, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		if entry["git_url"] == snagRemoteURL {
			ref, _ := entry["ref"].(string)
			return ref, nil
		}
	}
	return "", nil
}

// installOrUpdateSnagRemote adds or updates the snag remote in the given config file.
// If createIfMissing is true and the file doesn't exist, it creates it.
// If dryRun is true, it returns a unified diff string describing the change without writing.
func installOrUpdateSnagRemote(filename string, createIfMissing bool, dryRun bool) (string, error) {
	ref := Version

	data, err := os.ReadFile(filename)
	if err != nil {
		if !os.IsNotExist(err) || !createIfMissing {
			return "", fmt.Errorf("reading %s: %w", filename, err)
		}
		// File doesn't exist — create with just the snag remote block.
		newContent := snagRemoteBlockTrimmed(ref)
		if dryRun {
			return unifiedDiff(filename, "", newContent), nil
		}
		if err := os.WriteFile(filename, []byte(newContent), 0644); err != nil {
			return "", fmt.Errorf("writing %s: %w", filename, err)
		}
		fmt.Fprintf(os.Stderr, "Created %s with snag %s remote\n", filename, ref)
		return "", nil
	}

	existingRef, err := findSnagRemote(data)
	if err != nil {
		return "", fmt.Errorf("parsing %s: %w", filename, err)
	}

	content := string(data)

	if existingRef == "" {
		// No snag remote — append block to end of file.
		block := snagRemoteBlock(ref)
		newContent := content
		if !strings.HasSuffix(newContent, "\n") {
			newContent += "\n"
		}
		newContent += block
		if dryRun {
			return unifiedDiff(filename, content, newContent), nil
		}
		if err := os.WriteFile(filename, []byte(newContent), 0644); err != nil {
			return "", fmt.Errorf("writing %s: %w", filename, err)
		}
		fmt.Fprintf(os.Stderr, "Added snag %s remote to %s\n", ref, filename)
		return "", nil
	}

	if existingRef == ref {
		fmt.Fprintf(os.Stderr, "snag remote already configured at %s in %s — no changes needed\n", ref, filename)
		return "", nil
	}

	// Snag remote exists at a different version — surgically replace the ref.
	oldRef := "ref: " + existingRef
	newRef := "ref: " + ref
	updated := strings.Replace(content, oldRef, newRef, 1)
	if updated == content {
		return "", fmt.Errorf("found snag remote at %s but could not locate ref line in %s", existingRef, filename)
	}
	if dryRun {
		return unifiedDiff(filename, content, updated), nil
	}
	if err := os.WriteFile(filename, []byte(updated), 0644); err != nil {
		return "", fmt.Errorf("writing %s: %w", filename, err)
	}
	fmt.Fprintf(os.Stderr, "Updated snag remote from %s to %s in %s\n", existingRef, ref, filename)
	return "", nil
}

// unifiedDiff generates a minimal unified diff between old and new content for filename.
func unifiedDiff(filename, oldText, newText string) string {
	oldLines := splitLines(oldText)
	newLines := splitLines(newText)

	var b strings.Builder
	if oldText == "" {
		// New file.
		fmt.Fprintf(&b, "--- /dev/null\n")
		fmt.Fprintf(&b, "+++ b/%s\n", filename)
		fmt.Fprintf(&b, "@@ -0,0 +1,%d @@\n", len(newLines))
		for _, line := range newLines {
			fmt.Fprintf(&b, "+%s\n", line)
		}
		return b.String()
	}

	// Find the first and last differing lines for a single hunk.
	start := 0
	for start < len(oldLines) && start < len(newLines) && oldLines[start] == newLines[start] {
		start++
	}
	endOld := len(oldLines)
	endNew := len(newLines)
	for endOld > start && endNew > start && oldLines[endOld-1] == newLines[endNew-1] {
		endOld--
		endNew--
	}

	// Context: up to 3 lines before and after.
	ctxBefore := 3
	if start < ctxBefore {
		ctxBefore = start
	}
	ctxAfterOld := 3
	if len(oldLines)-endOld < ctxAfterOld {
		ctxAfterOld = len(oldLines) - endOld
	}
	ctxAfterNew := 3
	if len(newLines)-endNew < ctxAfterNew {
		ctxAfterNew = len(newLines) - endNew
	}
	// Use the smaller of the two after-contexts (they should be equal for our diffs).
	ctxAfter := ctxAfterOld
	if ctxAfterNew < ctxAfter {
		ctxAfter = ctxAfterNew
	}

	hunkStartOld := start - ctxBefore
	hunkStartNew := start - ctxBefore
	hunkEndOld := endOld + ctxAfter
	hunkEndNew := endNew + ctxAfter

	fmt.Fprintf(&b, "--- a/%s\n", filename)
	fmt.Fprintf(&b, "+++ b/%s\n", filename)
	fmt.Fprintf(&b, "@@ -%d,%d +%d,%d @@\n",
		hunkStartOld+1, hunkEndOld-hunkStartOld,
		hunkStartNew+1, hunkEndNew-hunkStartNew)

	// Leading context.
	for i := hunkStartOld; i < start; i++ {
		fmt.Fprintf(&b, " %s\n", oldLines[i])
	}
	// Removed lines.
	for i := start; i < endOld; i++ {
		fmt.Fprintf(&b, "-%s\n", oldLines[i])
	}
	// Added lines.
	for i := start; i < endNew; i++ {
		fmt.Fprintf(&b, "+%s\n", newLines[i])
	}
	// Trailing context.
	for i := endOld; i < hunkEndOld; i++ {
		fmt.Fprintf(&b, " %s\n", oldLines[i])
	}

	return b.String()
}

// splitLines splits text into lines, handling the trailing newline correctly.
func splitLines(text string) []string {
	if text == "" {
		return nil
	}
	text = strings.TrimRight(text, "\n")
	return strings.Split(text, "\n")
}

// findDiffPager returns the user's preferred diff pager command, checking
// GIT_PAGER, git config core.pager, PAGER, in that order. Returns "" if
// none configured or the binary isn't found on PATH.
var findDiffPager = func() string {
	// GIT_PAGER takes top priority.
	if p := os.Getenv("GIT_PAGER"); p != "" {
		if name := firstWord(p); name != "" {
			if _, err := exec.LookPath(name); err == nil {
				return p
			}
		}
	}

	// git config core.pager.
	if out, err := exec.Command("git", "config", "core.pager").Output(); err == nil {
		p := strings.TrimSpace(string(out))
		if p != "" {
			if name := firstWord(p); name != "" {
				if _, err := exec.LookPath(name); err == nil {
					return p
				}
			}
		}
	}

	// PAGER env var.
	if p := os.Getenv("PAGER"); p != "" {
		if name := firstWord(p); name != "" {
			if _, err := exec.LookPath(name); err == nil {
				return p
			}
		}
	}

	return ""
}

// firstWord returns the first whitespace-delimited token from s.
func firstWord(s string) string {
	s = strings.TrimSpace(s)
	if i := strings.IndexAny(s, " \t"); i != -1 {
		return s[:i]
	}
	return s
}

// showDiffOutput writes diff text to stderr, piping through the user's pager
// when stderr is a TTY and a pager is available.
func showDiffOutput(diff string) {
	if diff == "" {
		return
	}

	if isTTY() {
		if pager := findDiffPager(); pager != "" {
			cmd := exec.Command("sh", "-c", pager)
			cmd.Stdin = strings.NewReader(diff)
			cmd.Stdout = os.Stderr // pager output goes to stderr like the rest of our output
			cmd.Stderr = os.Stderr
			if err := cmd.Run(); err == nil {
				return
			}
			// Fall through to plain output on pager error.
		}
	}

	fmt.Fprint(os.Stderr, diff)
}

// isTTY reports whether stdin and stderr are connected to a terminal.
var isTTY = func() bool {
	return term.IsTerminal(int(os.Stdin.Fd())) && term.IsTerminal(int(os.Stderr.Fd()))
}

// promptForConfigTarget asks the user interactively whether to install to shared or local config.
// Returns "shared" or "local".
var promptForConfigTarget = func() (string, error) {
	var choice string
	err := huh.NewSelect[string]().
		Title("Where should snag hooks be installed?").
		Options(
			huh.NewOption("Shared config (lefthook.yml) — checked in, whole team gets it", "shared"),
			huh.NewOption("Local config (lefthook-local.yml) — gitignored, just for you", "local"),
		).
		Value(&choice).
		Run()
	if err != nil {
		return "", fmt.Errorf("prompt cancelled: %w", err)
	}
	return choice, nil
}

func runInstallHooks(cmd *cobra.Command, args []string) error {
	useLocal, _ := cmd.Flags().GetBool("local")
	useShared, _ := cmd.Flags().GetBool("shared")
	dryRun, _ := cmd.Flags().GetBool("dry-run")

	if useLocal && useShared {
		return fmt.Errorf("--local and --shared are mutually exclusive")
	}

	sharedFile, sharedErr := findLefthookConfig()
	localFile, _ := findLefthookLocalConfig()

	// Check for existing snag remotes in both configs.
	sharedHasSnag := false
	localHasSnag := false

	if sharedErr == nil {
		data, err := os.ReadFile(sharedFile)
		if err == nil {
			existing, _ := findSnagRemote(data)
			sharedHasSnag = existing != ""
		}
	}

	if localFile != "" {
		data, err := os.ReadFile(localFile)
		if err == nil {
			existing, _ := findSnagRemote(data)
			localHasSnag = existing != ""
		}
	}

	// dryRunCollect gathers diff output when in dry-run mode.
	var dryRunDiffs strings.Builder
	collectDiff := func(diff string, err error) error {
		if err != nil {
			return err
		}
		if diff != "" {
			dryRunDiffs.WriteString(diff)
		}
		return nil
	}

	// Detection-first: if snag is already present somewhere, update in place.
	if sharedHasSnag || localHasSnag {
		var firstErr error
		if sharedHasSnag {
			diff, err := installOrUpdateSnagRemote(sharedFile, false, dryRun)
			if err != nil {
				firstErr = err
			} else if dryRun {
				dryRunDiffs.WriteString(diff)
			}
		}
		if localHasSnag {
			diff, err := installOrUpdateSnagRemote(localFile, false, dryRun)
			if err != nil && firstErr == nil {
				firstErr = err
			} else if dryRun {
				dryRunDiffs.WriteString(diff)
			}
		}
		if dryRun {
			showDiffOutput(dryRunDiffs.String())
			return firstErr
		}
		if sharedHasSnag && localHasSnag {
			fmt.Fprintf(os.Stderr, "Note: snag remote found in both %s and %s; updated both.\n", sharedFile, localFile)
		}
		fmt.Fprintf(os.Stderr, "Run `lefthook install` to activate.\n")
		return firstErr
	}

	// Fresh install — decide target based on flags or prompt.
	if useLocal {
		target := localFile
		if target == "" {
			target = "lefthook-local.yml"
		}
		if err := collectDiff(installOrUpdateSnagRemote(target, true, dryRun)); err != nil {
			return err
		}
		if dryRun {
			showDiffOutput(dryRunDiffs.String())
			return nil
		}
		fmt.Fprintf(os.Stderr, "Run `lefthook install` to activate.\n")
		return nil
	}

	if useShared {
		if sharedErr != nil {
			return sharedErr
		}
		if err := collectDiff(installOrUpdateSnagRemote(sharedFile, false, dryRun)); err != nil {
			return err
		}
		if dryRun {
			showDiffOutput(dryRunDiffs.String())
			return nil
		}
		fmt.Fprintf(os.Stderr, "Run `lefthook install` to activate.\n")
		return nil
	}

	// No flags — prompt if TTY, otherwise default to shared.
	if !dryRun && isTTY() {
		choice, err := promptForConfigTarget()
		if err != nil {
			return err
		}
		if choice == "local" {
			target := localFile
			if target == "" {
				target = "lefthook-local.yml"
			}
			if _, err := installOrUpdateSnagRemote(target, true, false); err != nil {
				return err
			}
			fmt.Fprintf(os.Stderr, "Run `lefthook install` to activate.\n")
			return nil
		}
		// choice == "shared", fall through
	}

	// Default: shared config.
	if sharedErr != nil {
		return sharedErr
	}
	if err := collectDiff(installOrUpdateSnagRemote(sharedFile, false, dryRun)); err != nil {
		return err
	}
	if dryRun {
		showDiffOutput(dryRunDiffs.String())
		return nil
	}
	fmt.Fprintf(os.Stderr, "Run `lefthook install` to activate.\n")
	return nil
}
