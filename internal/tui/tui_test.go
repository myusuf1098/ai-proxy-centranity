package tui_test

import (
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/myusuf1098/ai-proxy-centranity/internal/tui"
)

func TestTUIInitialModel(t *testing.T) {
	m := tui.NewModel("http://127.0.0.1:8088")
	if m.ActiveTab() != tui.TabOverview {
		t.Errorf("expected initial tab OVERVIEW, got %v", m.ActiveTab())
	}
}

func TestTUITabNavigation(t *testing.T) {
	m := tui.NewModel("http://127.0.0.1:8088")

	// Press Tab to move to next screen (REQUESTS)
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(tui.Model)
	if m.ActiveTab() != tui.TabRequests {
		t.Errorf("expected tab REQUESTS after Tab key, got %v", m.ActiveTab())
	}

	// Press key '3' to switch to MODELS screen (index 2)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'3'}})
	m = updated.(tui.Model)
	if m.ActiveTab() != tui.TabModels {
		t.Errorf("expected tab MODELS after pressing '3', got %v", m.ActiveTab())
	}
}

func TestTUIRendering(t *testing.T) {
	m := tui.NewModel("http://127.0.0.1:8088")
	// Set window size
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(tui.Model)

	view := m.View()
	if view == "" {
		t.Fatalf("expected non-empty view")
	}

	if !strings.Contains(view, "OVERVIEW") || !strings.Contains(view, "ProxyGateway Enterprise") {
		t.Errorf("expected header and tab bar in rendered view, got: %s", view)
	}

	if !strings.Contains(view, "q: Quit") || !strings.Contains(view, "Tab: Next Tab") {
		t.Errorf("expected shortcuts footer in rendered view")
	}
}
