// ofinbox is a keyboard-driven TUI for processing the OmniFocus inbox:
// step through inbox items and complete, drop, file, tag, flag, or schedule
// each one with a single keystroke.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/packetslave/experiments/ofinbox/internal/omnifocus"
	"github.com/packetslave/experiments/ofinbox/internal/tui"
)

func main() {
	demo := flag.Bool("demo", false, "use built-in sample data instead of OmniFocus")
	flag.Parse()

	var client omnifocus.Client
	switch {
	case *demo:
		client = omnifocus.NewDemoClient()
	case runtime.GOOS != "darwin":
		fmt.Fprintln(os.Stderr, "ofinbox talks to OmniFocus via osascript and needs macOS.")
		fmt.Fprintln(os.Stderr, "On other platforms, try it with sample data: ofinbox -demo")
		os.Exit(1)
	default:
		client = omnifocus.NewOsascriptClient()
	}

	p := tea.NewProgram(tui.New(client), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
