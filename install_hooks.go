package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

const snagRemoteURL = "https://github.com/dpritchett/snag.git"

var defaultRecipes = []string{
	"recipes/lefthook-blocklist.yml",
}

func runInstallHooks(cmd *cobra.Command, args []string) error {
	const filename = "lefthook.yml"

	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("no %s found — run `lefthook init` first", filename)
		}
		return fmt.Errorf("reading %s: %w", filename, err)
	}

	var raw map[string]interface{}
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("parsing %s: %w", filename, err)
	}
	if raw == nil {
		raw = make(map[string]interface{})
	}

	ref := Version

	// Check existing remotes for a snag entry.
	remotes, _ := raw["remotes"].([]interface{})
	for i, r := range remotes {
		entry, ok := r.(map[string]interface{})
		if !ok {
			continue
		}
		if entry["git_url"] == snagRemoteURL {
			existingRef, _ := entry["ref"].(string)
			if existingRef == ref {
				fmt.Fprintf(os.Stderr, "snag remote already configured at %s in %s\n", ref, filename)
				return nil
			}
			// Update ref in place.
			entry["ref"] = ref
			remotes[i] = entry
			raw["remotes"] = remotes

			out, err := yaml.Marshal(raw)
			if err != nil {
				return fmt.Errorf("marshalling %s: %w", filename, err)
			}
			if err := os.WriteFile(filename, out, 0644); err != nil {
				return fmt.Errorf("writing %s: %w", filename, err)
			}
			fmt.Fprintf(os.Stderr, "Updated snag remote from %s to %s in %s\n", existingRef, ref, filename)
			fmt.Fprintf(os.Stderr, "Run `lefthook install` to activate.\n")
			return nil
		}
	}

	// No snag remote found — append one.
	newRemote := map[string]interface{}{
		"git_url": snagRemoteURL,
		"ref":     ref,
		"configs": defaultRecipes,
	}
	remotes = append(remotes, newRemote)
	raw["remotes"] = remotes

	out, err := yaml.Marshal(raw)
	if err != nil {
		return fmt.Errorf("marshalling %s: %w", filename, err)
	}
	if err := os.WriteFile(filename, out, 0644); err != nil {
		return fmt.Errorf("writing %s: %w", filename, err)
	}
	fmt.Fprintf(os.Stderr, "Added snag %s remote to %s\n", ref, filename)
	fmt.Fprintf(os.Stderr, "Run `lefthook install` to activate.\n")
	return nil
}
