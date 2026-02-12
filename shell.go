package main

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"
)

// shellHook defines the contract for shell-specific hook generation.
// Each method returns one shell-specific code fragment. The compiler
// ensures every shell implements every detection stage.
type shellHook interface {
	name() string                // "fish", "bash", "zsh"
	preamble() string            // trigger/function setup (varies per shell)
	checkGitDir() string         // stage 1: fast bail if not a git repo
	checkHooksInstalled() string // stage 2: fast bail if lefthook+snag present
	checkSnagConfig() string     // stage 3: slow path — snag config has output
	checkQuiet() string          // stage 4: respect SNAG_QUIET
	getRepoName() string         // stage 5: git rev-parse --show-toplevel
	warn() string                // stage 6: colored warning to stderr
	bell() string                // stage 7: audible bell
	postamble() string           // close function / register hook
}

func renderHook(h shellHook) string {
	var b strings.Builder
	b.WriteString(h.preamble())
	b.WriteString(h.checkGitDir())
	b.WriteString(h.checkHooksInstalled())
	b.WriteString(h.checkSnagConfig())
	b.WriteString(h.checkQuiet())
	b.WriteString(h.getRepoName())
	b.WriteString(h.warn())
	b.WriteString(h.bell())
	b.WriteString(h.postamble())
	return b.String()
}

// --- fish ---

type fishShell struct{}

func (fishShell) name() string { return "fish" }

func (fishShell) preamble() string {
	return "function __snag_check --on-variable PWD\n"
}

func (fishShell) checkGitDir() string {
	return `    # Fast bail: not a git repo
    test -d .git; or return
`
}

func (fishShell) checkHooksInstalled() string {
	return `
    # Fast bail: lefthook is the hook runner AND its config references snag
    set -l hook .git/hooks/pre-commit
    if test -f $hook; and grep -q lefthook $hook
        grep -rql snag lefthook.yml lefthook-local.yml 2>/dev/null; and return
    end
`
}

func (fishShell) checkSnagConfig() string {
	return `
    # Check if snag config governs this repo (walks up directory tree)
    snag config 2>/dev/null | grep -q .; or return
`
}

func (fishShell) checkQuiet() string {
	return `
    # Respect SNAG_QUIET
    test -n "$SNAG_QUIET"; and return
`
}

func (fishShell) getRepoName() string {
	return `
    set -l repo_id (git rev-parse --show-toplevel 2>/dev/null)
    test -z "$repo_id"; and return
`
}

func (fishShell) warn() string {
	return `
    echo (set_color --bold red)"snag:"(set_color normal)" hooks not installed in "(set_color --bold yellow)(basename $repo_id)(set_color normal)" — run: "(set_color green)"snag install && lefthook install"(set_color normal) >&2
`
}

func (fishShell) bell() string {
	return "    printf '\\a' # audible bell\n"
}

func (fishShell) postamble() string {
	return "end\n"
}

// --- bash ---

type bashShell struct{}

func (bashShell) name() string { return "bash" }

func (bashShell) preamble() string {
	return `__snag_check() {
    [[ "$PWD" == "$__snag_last_pwd" ]] && return
    __snag_last_pwd="$PWD"
`
}

func (bashShell) checkGitDir() string {
	return `
    # Fast bail: not a git repo
    [[ -d .git ]] || return
`
}

func (bashShell) checkHooksInstalled() string {
	return `
    # Fast bail: lefthook is the hook runner AND its config references snag
    local hook=".git/hooks/pre-commit"
    if [[ -f "$hook" ]] && grep -q lefthook "$hook"; then
        grep -rql snag lefthook.yml lefthook-local.yml 2>/dev/null && return
    fi
`
}

func (bashShell) checkSnagConfig() string {
	return `
    # Check if snag config governs this repo (walks up directory tree)
    snag config 2>/dev/null | grep -q . || return
`
}

func (bashShell) checkQuiet() string {
	return `
    # Respect SNAG_QUIET
    [[ -n "$SNAG_QUIET" ]] && return
`
}

func (bashShell) getRepoName() string {
	return `
    local repo_id
    repo_id="$(git rev-parse --show-toplevel 2>/dev/null)"
    [[ -z "$repo_id" ]] && return
`
}

func (bashShell) warn() string {
	return `
    printf '\033[1;31msnag:\033[0m hooks not installed in \033[1;33m%s\033[0m — run: \033[32msnag install && lefthook install\033[0m\n' "$(basename "$repo_id")" >&2
`
}

func (bashShell) bell() string {
	return "    printf '\\a' # audible bell\n"
}

func (bashShell) postamble() string {
	return `}
PROMPT_COMMAND="__snag_check${PROMPT_COMMAND:+;$PROMPT_COMMAND}"
`
}

// --- zsh ---

type zshShell struct{}

func (zshShell) name() string { return "zsh" }

func (zshShell) preamble() string {
	return "__snag_check() {\n"
}

func (zshShell) checkGitDir() string {
	return `    # Fast bail: not a git repo
    [[ -d .git ]] || return
`
}

func (zshShell) checkHooksInstalled() string {
	return `
    # Fast bail: lefthook is the hook runner AND its config references snag
    local hook=".git/hooks/pre-commit"
    if [[ -f "$hook" ]] && grep -q lefthook "$hook"; then
        grep -rql snag lefthook.yml lefthook-local.yml 2>/dev/null && return
    fi
`
}

func (zshShell) checkSnagConfig() string {
	return `
    # Check if snag config governs this repo (walks up directory tree)
    snag config 2>/dev/null | grep -q . || return
`
}

func (zshShell) checkQuiet() string {
	return `
    # Respect SNAG_QUIET
    [[ -n "$SNAG_QUIET" ]] && return
`
}

func (zshShell) getRepoName() string {
	return `
    local repo_id
    repo_id="$(git rev-parse --show-toplevel 2>/dev/null)"
    [[ -z "$repo_id" ]] && return
`
}

func (zshShell) warn() string {
	return `
    printf '\033[1;31msnag:\033[0m hooks not installed in \033[1;33m%s\033[0m — run: \033[32msnag install && lefthook install\033[0m\n' "$(basename "$repo_id")" >&2
`
}

func (zshShell) bell() string {
	return "    printf '\\a' # audible bell\n"
}

func (zshShell) postamble() string {
	return `}
chpwd_functions+=(__snag_check)
`
}

// --- command ---

func buildShellCmd() *cobra.Command {
	shellCmd := &cobra.Command{
		Use:   "shell",
		Short: "Print shell integration hooks",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var h shellHook
			switch args[0] {
			case "fish":
				h = fishShell{}
			case "bash":
				h = bashShell{}
			case "zsh":
				h = zshShell{}
			default:
				return fmt.Errorf("unsupported shell: %s (supported: bash, fish, zsh)", args[0])
			}
			fmt.Fprint(cmd.OutOrStdout(), renderHook(h))
			return nil
		},
	}
	return shellCmd
}
