# snag

A composable git hook policy kit.

[![CI](https://github.com/dpritchett/snag/actions/workflows/ci.yml/badge.svg)](https://github.com/dpritchett/snag/actions/workflows/ci.yml)

Every repo needs git hooks — formatting checks, secret scanning, commit message
standards. Most teams solve this one repo at a time: a shell script here, a
husky config there, maybe a pre-commit framework if someone sets it up.

You could write a shell one-liner for each check. Then you need it in three hook
phases, each with slightly different behavior, across 40 repos. The one-liner is
now a project.

snag is that project, kept small on purpose.

## What you get

- **A curated set of lefthook recipe files** covering common checks (content
  policy, secret scanning, formatting, linting). Each recipe is a standalone
  fragment any repo can pull in via
  [lefthook remotes](https://github.com/evilmartians/lefthook/blob/master/docs/configuration.md#remotes).

- **A small Go CLI** (`snag`) for per-repo content policy enforcement via a
  `.blocklist` file. For checks where no good off-the-shelf tool exists yet.

## Install

### go install

```bash
go install github.com/dpritchett/snag@latest
```

### mise

```bash
mise use -g go:github.com/dpritchett/snag@latest
```

### Binary releases

Pre-built binaries are available on the
[Releases](https://github.com/dpritchett/snag/releases) page (via GoReleaser).

### Recipe-only usage

If you only want the lefthook recipes (gitleaks, shellcheck, Go checks) and
don't need the `.blocklist` CLI, you don't need to install the binary at all —
just point your lefthook remotes at this repo.

## Quick start

If you already have a lefthook config, the fastest path is:

```bash
snag install         # adds the snag remote to your lefthook config
lefthook install     # activates the hooks
```

Or add the remote manually:

```yaml
# lefthook.yml
remotes:
  - git_url: https://github.com/dpritchett/snag.git
    ref: v0.4.3
    configs:
      - recipes/lefthook-blocklist.yml
      - recipes/lefthook-gitleaks.yml
```

Add a `.blocklist` to your repo root (or a parent directory):

```
# Patterns to deny (case-insensitive substring match)
TODO
HACK
DO NOT MERGE
fixme
WIP
```

Run `lefthook install` and you're set.

## Recipes

| Recipe | Hook phase(s) | What it does | Requires |
|---|---|---|---|
| `lefthook-blocklist.yml` | pre-commit, commit-msg, pre-push | Content policy via `.blocklist` | `snag` CLI |
| `lefthook-gitleaks.yml` | pre-commit | Secret scanning | `gitleaks` |
| `lefthook-go.yml` | pre-commit | `go fmt`, `go vet`, `go test` | Go toolchain |
| `lefthook-shellcheck.yml` | pre-commit | Lint staged shell scripts | `shellcheck` |

Each recipe is independent. A repo can pull one or all of them. Pin `ref` to a
tag (e.g. `ref: v1.0.0`) for stability, or use `main` to track latest.

**Local overrides.** Any repo can add a `lefthook-local.yml` to extend or
override what the remote recipes provide. lefthook merges local config on top of
remote config automatically.

### More lefthook examples

```yaml
# JS project — blocklist + gitleaks only
remotes:
  - git_url: https://github.com/dpritchett/snag.git
    ref: main
    configs:
      - recipes/lefthook-blocklist.yml
      - recipes/lefthook-gitleaks.yml
```

```yaml
# Go project — full stack
remotes:
  - git_url: https://github.com/dpritchett/snag.git
    ref: main
    configs:
      - recipes/lefthook-blocklist.yml
      - recipes/lefthook-gitleaks.yml
      - recipes/lefthook-go.yml
```

## CLI usage

```
snag check diff        # pre-commit: scan staged changes
snag check msg FILE    # commit-msg: clean trailers, reject body matches
snag check push        # pre-push: scan all unpushed commits
snag install           # add/update snag remote in lefthook config
snag version           # print version and exit
```

All three perform case-insensitive substring matching and exit 0 (clean) or 1
(match found, with a human-readable error).

By default, snag walks up from the current directory to the filesystem root,
loading every `.blocklist` it finds and merging all patterns. A parent directory
blocklist protects every repo underneath it — no per-repo setup required.

If `--blocklist` is passed explicitly, only that file is used (no walk).

### `snag install`

Adds or updates the snag remote in your lefthook config. Finds
`lefthook.yml`, `lefthook.yaml`, `.lefthook.yml`, or `.lefthook.yaml`.
Pins the `ref` to the running binary's version.

```
$ snag install
Added snag v0.4.3 remote to lefthook.yaml
Run `lefthook install` to activate.
```

If a snag remote already exists at an older version, it updates the ref
in place without touching the rest of the file. If it's already current,
it does nothing.

### `snag check diff`

```
$ snag check diff
snag: match "do not merge" in staged diff
```

### `snag check msg`

Two-pass approach: first strips git trailer lines (`Key: Value`) matching the
blocklist, rewriting the file in place. Then checks the remaining message body.

```
$ snag check msg .git/COMMIT_EDITMSG
snag: removed 1 trailer line(s)
```

```
$ snag check msg .git/COMMIT_EDITMSG
snag: match "fixme" in commit message
  to recover: git commit -eF .git/COMMIT_EDITMSG
```

### `snag check push`

Scans all unpushed commits — both messages and diffs. The safety net for
anything that slipped past per-commit hooks.

```
$ snag check push
snag: 4 patterns checked against 3 commits
```

### Flags

```
--blocklist PATH    # use only this blocklist file (disables directory walk)
--quiet             # suppress informational output
--version           # print version and exit
```

### Environment variables

```
SNAG_BLOCKLIST      # extra patterns, colon or newline separated, always merged
```

```bash
# single line — colon-separated
export SNAG_BLOCKLIST="secretword:codename:do not merge"

# multi-line also works (same format as a .blocklist file)
export SNAG_BLOCKLIST="secretword
codename
do not merge"
```

`SNAG_BLOCKLIST` is additive — its patterns are merged on top of whatever the
directory walk (or explicit `--blocklist`) found. Handy for CI, shell rc files,
direnv, or mise.

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

Each repo can carry its own `.blocklist` with whatever patterns make sense for
that project. You can also place a `.blocklist` in a parent directory to cover
every repo underneath it:

```
~/projects/acme/.blocklist          ← "codename", "customername"
~/projects/acme/api/.blocklist      ← additional repo-specific patterns (optional)
~/projects/acme/api/service/        ← no .blocklist, still protected by both above
```

snag ships no default patterns — that's a policy decision, not a tool decision.

## Hook runner examples

The snag CLI is hook-runner-agnostic. The recipes target lefthook, but the CLI
works anywhere:

### lefthook

```yaml
pre-commit:
  commands:
    blocklist:
      run: snag check diff

commit-msg:
  commands:
    blocklist:
      run: snag check msg {1}

pre-push:
  commands:
    blocklist:
      run: snag check push
```

### husky

```json
{
  "husky": {
    "hooks": {
      "pre-commit": "snag check diff",
      "commit-msg": "snag check msg $HUSKY_GIT_PARAMS",
      "pre-push": "snag check push"
    }
  }
}
```

### pre-commit (Python framework)

```yaml
# .pre-commit-config.yaml
repos:
  - repo: local
    hooks:
      - id: snag-diff
        name: snag blocklist
        entry: snag check diff
        language: system
        stages: [pre-commit]
      - id: snag-msg
        name: snag commit message
        entry: snag check msg
        language: system
        stages: [commit-msg]
      - id: snag-push
        name: snag push check
        entry: snag check push
        language: system
        stages: [pre-push]
```

### Raw githooks

```bash
#!/bin/sh
# .git/hooks/pre-commit
snag check diff
```

## direnv canary

If you use direnv, add a canary check to `.envrc` so you never forget to wire
up hooks:

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

## License

MIT — see [LICENSE](LICENSE).
