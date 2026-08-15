package main

import (
	"fmt"
	"os"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/myusuf1098/ai-proxy-centranity/internal/config"
	"github.com/myusuf1098/ai-proxy-centranity/internal/tui"
)

func main() {
	apiURL := "http://127.0.0.1:8088"
	if val, ok := os.LookupEnv("PG_API_BASE_URL"); ok {
		apiURL = val
	} else if cfg, err := config.Load(); err == nil {
		apiURL = fmt.Sprintf("http://127.0.0.1:%d", cfg.Server.Port)
	}

	adminToken := os.Getenv("PG_ADMIN_TOKEN")

	model := tui.NewModelWithToken(apiURL, adminToken)
	p := tea.NewProgram(model, tea.WithAltScreen())

	if _, err := p.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}
}
