package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

const fishHook = `function __snag_check --on-variable PWD
    # Fast bail: not a git repo
    test -d .git; or return

    # Fast bail: no snag config in this directory
    test -f snag.toml -o -f snag-local.toml -o -f .blocklist; or return

    # Fast bail: lefthook pre-commit hook exists and mentions lefthook
    set -l hook .git/hooks/pre-commit
    test -f $hook; and grep -q lefthook $hook; and return

    # Slow path: call snag for full walk-up detection
    snag check checkout -q 2>/dev/null; and return

    # Once-per-repo-per-session guard
    set -l repo_id (git rev-parse --show-toplevel 2>/dev/null)
    test -z "$repo_id"; and return
    contains -- $repo_id $__snag_warned; and return
    set -g -a __snag_warned $repo_id

    # Respect SNAG_QUIET
    test -n "$SNAG_QUIET"; and return

    echo "snag: hooks not installed in "(basename $repo_id)" — run: snag install && lefthook install" >&2
end
`

func buildShellCmd() *cobra.Command {
	shellCmd := &cobra.Command{
		Use:   "shell",
		Short: "Print shell integration hooks",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			switch args[0] {
			case "fish":
				fmt.Fprint(cmd.OutOrStdout(), fishHook)
				return nil
			default:
				return fmt.Errorf("unsupported shell: %s (supported: fish)", args[0])
			}
		},
	}
	return shellCmd
}
