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
		fmt.Fprintln(os.Stderr, "  brew install guumaster/tap/hostctl")
		os.Exit(1)
	}

	// Always acquire sudo credentials before TUI starts.
	// This ensures a fresh credential cache even if a prior one is about to expire.
	fmt.Println("lazyhosts needs sudo to modify /etc/hosts.")
	if err := hostctl.AcquireSudo(); err != nil {
		fmt.Fprintln(os.Stderr, "Warning: failed to acquire sudo. Some features will be unavailable.")
	}

	// Keep sudo alive in the background while TUI is running.
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
