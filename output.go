package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/termenv"
	"golang.org/x/term"
)

// newSafeRenderer creates a lipgloss renderer that skips terminal
// auto-detection (OSC queries) when the writer is not a TTY.
// Without this, termenv blocks ~5 s per invocation inside git hooks
// waiting for a terminal response that never comes.
func newSafeRenderer(w *os.File) *lipgloss.Renderer {
	if !term.IsTerminal(int(w.Fd())) {
		return lipgloss.NewRenderer(w, termenv.WithProfile(termenv.Ascii))
	}
	return lipgloss.NewRenderer(w)
}

var renderer = newSafeRenderer(os.Stderr)
var stdoutRenderer = newSafeRenderer(os.Stdout)

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
