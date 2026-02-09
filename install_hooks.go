package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"
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

// findLefthookConfig returns the first existing lefthook config filename.
func findLefthookConfig() (string, error) {
	for _, name := range lefthookCandidates {
		if _, err := os.Stat(name); err == nil {
			return name, nil
		}
	}
	return "", fmt.Errorf("no lefthook config found (tried %v) — run `lefthook init` first", lefthookCandidates)
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

func runInstallHooks(cmd *cobra.Command, args []string) error {
	filename, err := findLefthookConfig()
	if err != nil {
		return err
	}

	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("reading %s: %w", filename, err)
	}

	ref := Version
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
		fmt.Fprintf(os.Stderr, "Run `lefthook install` to activate.\n")
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
	fmt.Fprintf(os.Stderr, "Run `lefthook install` to activate.\n")
	return nil
}
