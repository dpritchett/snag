# snag — Product Requirements Document

## Problem

Every repo needs git hooks. Formatting checks, secret scanning, commit message
standards, content policies — the list grows with each project. Most teams
solve this one repo at a time: a shell script here, a husky config there,
maybe a pre-commit framework if someone sets it up.

The result is fragmented. Your Go repos have different hooks than your JS repos.
New repos start with nothing. That one careful setup from two years ago has
drifted. Nobody remembers which repos have secret scanning and which don't.

You could write a shell one-liner for each check. Then you need it in three
hook phases (pre-commit, commit-msg, pre-push), each with slightly different
behavior. Then you realize you need it in 40 repos, each with different hook
stacks. The one-liner is now a project.

snag is that project, kept small on purpose.

## Goal

A composable git hook policy kit. snag provides two things:

1. **A curated set of lefthook recipe files** covering common checks (content
   policy, secret scanning, formatting, linting). Each recipe is a standalone
   lefthook fragment that any repo can pull in via
   [lefthook remotes](https://github.com/evilmartians/lefthook/blob/master/docs/configuration.md#remotes).

2. **A small Go CLI** (`snag`) for checks that don't have an existing
   off-the-shelf tool — specifically, per-repo content policy enforcement via
   a `.blocklist` file.

Most recipes call existing tools (gitleaks, shellcheck, go vet). The snag CLI
exists to fill the gap where no good tool exists yet: matching staged content,
commit messages, and unpushed diffs against a simple deny list.

## Non-Goals

- **Not a hook framework.** snag doesn't manage hooks, install itself into
  `.git/hooks`, or wrap other tools. That's lefthook's job (or husky, or
  pre-commit, or raw githooks).
- **Not a linter.** The CLI doesn't understand language syntax, ASTs, or file
  types. It matches substrings against text.
- **Not a replacement for language-specific tooling.** snag recipes *call*
  shellcheck, gitleaks, go vet, etc. rather than reimplementing them.
- **No regex or glob support in the CLI.** Case-insensitive substring matching
  is the right level of complexity for a deny list. If you need regex, you need
  a different tool.

## Recipes

The `recipes/` directory is the primary product surface. Each file is a
self-contained lefthook fragment that any repo can adopt independently.

### Shipped recipes

| Recipe | Hook phase(s) | What it does | Requires |
|---|---|---|---|
| `lefthook-blocklist.yml` | pre-commit, commit-msg, pre-push | Content policy via `.blocklist` | `snag` CLI |
| `lefthook-gitleaks.yml` | pre-commit | Secret scanning | `gitleaks` |
| `lefthook-go.yml` | pre-commit | `go fmt`, `go vet`, `go test` | Go toolchain |
| `lefthook-shellcheck.yml` | pre-commit | Lint staged shell scripts | `shellcheck` |

Each recipe is independent. A repo can pull one or all of them.

### Adopting recipes via lefthook remotes

Each consuming repo's `lefthook.yml` picks only the recipes it needs:

```yaml
# repo-a/lefthook.yml — JS project, needs blocklist + gitleaks only
remotes:
  - git_url: https://github.com/dpritchett/snag.git
    ref: main
    configs:
      - recipes/lefthook-blocklist.yml
      - recipes/lefthook-gitleaks.yml
```

```yaml
# repo-b/lefthook.yml — Go project, full stack
remotes:
  - git_url: https://github.com/dpritchett/snag.git
    ref: main
    configs:
      - recipes/lefthook-blocklist.yml
      - recipes/lefthook-gitleaks.yml
      - recipes/lefthook-go.yml
```

```yaml
# repo-c/lefthook.yml — shell scripts and markdown
remotes:
  - git_url: https://github.com/dpritchett/snag.git
    ref: main
    configs:
      - recipes/lefthook-blocklist.yml
      - recipes/lefthook-shellcheck.yml
```

Each repo picks its own subset. No monolithic config. Pin `ref` to a tag
(e.g. `ref: v1.0.0`) for stability, or use `main` to track latest.

**Local overrides.** Any repo can add a `lefthook-local.yml` to extend or
override what the remote recipes provide. lefthook merges local config on top
of remote config automatically — no special setup required.

## The `snag` CLI

A single-purpose binary for content policy checks that don't have an existing
tool. Works with any git hook runner, not just lefthook.

### Three subcommands, one per hook phase

```
snag diff          # pre-commit: scan staged changes
snag msg FILE      # commit-msg: clean trailers, reject body matches
snag push          # pre-push: scan all unpushed commits
```

All three:

1. Read `.blocklist` from the repo root (discovered via
   `git rev-parse --show-toplevel`), or from `--blocklist PATH`
2. Perform case-insensitive substring matching
3. Exit 0 (clean) or 1 (match found, with a human-readable error)

### `snag diff`

Runs `git diff --staged`, lowercases it, checks for matches.

```
$ snag diff
snag: match "do not merge" in staged diff
```

Exit 1. Commit blocked.

### `snag msg FILE`

Reads the commit message file passed by the hook runner. Two-pass approach:

**Pass 1 — Trailer stripping.** Lines in git trailer format (`Key: Value`, no
spaces in the key) that match a blocklist pattern are silently removed. The
file is rewritten in place. This lets you automatically clean up unwanted
trailers injected by tools without rejecting the whole commit.

**Pass 2 — Body check.** The remaining message is checked for matches. If
found, the commit is rejected with a recovery hint so the user doesn't lose
their message.

```
$ snag msg .git/COMMIT_EDITMSG
snag: removed 1 trailer line(s)
```

```
$ snag msg .git/COMMIT_EDITMSG
snag: match "fixme" in commit message
  to recover: git commit -eF .git/COMMIT_EDITMSG
```

### `snag push`

Determines the unpushed commit range (`@{upstream}..HEAD`, or `HEAD` if no
upstream is set). For each commit, checks both the message and the full diff.

This is the safety net — anything that slipped past the per-commit hooks gets
caught before it reaches the remote.

```
$ snag push
snag: 4 patterns checked against 3 commits
```

```
$ snag push
snag: match "hack" in diff of a1b2c3d4
```

### Flags

```
--blocklist PATH    # override .blocklist location (default: repo root)
--quiet             # suppress informational output (trailer count, commit summary)
--version           # print version and exit
```

No config files. No environment variables. No YAML. The `.blocklist` file is
the entire configuration surface for the CLI.

## `.blocklist` file format

```
# Patterns to deny from commits (case-insensitive substring match)
# One per line. Blank lines and comments (#) are ignored.
TODO
HACK
DO NOT MERGE
fixme
WIP
```

Each repo carries its own `.blocklist` with whatever patterns make sense for
that project. snag ships no default patterns — that's a policy decision, not a
tool decision.

## Hook runner compatibility

The snag CLI is hook-runner-agnostic. The recipes target lefthook, but the CLI
works anywhere:

### lefthook (via recipes)

```yaml
pre-commit:
  commands:
    blocklist:
      run: snag diff

commit-msg:
  commands:
    blocklist:
      run: snag msg {1}

pre-push:
  commands:
    blocklist:
      run: snag push
```

### husky (package.json)

```json
{
  "husky": {
    "hooks": {
      "pre-commit": "snag diff",
      "commit-msg": "snag msg $HUSKY_GIT_PARAMS",
      "pre-push": "snag push"
    }
  }
}
```

### Raw githooks

```bash
#!/bin/sh
# .git/hooks/pre-commit
snag diff
```

## "Did I forget to set up hooks?"

If you use direnv, add a canary check to `.envrc` (or to
`~/.config/direnv/direnvrc` to apply globally):

```bash
if [ -f .blocklist ] && ! [ -f lefthook.yml ]; then
  printf '\033[33m!! %s has a .blocklist but no lefthook.yml\033[0m\n' \
    "$(basename "$PWD")"
elif [ -f lefthook.yml ] && \
     ! grep -q lefthook .git/hooks/pre-commit 2>/dev/null; then
  printf '\033[33m!! lefthook hooks not installed — run: lefthook install\033[0m\n'
fi
```

Every time you `cd` into a repo, direnv tells you if hooks aren't wired up.

## Distribution

| Channel | Command |
|---|---|
| `go install` | `go install github.com/dpritchett/snag@latest` |
| Homebrew | Formula or tap (optional, add if there's demand) |
| mise | `[tools]` entry: `"go:github.com/dpritchett/snag" = "latest"` |
| Binary releases | GoReleaser via GitHub Actions. Covers non-Go users. |

Repos that only use recipes without the blocklist check don't need the snag
binary at all — just point lefthook remotes at the repo.

## Project structure

```
snag/
  main.go              # cobra root + three subcommands
  blocklist.go         # loadBlocklist, matchesBlocklist, isTrailerLine
  blocklist_test.go    # table-driven tests
  git.go               # StagedDiff(), CommitMessage(), UnpushedCommits()
  git_test.go          # tests against temp git repos
  .goreleaser.yml      # release automation
  recipes/
    lefthook-blocklist.yml    # snag diff/msg/push wiring
    lefthook-gitleaks.yml     # secret scanning via gitleaks
    lefthook-go.yml           # go fmt, vet, test
    lefthook-shellcheck.yml   # shellcheck on staged scripts
```

Flat layout. No `internal/`, no `pkg/`, no `cmd/` hierarchy until the project
earns it.

The `recipes/` directory ships lefthook fragments alongside the CLI. One repo
to clone, one place to find examples. Recipes that don't call snag (gitleaks,
shellcheck, Go checks) still belong here — the value is in the curated
collection, not just the CLI.

## Testing strategy

**Unit tests (no git required):**

- `loadBlocklist` — comments, blank lines, case normalization, missing file
  returns nil
- `matchesBlocklist` — match, no match, empty patterns, empty input
- `isTrailerLine` — valid trailers, non-trailers, edge cases (no colon, spaces
  in key, leading whitespace)
- Trailer stripping — message with mixed trailer/body matches, only trailers
  stripped, body match still rejects

**Integration tests (temp git repos):**

- `diff` subcommand with staged changes containing a match
- `msg` subcommand rewriting a message file in place
- `push` subcommand with multiple unpushed commits, match in second commit's
  diff
- Clean runs (exit 0) for all three subcommands
- Missing `.blocklist` file — all three subcommands exit 0 silently

## Success criteria

- Adding a full hook policy to a new repo is a one-file commit: `lefthook.yml`
  pointing at snag's recipes. Add a `.blocklist` if you want content checks.
- A new recipe can be added to the collection without touching the Go CLI
- Each recipe is self-contained — no recipe depends on another recipe
- The snag CLI stays under 300 lines of Go. If it grows past that, something
  has gone wrong with the scope.
- Trailer stripping handles unwanted trailers automatically without rejecting
  the commit
- Pre-push catch-all prevents policy violations from reaching the remote
- Installable in one command with no environment setup
