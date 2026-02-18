# Lefthook Config Merge Order

Understanding lefthook's config merge behavior is critical for snag's
`install` command. This document captures what we've learned the hard way
so future contributors don't have to re-discover it.

## The merge chain

Lefthook v2 merges configs in this order (last writer wins):

```
main config (lefthook.yml)
    ↓ overridden by
main config remotes
    ↓ overridden by
local config (lefthook-local.yml)
    ↓ overridden by
local config remotes
```

Each layer can add new hook types and commands. When two layers define
the same top-level hook type, the later layer's definition replaces the
earlier one entirely — it's a **replace**, not a deep merge.

## Why this matters for snag

Snag ships a remote recipe (`recipes/lefthook-snag-filter.yml`) that
defines commands for six hook types: `pre-commit`, `commit-msg`,
`pre-push`, `post-checkout`, `prepare-commit-msg`, and `pre-rebase`.

Lefthook only creates `.git/hooks/<type>` scripts for hook types it sees
in the merged config. If a hook type is only defined in a remote recipe
and nowhere in the local configs, `lefthook install` won't create a hook
script for it, and the remote recipe's commands for that type never fire.

To fix this, `snag install` writes empty "stubs" for each recipe hook
type (e.g. `commit-msg:` with no value) so that `lefthook install`
creates the corresponding `.git/hooks/` scripts.

## The clobber trap

Here's the catch: **stubs must go in the main config, never in the local
config.**

If you put an empty `commit-msg:` stub in `lefthook-local.yml`, the
local config's empty definition wins over the remote recipe's
`commit-msg: { commands: { snag-filter: ... } }`. The remote recipe's
commands are silently dropped, and `lefthook run commit-msg` reports
`(skip) empty`.

When the stub lives in `lefthook.yml` (the main config), the remote
recipe's commands override the empty stub during merge, and everything
works as expected.

## The snag install strategy

```
Remote block → goes to whichever config the user chose (shared or local)
Hook stubs   → always go to the main config (lefthook.yml)
```

This means `snag install --local` touches **two** files:

1. `lefthook-local.yml` — gets the `remotes:` block pointing at the snag recipe
2. `lefthook.yml` — gets empty hook-type stubs so `lefthook install` activates them

The stubs are harmless in the main config — they're just empty YAML keys
that tell lefthook "install a hook for this type." The remote recipe
fills in the actual commands during merge.

## Verifying the merge

Use `lefthook dump` to inspect the final merged config. Every hook type
from the snag recipe should show its `commands:` block, not `{}`.

```bash
lefthook dump | yq '.commit-msg'
# Should show:
#   commands:
#     snag-filter:
#       run: snag check msg {1}
#       fail_text: ...
```

If you see `{}` or `null` for a hook type, the stubs are clobbering the
remote recipe — check which config file they ended up in.

## References

- [#43](https://github.com/dpritchett/snag/issues/43) — lefthook doesn't install hooks for types only defined in remote recipe
- [#44](https://github.com/dpritchett/snag/issues/44) — empty stubs in local config clobber remote recipe commands
