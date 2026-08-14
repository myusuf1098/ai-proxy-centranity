package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/myusuf1098/ai-proxy-centranity/internal/config"
	"github.com/myusuf1098/ai-proxy-centranity/internal/tui"
)

func main() {
	cfg, err := config.Load()
	apiURL := "http://127.0.0.1:8088"
	if err == nil {
		apiURL = fmt.Sprintf("http://127.0.0.1:%d", cfg.Server.Port)
	}

	model := tui.NewModel(apiURL)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
