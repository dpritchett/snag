# AGENTS.md

This file provides guidance to AI coding agents working with code in this repository.

## What is snag?

A composable git hook policy kit written in Go. It enforces content policies (via `snag.toml`) across six git hook phases: pre-commit (`check diff`), commit-msg (`check msg`), pre-push (`check push`), post-checkout (`check checkout`), prepare-commit-msg (`check prepare`), and pre-rebase (`check rebase`). It ships both a Go CLI and reusable lefthook recipe files in `recipes/`.

Design philosophy: minimal configuration surface. `snag.toml` is the primary config format — it's version-controlled team policy (like `lefthook.yml`). snag ships no default patterns — that's a policy decision, not a tool decision.

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
| `hooks.go` | `Hook` struct + `hooks` registry slice. Each entry carries the hook's name, cobra metadata, `RunE` check, and `TestFn` scenario. Adding a hook means adding one struct literal — the compiler enforces that every hook has a test |
| `main.go` | Cobra CLI scaffolding, `check` parent command (subcommands generated from `hooks` registry), `install` command, persistent flags (`--quiet`), version detection via `runtime/debug.BuildInfo`. Cobra auto-provides `completion` subcommand for fish/bash/zsh |
| `config.go` | Structured config: `snagTOML`/`BlockConfig` types, `loadSnagTOML`, `walkConfig` (walks up from CWD to root for `snag.toml`), `resolveBlockConfig` (per-hook pattern resolution with all sources), `PushPatterns`/`HasAnyPatterns` helpers |
| `patterns.go` | Core pattern primitives: `matchesPattern`, `isTrailerLine`, `deduplicatePatterns`, `stripDiffNoise`, `stripDiffMeta`, `isDiffMeta` |
| `diff.go` | Pre-commit: runs `git diff --staged`, checks output against patterns |
| `msg.go` | Commit-msg: two-pass — (1) silently removes trailer lines (e.g. `Generated-by`) matching block patterns so the commit proceeds without them, then (2) rejects the commit if the remaining body matches. Trailers are stripped, body text is blocked |
| `push.go` | Pre-push: scans commit messages AND diffs for all unpushed commits (`@{upstream}..HEAD`) |
| `checkout.go` | Post-checkout: warns when a repo has a snag config (`snag.toml`) but snag hooks aren't installed. Checks lefthook configs for snag remote and `.git/hooks/` for snag scripts |
| `prepare.go` | Prepare-commit-msg: checks auto-generated commit messages (merge, template, amend) against patterns. Skips `-m` messages (commit-msg handles those) |
| `rebase.go` | Pre-rebase: blocks rebase of protected branches (main, master by default). Override via `SNAG_PROTECTED_BRANCHES` env var |
| `audit.go` | `snag audit` — scans git history for policy violations. Checks commit messages against `bc.Msg` and diffs against `bc.Diff`. Reports all matches grouped by commit. Supports `--limit N` and explicit revision ranges |
| `shell.go` | `snag shell <bash\|fish\|zsh>` — emits shell-specific hooks that warn on `cd` into repos where snag config exists but hooks aren't installed. Uses a `shellHook` interface with per-stage methods; `renderHook()` assembles them. Adding a shell or stage is compiler-enforced |
| `install_hooks.go` | `snag install` — adds/updates snag remote in lefthook config. Reads YAML to understand structure, writes via string append/replace to preserve formatting |

**Data flow:** git hook → `snag check <subcommand>` → `resolveBlockConfig` (walk up for `snag.toml` files + env vars) → shell out to git → per-hook pattern match → exit code (0 = clean, 1 = violation).

**Config resolution order (`resolveBlockConfig`):**
1. `walkConfig` from CWD to root. Both `snag.toml` and `snag-local.toml` are checked at each level and merged additively up the tree. `snag-local.toml` only adds patterns — it never overrides `snag.toml`.
2. `SNAG_PROTECTED_BRANCHES` env var → always merges into Branch
3. Default protected branches `["main", "master"]` → only when Branch is still empty
4. Lowercase Diff/Msg/Push; preserve Branch case; deduplicate all lists
5. `SNAG_IGNORE` env var → removes patterns or clears entire phases after deduplication. Comma-separated entries: `<phase>` clears the phase, `<phase>:<pattern>` removes one pattern. Phases: `diff`, `msg`, `push`, `branch`

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

- **Policy engine must stay hook-runner-agnostic.** The check commands (`diff.go`, `msg.go`, `push.go`, `prepare.go`, `rebase.go`), config (`config.go`, `snag.toml`), and pattern matching (`patterns.go`) must never reference lefthook, husky, pre-commit, or any other hook runner. Runner-specific code is confined to `install_hooks.go` (installation), `checkout.go` (detection), and `shell.go` (nudge hooks). If a new file needs runner awareness, that's a design smell. See #41.
- `snag.toml` is version-controlled team policy
- `snag-local.toml` is gitignored, personal/sensitive patterns — additive overlay alongside `snag.toml` at each directory level
- Sensitive patterns belong in `snag-local.toml`, not in committed config
- `resolveBlockConfig` centralizes all per-hook pattern resolution; subcommands use the appropriate field (`bc.Diff`, `bc.Msg`, `bc.PushPatterns()`, `bc.Branch`)
- `install` must never mangle existing YAML — parse to read, string ops to write
- Version comes from ldflags (GoReleaser) or `runtime/debug.BuildInfo` (`go install`)
