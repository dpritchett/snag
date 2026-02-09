package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
)

var Version = "dev"

func init() {
	if Version == "dev" {
		if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" && info.Main.Version != "(devel)" {
			Version = info.Main.Version
		}
	}
}

func buildRootCmd() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "snag",
		Short:   "Composable git hook policy kit",
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

	rootCmd.AddCommand(diffCmd, msgCmd, pushCmd, versionCmd)
	return rootCmd
}

func main() {
	if err := buildRootCmd().Execute(); err != nil {
		os.Exit(1)
	}
}
