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

	checkCmd := &cobra.Command{
		Use:   "check",
		Short: "Run policy checks (diff, msg, push)",
	}

	for _, h := range hooks {
		cmd := &cobra.Command{
			Use:          h.Use,
			Short:        h.Short,
			Args:         h.Args,
			SilenceUsage: true,
			RunE:         h.RunE,
		}
		checkCmd.AddCommand(cmd)
	}

	versionCmd := &cobra.Command{
		Use:   "version",
		Short: "Print version and exit",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Printf("snag version %s\n", Version)
		},
	}

	installCmd := &cobra.Command{
		Use:          "install",
		Short:        "Add or update snag remote in lefthook config",
		SilenceUsage: true,
		RunE:         runInstallHooks,
	}
	installCmd.Flags().Bool("local", false, "install to lefthook-local.yml (gitignored, just for you)")
	installCmd.Flags().Bool("shared", false, "install to lefthook.yml (checked in, whole team)")
	installCmd.Flags().BoolP("dry-run", "n", false, "show what would be changed without writing files")
	installCmd.MarkFlagsMutuallyExclusive("local", "shared")

	rootCmd.AddCommand(checkCmd, versionCmd, installCmd, buildTestCmd(), buildDemoCmd())
	return rootCmd
}

func main() {
	if err := buildRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
