package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

const fishHook = `function __snag_check --on-variable PWD
    # Fast bail: not a git repo
    test -d .git; or return

    # Fast bail: lefthook is the hook runner AND its config references snag
    set -l hook .git/hooks/pre-commit
    if test -f $hook; and grep -q lefthook $hook
        grep -rql snag lefthook.yml lefthook-local.yml 2>/dev/null; and return
    end

    # Check if snag config governs this repo (walks up directory tree)
    snag config 2>/dev/null | grep -q .; or return

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
