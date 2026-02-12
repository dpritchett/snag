package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var defaultInitConfig = `min_version = "` + minVersionForInit + `"

[block]
diff = [
  "DO NOT MERGE",
  "DO NOT COMMIT",
  "FIXME",
  "HACK",
]
msg = [
  "DO NOT MERGE",
  "FIXME",
  "WIP",
  "fixup!",
  "squash!",
]
# push: omit to inherit diff + msg patterns as a safety net
branch = ["main", "master"]
`

var defaultLocalConfig = `min_version = "` + minVersionForInit + `"

# Personal/sensitive patterns — this file should be gitignored.
[block]
diff = []
msg  = []
`

// minVersionForInit is the snag version that introduced snag.toml support.
const minVersionForInit = "0.10.0"

func buildInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:          "init",
		Short:        "Generate a starter snag.toml in the current directory",
		SilenceUsage: true,
		RunE:         runInit,
	}
	cmd.Flags().Bool("force", false, "overwrite existing config file")
	cmd.Flags().Bool("local", false, "generate snag-local.toml (gitignored, personal patterns)")
	return cmd
}

func runInit(cmd *cobra.Command, args []string) error {
	dir, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("getting working directory: %w", err)
	}

	local, _ := cmd.Flags().GetBool("local")
	force, _ := cmd.Flags().GetBool("force")
	quiet, _ := cmd.Flags().GetBool("quiet")

	if local {
		return initLocal(dir, force, quiet)
	}
	return initShared(dir, force, quiet)
}

func initShared(dir string, force, quiet bool) error {
	dest := filepath.Join(dir, "snag.toml")

	if !force && fileExists(dest) {
		return fmt.Errorf("snag.toml already exists (use --force to overwrite)")
	}

	if err := os.WriteFile(dest, []byte(defaultInitConfig), 0644); err != nil {
		return fmt.Errorf("writing snag.toml: %w", err)
	}
	if !quiet {
		infof("created snag.toml with starter patterns")
		hintf("edit snag.toml to customize your team policy")
	}
	return nil
}

func initLocal(dir string, force, quiet bool) error {
	dest := filepath.Join(dir, "snag-local.toml")

	if !force && fileExists(dest) {
		return fmt.Errorf("snag-local.toml already exists (use --force to overwrite)")
	}

	if err := os.WriteFile(dest, []byte(defaultLocalConfig), 0644); err != nil {
		return fmt.Errorf("writing snag-local.toml: %w", err)
	}
	if !quiet {
		infof("created snag-local.toml for personal/sensitive patterns")
		hintf("add snag-local.toml to .gitignore")
	}
	return nil
}
