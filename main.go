package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/jr/lazyhosts/internal/hostctl"
	"github.com/jr/lazyhosts/internal/model"
)

func main() {
	if !hostctl.IsInstalled() {
		fmt.Fprintln(os.Stderr, "Error: hostctl is not installed.")
		fmt.Fprintln(os.Stderr, "Install it from: https://github.com/guumaster/hostctl")
		if hostctl.NeedsSudo() {
			fmt.Fprintln(os.Stderr, "  brew install guumaster/tap/hostctl")
		} else {
			fmt.Fprintln(os.Stderr, "  scoop install hostctl")
		}
		os.Exit(1)
	}

	// Acquire elevated privileges before TUI starts.
	if hostctl.NeedsSudo() {
		fmt.Println("lazyhosts needs sudo to modify /etc/hosts.")
		if err := hostctl.AcquireSudo(); err != nil {
			fmt.Fprintln(os.Stderr, "Warning: failed to acquire sudo. Some features will be unavailable.")
		}
	} else if !hostctl.HasElevatedPrivilege() {
		fmt.Fprintln(os.Stderr, "Warning: not running as Administrator. Some features will be unavailable.")
		fmt.Fprintln(os.Stderr, "Please right-click your terminal and select \"Run as administrator\".")
	}

	// Keep sudo alive in the background while TUI is running (no-op on Windows).
	stopKeepalive := hostctl.SudoKeepalive()
	defer stopKeepalive()

	p := tea.NewProgram(
		model.New(),
		tea.WithAltScreen(),
		tea.WithMouseCellMotion(),
	)

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running lazyhosts: %v\n", err)
		os.Exit(1)
	}
}
