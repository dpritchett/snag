package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

// newSafeRenderer creates a lipgloss renderer that never triggers
// termenv's OSC 11 background-color probe. That probe hangs ~5 s
// inside lefthook's pty (all fds are TTYs but no terminal emulator
// responds). We still check the writer fd for TTY-ness so pipes
// get plain text and real terminals get ANSI colors.
func newSafeRenderer(w *os.File) *lipgloss.Renderer {
	profile := termenv.Ascii
	if term.IsTerminal(int(w.Fd())) {
		profile = termenv.ANSI
	}
	r := lipgloss.NewRenderer(w, termenv.WithProfile(profile))
	r.SetColorProfile(profile)
	r.SetHasDarkBackground(true)
	return r
}

var renderer = newSafeRenderer(os.Stderr)
var stdoutRenderer = newSafeRenderer(os.Stdout)

func init() {
	// Pin the default lipgloss renderer so any code that calls
	// lipgloss.HasDarkBackground() or uses AdaptiveColor won't
	// trigger termenv's OSC 11 probe.
	r := lipgloss.DefaultRenderer()
	r.SetColorProfile(termenv.ANSI)
	r.SetHasDarkBackground(true)
}

var (
	errorStyle = renderer.NewStyle().Bold(true).Foreground(lipgloss.Color("9")) // red
	warnStyle  = renderer.NewStyle().Foreground(lipgloss.Color("11"))           // yellow
	infoStyle  = renderer.NewStyle().Foreground(lipgloss.Color("10"))           // green
	hintStyle  = renderer.NewStyle().Foreground(lipgloss.Color("8"))            // dim/gray

	// Stdout styles for report output (audit, etc.)
	shaStyle     = stdoutRenderer.NewStyle().Foreground(lipgloss.Color("11")) // yellow
	patternStyle = stdoutRenderer.NewStyle().Bold(true).Foreground(lipgloss.Color("9"))
	dimStyle     = stdoutRenderer.NewStyle().Foreground(lipgloss.Color("8"))
)

func errorf(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	fmt.Fprintln(os.Stderr, errorStyle.Render("snag:")+" "+msg)
}

func warnf(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	fmt.Fprintln(os.Stderr, warnStyle.Render("snag:")+" "+msg)
}

func infof(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	fmt.Fprintln(os.Stderr, infoStyle.Render("snag:")+" "+msg)
}

func hintf(format string, a ...any) {
	msg := fmt.Sprintf(format, a...)
	fmt.Fprintln(os.Stderr, hintStyle.Render("  "+msg))
}

func bell() {
	if term.IsTerminal(int(os.Stderr.Fd())) {
		fmt.Fprint(os.Stderr, "\a")
	}
}
