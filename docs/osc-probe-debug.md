# OSC probe debug trace — issue #46

## Problem

`snag version` hangs ~5 s inside lefthook's pty. The hang comes from
termenv's OSC 11 background-color probe (`termStatusReport(11)`), which
writes `\x1b]11;?\x1b\\` and waits 5 s for a response that never arrives
in lefthook's pty.

## What we've ruled out

| Hypothesis | Result |
|---|---|
| Check stdin TTY instead of writer | Doesn't help — lefthook pty gives all 3 fds as TTY |
| Pin `lipgloss.Renderer.SetColorProfile` | Prevents lipgloss from calling `EnvColorProfile()` — good, but OSC still fires |
| Pin `lipgloss.Renderer.SetHasDarkBackground` | Prevents lipgloss from calling `termenv.Output.HasDarkBackground()` — good, but OSC still fires |
| Pin `lipgloss.DefaultRenderer()` in `init()` | Same — prevents lipgloss-layer calls, but something else triggers the probe |

## Current theory

The termenv **package-level** default output (`var output = NewOutput(os.Stdout)`)
is not pinned. Something calls `termenv.HasDarkBackground()` or
`termenv.BackgroundColor()` (the package-level functions) which go straight
to the un-pinned termenv output and trigger the OSC probe.

Pinning the lipgloss default renderer does NOT pin the underlying termenv
default output.

## Root cause

`bubbletea@v1.3.10/tea_init.go:21`:

```go
func init() {
    _ = lipgloss.HasDarkBackground()
}
```

Bubbletea's `init()` force-calls `lipgloss.HasDarkBackground()` at package
load time — before snag's `init()` can pin anything. This triggers the OSC
probe on the **default** lipgloss renderer, which uses the un-pinned termenv
default output (`*os.File` on stdout). The comment says this is a workaround
for bubbletea programs that acquire the terminal before termenv can query it,
and that it will be removed in v2.

## Fix

Pin the termenv default output during **var initialization**, not in `init()`.
Go evaluates package-level vars before any `init()` functions. By replacing
the termenv default output in a var initializer, we beat bubbletea's `init()`
to the punch.
