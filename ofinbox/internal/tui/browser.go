package tui

import (
	"os/exec"
	"runtime"

	tea "github.com/charmbracelet/bubbletea"
)

// openURL launches a URL in the default browser. A package var so tests can
// stub it out.
var openURL = func(url string) error {
	if runtime.GOOS == "darwin" {
		return exec.Command("open", url).Run()
	}
	return exec.Command("xdg-open", url).Run()
}

// openedMsg reports the result of an openURLCmd.
type openedMsg struct {
	url string
	err error
}

func openURLCmd(url string) tea.Cmd {
	return func() tea.Msg {
		return openedMsg{url: url, err: openURL(url)}
	}
}
