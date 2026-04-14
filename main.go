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
