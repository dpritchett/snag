package main

import (
	"fmt"
	"os"

	"github.com/charmbracelet/lipgloss"
	"golang.org/x/term"
)

var renderer = lipgloss.NewRenderer(os.Stderr)

var (
	errorStyle = renderer.NewStyle().Bold(true).Foreground(lipgloss.Color("9")) // red
	warnStyle  = renderer.NewStyle().Foreground(lipgloss.Color("11"))           // yellow
	infoStyle  = renderer.NewStyle().Foreground(lipgloss.Color("10"))           // green
	hintStyle  = renderer.NewStyle().Foreground(lipgloss.Color("8"))            // dim/gray
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
