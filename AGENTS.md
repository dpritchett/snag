# AGENTS.md

This file provides guidance to AI coding agents working with code in this repository.

## What is snag?

A composable git hook policy kit written in Go. It enforces content policies (via a `.blocklist` file) across three git hook phases: pre-commit (`diff`), commit-msg (`msg`), and pre-push (`push`). It ships both a Go CLI and reusable lefthook recipe files in `recipes/`.

Design philosophy: no config files, no env vars, no YAML beyond the blocklist. The `.blocklist` file is the entire configuration surface. snag ships no default patterns — that's a policy decision, not a tool decision.

## Build & Test Commands

```bash
go build -o snag .       # build binary
go test ./...            # run all tests
go test -v ./...         # verbose test output
go test -run TestName .  # run a single test
go vet ./...             # static analysis (also runs in CI)
```

No Makefile — standard Go toolchain only. CI runs `vet → test → build`.

## Release

GoReleaser handles cross-platform builds (linux/darwin/windows, amd64/arm64). Version is injected via ldflags (`-X main.Version={{.Version}}`). To release: push a `v*` tag and the `release.yml` workflow handles the rest. No files need updating — there's no hardcoded version anywhere.

## Architecture

All production code lives in the package `main` at the repo root (~500 lines total).

| File | Purpose |
|------|---------|
| `main.go` | Cobra CLI scaffolding, three subcommands (`diff`, `msg`, `push`), persistent flags (`--blocklist`, `--quiet`) |
| `blocklist.go` | Core policy engine: `loadBlocklist`, `matchesBlocklist` (case-insensitive substring), `isTrailerLine` (git trailer detection) |
| `diff.go` | Pre-commit: runs `git diff --staged`, checks output against blocklist |
| `msg.go` | Commit-msg: two-pass — strip matching trailer lines then check remaining body |
| `push.go` | Pre-push: scans commit messages AND diffs for all unpushed commits (`@{upstream}..HEAD`) |

**Data flow:** git hook → snag subcommand → load `.blocklist` → shell out to git → pattern match → exit code (0 = clean, 1 = violation).

## Testing Patterns

Tests are table-driven and create real temporary git repos using helpers defined in the test files:
- `initGitRepo(t)` — temp dir with `git init`
- `stageFile(t, dir, name, content)` — create + `git add`
- `commitFile(t, dir, name, content, message)` — full commit
- `initialCommit(t, dir)` — seed commit for diff baselines

## Lefthook Recipes

`recipes/` contains composable lefthook configs consumed via lefthook's `remotes` feature. These are standalone YAML files meant for external repos to reference — they are not used by snag's own CI.
