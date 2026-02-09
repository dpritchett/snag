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
func installOrUpdateSnagRemote(filename string, createIfMissing bool) error {
	ref := Version

	data, err := os.ReadFile(filename)
	if err != nil {
		if !os.IsNotExist(err) || !createIfMissing {
			return fmt.Errorf("reading %s: %w", filename, err)
		}
		// File doesn't exist — create with just the snag remote block.
		if err := os.WriteFile(filename, []byte(snagRemoteBlockTrimmed(ref)), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", filename, err)
		}
		fmt.Fprintf(os.Stderr, "Created %s with snag %s remote\n", filename, ref)
		return nil
	}

	existingRef, err := findSnagRemote(data)
	if err != nil {
		return fmt.Errorf("parsing %s: %w", filename, err)
	}

	content := string(data)

	if existingRef == "" {
		// No snag remote — append block to end of file.
		if !strings.HasSuffix(content, "\n") {
			content += "\n"
		}
		content += snagRemoteBlock(ref)
		if err := os.WriteFile(filename, []byte(content), 0644); err != nil {
			return fmt.Errorf("writing %s: %w", filename, err)
		}
		fmt.Fprintf(os.Stderr, "Added snag %s remote to %s\n", ref, filename)
		return nil
	}

	if existingRef == ref {
		fmt.Fprintf(os.Stderr, "snag remote already configured at %s in %s\n", ref, filename)
		return nil
	}

	// Snag remote exists at a different version — surgically replace the ref.
	oldRef := "ref: " + existingRef
	newRef := "ref: " + ref
	updated := strings.Replace(content, oldRef, newRef, 1)
	if updated == content {
		return fmt.Errorf("found snag remote at %s but could not locate ref line in %s", existingRef, filename)
	}
	if err := os.WriteFile(filename, []byte(updated), 0644); err != nil {
		return fmt.Errorf("writing %s: %w", filename, err)
	}
	fmt.Fprintf(os.Stderr, "Updated snag remote from %s to %s in %s\n", existingRef, ref, filename)
	return nil
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

	// Detection-first: if snag is already present somewhere, update in place.
	if sharedHasSnag || localHasSnag {
		var firstErr error
		if sharedHasSnag {
			if err := installOrUpdateSnagRemote(sharedFile, false); err != nil {
				firstErr = err
			}
		}
		if localHasSnag {
			if err := installOrUpdateSnagRemote(localFile, false); err != nil {
				if firstErr == nil {
					firstErr = err
				}
			}
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
		if err := installOrUpdateSnagRemote(target, true); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Run `lefthook install` to activate.\n")
		return nil
	}

	if useShared {
		if sharedErr != nil {
			return sharedErr
		}
		if err := installOrUpdateSnagRemote(sharedFile, false); err != nil {
			return err
		}
		fmt.Fprintf(os.Stderr, "Run `lefthook install` to activate.\n")
		return nil
	}

	// No flags — prompt if TTY, otherwise default to shared.
	if isTTY() {
		choice, err := promptForConfigTarget()
		if err != nil {
			return err
		}
		if choice == "local" {
			target := localFile
			if target == "" {
				target = "lefthook-local.yml"
			}
			if err := installOrUpdateSnagRemote(target, true); err != nil {
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
	if err := installOrUpdateSnagRemote(sharedFile, false); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "Run `lefthook install` to activate.\n")
	return nil
}
