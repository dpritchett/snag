package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

const snagRemoteURL = "https://github.com/dpritchett/snag.git"

// snagRecipeHookTypes lists hook types defined in the snag recipe.
// lefthook only installs .git/hooks/ scripts for types it sees in local
// config files, so we must add stubs for types that come solely from the
// remote recipe.
var snagRecipeHookTypes = []string{
	"commit-msg",
	"post-checkout",
	"pre-commit",
	"pre-push",
	"pre-rebase",
	"prepare-commit-msg",
}

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
      - recipes/lefthook-snag-filter.yml
`, snagRemoteURL, ref)
}

// snagRemoteBlockTrimmed returns the remotes block without a leading newline (for new files).
func snagRemoteBlockTrimmed(ref string) string {
	return strings.TrimLeft(snagRemoteBlock(ref), "\n")
}

// missingHookStubs returns a YAML block of empty hook-type stubs for
// snag recipe hook types not already present as top-level keys in content.
// Returns "" when nothing is missing.
func missingHookStubs(content string) string {
	var raw map[string]interface{}
	_ = yaml.Unmarshal([]byte(content), &raw)

	var missing []string
	for _, ht := range snagRecipeHookTypes {
		if _, ok := raw[ht]; !ok {
			missing = append(missing, ht)
		}
	}
	if len(missing) == 0 {
		return ""
	}

	var b strings.Builder
	b.WriteString("\n# Hook stubs — lefthook needs these to install hooks for remote recipe types.\n")
	for _, ht := range missing {
		fmt.Fprintf(&b, "%s:\n", ht)
	}
	return b.String()
}

// ensureHookStubs adds empty hook-type stubs to the main lefthook config.
//
// Stubs MUST live in the main config (not lefthook-local) because lefthook v2
// merges configs as: main → remotes → local. Empty stubs in the local config
// clobber the remote recipe's commands. In the main config, the remote recipe
// overrides the stubs during merge.
func ensureHookStubs(mainFile string, dryRun bool) (string, error) {
	data, err := os.ReadFile(mainFile)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", mainFile, err)
	}

	content := string(data)
	stubs := missingHookStubs(content)
	if stubs == "" {
		return "", nil
	}

	newContent := content
	if !strings.HasSuffix(newContent, "\n") {
		newContent += "\n"
	}
	newContent += stubs

	if dryRun {
		return unifiedDiff(mainFile, content, newContent), nil
	}
	if err := os.WriteFile(mainFile, []byte(newContent), 0644); err != nil {
		return "", fmt.Errorf("writing %s: %w", mainFile, err)
	}
	fmt.Fprintf(os.Stderr, "Added hook stubs to %s\n", mainFile)
	return "", nil
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
	ref := versionRef()

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
		// Stubs always go in the main config (see ensureHookStubs doc).
		if sharedErr == nil {
			if err := collectDiff(ensureHookStubs(sharedFile, dryRun)); err != nil && firstErr == nil {
				firstErr = err
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
	target, targetIsLocal := "", false

	if useLocal {
		target = localFile
		if target == "" {
			target = "lefthook-local.yml"
		}
		targetIsLocal = true
	} else if useShared {
		if sharedErr != nil {
			return sharedErr
		}
		target = sharedFile
	} else if !dryRun && isTTY() {
		choice, err := promptForConfigTarget()
		if err != nil {
			return err
		}
		if choice == "local" {
			target = localFile
			if target == "" {
				target = "lefthook-local.yml"
			}
			targetIsLocal = true
		} else {
			target = sharedFile
		}
	} else {
		// Default: shared config.
		if sharedErr != nil {
			return sharedErr
		}
		target = sharedFile
	}

	if err := collectDiff(installOrUpdateSnagRemote(target, targetIsLocal, dryRun)); err != nil {
		return err
	}

	// Stubs always go in the main config — never in local, because
	// lefthook v2 merges local OVER remotes and empty stubs clobber
	// the remote recipe's commands.
	if sharedErr == nil {
		if err := collectDiff(ensureHookStubs(sharedFile, dryRun)); err != nil {
			return err
		}
	}

	if dryRun {
		showDiffOutput(dryRunDiffs.String())
		return nil
	}
	fmt.Fprintf(os.Stderr, "Run `lefthook install` to activate.\n")
	return nil
}

// versionRef returns the Version string with a "v" prefix for use as a git tag ref.
// Dev builds are returned as-is since they aren't real tags.
func versionRef() string {
	if strings.HasPrefix(Version, "dev") {
		return Version
	}
	return "v" + strings.TrimPrefix(Version, "v")
}
