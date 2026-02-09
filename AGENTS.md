# AGENTS.md

This file provides guidance to AI coding agents working with code in this repository.

## What is snag?

A composable git hook policy kit written in Go. It enforces content policies (via a `.blocklist` file) across three git hook phases: pre-commit (`diff`), commit-msg (`msg`), and pre-push (`push`). It ships both a Go CLI and reusable lefthook recipe files in `recipes/`.

Design philosophy: minimal configuration surface. The `.blocklist` file and optional `SNAG_BLOCKLIST` env var are the entire policy interface. snag ships no default patterns — that's a policy decision, not a tool decision.

## Build & Test Commands

```bash
go build -o snag .       # build binary
go test ./...            # run all tests
go test -v ./...         # verbose test output
go test -run TestName .  # run a single test
go vet ./...             # static analysis (also runs in CI)
```

No Makefile — standard Go toolchain only. CI runs `gofmt check → vet → test → build`.

## Release

GoReleaser handles cross-platform builds (linux/darwin/windows, amd64/arm64). Version is injected via ldflags (`-X main.Version={{.Version}}`). To release: push a `v*` tag and the `release.yml` workflow handles the rest. No files need updating — there's no hardcoded version anywhere.

## Architecture

All production code lives in the package `main` at the repo root.

| File | Purpose |
|------|---------|
| `main.go` | Cobra CLI scaffolding, subcommands, persistent flags (`--blocklist`, `--quiet`), version detection via `runtime/debug.BuildInfo` |
| `blocklist.go` | Core policy engine: `loadBlocklist`, `matchesBlocklist`, `isTrailerLine`, plus `resolvePatterns` (directory walk + env var merge + dedup) |
| `diff.go` | Pre-commit: runs `git diff --staged`, checks output against blocklist |
| `msg.go` | Commit-msg: two-pass — strip matching trailer lines then check remaining body |
| `push.go` | Pre-push: scans commit messages AND diffs for all unpushed commits (`@{upstream}..HEAD`) |
| `install_hooks.go` | Adds/updates snag remote in lefthook config. Reads YAML to understand structure, writes via string append/replace to preserve formatting |

**Data flow:** git hook → snag subcommand → `resolvePatterns` (walk up for `.blocklist` files + `SNAG_BLOCKLIST` env var) → shell out to git → pattern match → exit code (0 = clean, 1 = violation).

**Blocklist resolution order:**
1. If `--blocklist` flag is explicitly passed → use only that file
2. Otherwise → walk from CWD to filesystem root, merge all `.blocklist` files found
3. Always merge `SNAG_BLOCKLIST` env var patterns on top
4. Deduplicate

## Testing Patterns

Tests are table-driven and create real temporary git repos using helpers defined in the test files:
- `initGitRepo(t)` — temp dir with `git init`
- `stageFile(t, dir, name, content)` — create + `git add`
- `commitFile(t, dir, name, content, message)` — full commit
- `initialCommit(t, dir)` — seed commit for diff baselines

## Lefthook Recipes

`recipes/` contains composable lefthook configs consumed via lefthook's `remotes` feature. These are standalone YAML files meant for external repos to reference. Recipes include `fail_text` for user-friendly error messages. The `lefthook-go.yml` recipe uses `stage_fixed: true` on `go-fmt`.

snag's own repo uses `lefthook.yml` to dogfood via `go run .`.

## Key Design Decisions

- `.blocklist` files should be gitignored — they contain sensitive patterns by definition
- `install-hooks` must never mangle existing YAML — parse to read, string ops to write
- `resolvePatterns` centralizes all blocklist resolution; subcommands call it identically
- Version comes from ldflags (GoReleaser) or `runtime/debug.BuildInfo` (`go install`)
