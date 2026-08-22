// ofinbox is a keyboard-driven TUI for processing the OmniFocus inbox:
// step through inbox items and complete, drop, file, tag, flag, or schedule
// each one with a single keystroke. With -serve it instead runs an HTTP
// server exposing the same operations to the phone frontend.
package main

import (
	"flag"
	"fmt"
	"net/http"
	"os"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/packetslave/experiments/ofinbox/internal/omnifocus"
	"github.com/packetslave/experiments/ofinbox/internal/server"
	"github.com/packetslave/experiments/ofinbox/internal/tui"
)

func main() {
	demo := flag.Bool("demo", false, "use built-in sample data instead of OmniFocus")
	serve := flag.Bool("serve", false, "run the HTTP server for the phone frontend instead of the TUI")
	addr := flag.String("addr", "127.0.0.1:4747", "listen address for -serve")
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

	if *serve {
		fmt.Printf("ofinbox serving on http://%s\n", *addr)
		if err := http.ListenAndServe(*addr, server.New(client)); err != nil {
			fmt.Fprintln(os.Stderr, "error:", err)
			os.Exit(1)
		}
		return
	}

	p := tea.NewProgram(tui.New(client), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}
