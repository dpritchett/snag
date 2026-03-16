package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

// newSafeRenderer creates a lipgloss renderer that skips terminal
// auto-detection (OSC queries) when stdin is not a TTY.
// The OSC 11 probe writes to the output but reads the response from stdin,
// so stdin is what determines whether auto-detection can succeed.
// Inside lefthook's pty the output writer IS a terminal (pty slave),
// but stdin is a pipe — checking stdin catches that case.
func newSafeRenderer(w *os.File) *lipgloss.Renderer {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return lipgloss.NewRenderer(w, termenv.WithProfile(termenv.Ascii))
	}
	return lipgloss.NewRenderer(w)
}

var renderer = newSafeRenderer(os.Stderr)
var stdoutRenderer = newSafeRenderer(os.Stdout)

func init() {
	// Pin the default renderer when stdin isn't a TTY.
	// huh (used by `snag install`) calls AdaptiveColor / HasDarkBackground
	// on lipgloss.DefaultRenderer(), which triggers the same OSC probe.
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		r := lipgloss.DefaultRenderer()
		r.SetColorProfile(termenv.ANSI)
		r.SetHasDarkBackground(true)
	}
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
