package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var Version = "dev"

func init() {
	if Version != "dev" {
		return
	}
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return
	}

	// Local build — use VCS info for a short, readable version.
	var revision, modified string
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			if len(s.Value) >= 7 {
				revision = s.Value[:7]
			}
		case "vcs.modified":
			if s.Value == "true" {
				modified = "-dirty"
			}
		}
	}
	if revision != "" {
		Version = "dev+" + revision + modified
		return
	}

	// go install @version — no VCS info, but module version is set.
	if v := info.Main.Version; v != "" && v != "(devel)" {
		Version = v
	}
}

func buildRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "snag",
		Short:   fmt.Sprintf("snag %s — Composable git hook policy kit", Version),
		Version: Version,
	}

	rootCmd.SetVersionTemplate("snag version {{.Version}}\n")

	rootCmd.PersistentFlags().String("blocklist", ".blocklist", "path to blocklist file")
	rootCmd.PersistentFlags().BoolP("quiet", "q", false, "suppress non-error output")

	diffCmd := &cobra.Command{
		Use:          "diff",
		Short:        "Check staged diff against policies",
		SilenceUsage: true,
		RunE:         runDiff,
	}

	msgCmd := &cobra.Command{
		Use:          "msg FILE",
		Short:        "Check commit message against policies",
		Args:         cobra.ExactArgs(1),
		SilenceUsage: true,
		RunE:         runMsg,
	}

	pushCmd := &cobra.Command{
		Use:          "push",
		Short:        "Check pre-push policies",
		SilenceUsage: true,
		RunE:         runPush,
	}

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version and exit",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("snag version %s\n", Version)
		},
	}

	installHooksCmd := &cobra.Command{
		Use:          "install-hooks",
		Short:        "Add or update snag remote in lefthook config",
		SilenceUsage: true,
		RunE:         runInstallHooks,
	}
	installHooksCmd.Flags().Bool("local", false, "install to lefthook-local.yml (gitignored, just for you)")
	installHooksCmd.Flags().Bool("shared", false, "install to lefthook.yml (checked in, whole team)")
	installHooksCmd.Flags().BoolP("dry-run", "n", false, "show what would be changed without writing files")
	installHooksCmd.MarkFlagsMutuallyExclusive("local", "shared")

	rootCmd.AddCommand(diffCmd, msgCmd, pushCmd, versionCmd, installHooksCmd, buildTestCmd())
	return rootCmd
}

func main() {
	if err := buildRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
