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

func TestTUINavbarWrapsSixPerRow(t *testing.T) {
	m := tui.NewModel("http://127.0.0.1:8088")
	updated, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	m = updated.(tui.Model)

	view := m.View()
	lines := strings.Split(view, "\n")

	var row1, row2 string
	for _, ln := range lines {
		if strings.Contains(ln, "1:OVERVIEW") {
			row1 = ln
		}
		if strings.Contains(ln, "7:ROUTING") {
			row2 = ln
		}
	}
	if row1 == "" || row2 == "" {
		t.Fatalf("expected two navbar rows, row1=%q row2=%q", row1, row2)
	}
	// Row 1 holds tabs 1-6, row 2 holds tabs 7-12.
	if !strings.Contains(row1, "1:OVERVIEW") || !strings.Contains(row1, "6:POLICIES") {
		t.Errorf("row 1 should hold tabs 1-6, got: %q", row1)
	}
	if strings.Contains(row1, "ROUTING") {
		t.Errorf("row 1 should not contain tab 7 (ROUTING), got: %q", row1)
	}
	if !strings.Contains(row2, "7:ROUTING") || !strings.Contains(row2, "12:SETTINGS") {
		t.Errorf("row 2 should hold tabs 7-12, got: %q", row2)
	}
}
