package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/myusuf1098/ai-proxy-centranity/internal/config"
	"github.com/myusuf1098/ai-proxy-centranity/internal/tui"
)

func main() {
	apiURL := getEnvString("PG_API_BASE_URL", "http://127.0.0.1:8088")
	if apiURL == "http://127.0.0.1:8088" {
		if cfg, err := config.Load(); err == nil {
			apiURL = fmt.Sprintf("http://127.0.0.1:%d", cfg.Server.Port)
		}
	}

	model := tui.NewModel(apiURL)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}

func getEnvString(key, fallback string) string {
	if val := os.Getenv(key); val != "" {
		return val
	}
	return fallback
}
