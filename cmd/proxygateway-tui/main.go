package main

import (
	"fmt"
	"os"

	"github.com/myusuf1098/ai-proxy-centranity/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error loading configuration: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("=== ProxyGateway Enterprise TUI ===\n")
	fmt.Printf("Version: 2.0 (Phase 1 Baseline)\n")
	fmt.Printf("Target Gateway API: http://127.0.0.1:%d\n", cfg.Server.Port)
	fmt.Printf("Press Ctrl+C to exit.\n")
}
