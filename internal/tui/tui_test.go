package tui_test

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

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

func TestTUIFetchDataSendsAdminToken(t *testing.T) {
	var mu sync.Mutex
	gotAuth := []string{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotAuth = append(gotAuth, r.Header.Get("Authorization"))
		mu.Unlock()
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	m := tui.NewModelWithToken(srv.URL, "secret-token")

	done := make(chan tea.Msg, 1)
	go func() {
		done <- m.Init()()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for fetchData to complete")
	}

	mu.Lock()
	defer mu.Unlock()
	if len(gotAuth) != 6 {
		t.Fatalf("expected 6 API requests, got %d", len(gotAuth))
	}
	for i, auth := range gotAuth {
		if auth != "Bearer secret-token" {
			t.Errorf("request %d: expected Authorization header %q, got %q", i, "Bearer secret-token", auth)
		}
	}
}

func TestTUIPostSendsAdminToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("expected POST, got %s", r.Method)
		}
		if r.Header.Get("Authorization") != "Bearer secret-token" {
			t.Errorf("expected auth header, got %q", r.Header.Get("Authorization"))
		}
		if r.Header.Get("Content-Type") != "application/json" {
			t.Errorf("expected JSON content-type, got %q", r.Header.Get("Content-Type"))
		}
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{"key":"sk-raw","id":"key_1"}`))
	}))
	defer srv.Close()

	m := tui.NewModelWithToken(srv.URL, "secret-token")
	resp, err := m.Do(http.MethodPost, "/api/v1/keys", `{"name":"x"}`)
	if err != nil {
		t.Fatalf("Do returned error: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Errorf("expected 201, got %d", resp.StatusCode)
	}
}

func TestTUIFetchDataWithoutTokenSendsNoAuth(t *testing.T) {
	var mu sync.Mutex
	gotAuth := []string{}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotAuth = append(gotAuth, r.Header.Get("Authorization"))
		mu.Unlock()
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	m := tui.NewModel(srv.URL)

	done := make(chan tea.Msg, 1)
	go func() {
		done <- m.Init()()
	}()

	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for fetchData to complete")
	}

	mu.Lock()
	defer mu.Unlock()
	for i, auth := range gotAuth {
		if auth != "" {
			t.Errorf("request %d: expected no Authorization header, got %q", i, auth)
		}
	}
}

// TestTUIProxyAddSubmitsPost verifies the action wiring: pressing the add
// action key (a) opens a modal form, filling fields + Enter submits, and the
// form's OnSubmit issues POST /api/v1/proxies.
func TestTUIProxyAddSubmitsPost(t *testing.T) {
	var mu sync.Mutex
	var gotMethod, gotPath, gotBody string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		gotMethod, gotPath = r.Method, r.URL.Path
		b, _ := io.ReadAll(r.Body)
		gotBody = string(b)
		mu.Unlock()
		w.WriteHeader(http.StatusCreated)
		w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	m := tui.NewModelWithToken(srv.URL, "token")

	// Go to PROXIES tab, then press 'a' to open the Add Proxy form.
	var updated tea.Model
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("8")})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("a")})
	m = updated.(tui.Model)

	// Fill the four fields: name, host, port, type.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("p1")})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("h1")})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("8080")})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyTab})
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("HTTP")})
	m = updated.(tui.Model)

	// Submit.
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEnter})
	m = updated.(tui.Model)

	mu.Lock()
	defer mu.Unlock()
	if gotMethod != http.MethodPost || gotPath != "/api/v1/proxies" {
		t.Fatalf("expected POST /api/v1/proxies, got %s %s body=%s", gotMethod, gotPath, gotBody)
	}
	if !strings.Contains(gotBody, `"name":"p1"`) || !strings.Contains(gotBody, `"host":"h1"`) {
		t.Fatalf("expected name/host in body, got %s", gotBody)
	}
}

// TestTUIProxiesDeleteRequiresConfirm verifies destructive actions gate on
// y/N confirmation: pressing 'd' alone issues no request; 'n' cancels; 'y'
// executes DELETE /api/v1/proxies/{id} on the selected row.
func TestTUIProxiesDeleteRequiresConfirm(t *testing.T) {
	var mu sync.Mutex
	var delMethod, delPath string
	delReqs := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		if r.Method == http.MethodDelete {
			delReqs++
			delMethod, delPath = r.Method, r.URL.Path
		}
		mu.Unlock()
		switch r.URL.Path {
		case "/api/v1/proxies":
			w.Write([]byte(`{"proxies":[{"id":"proxy_1","name":"direct","type":"DIRECT","host":"","port":0,"enabled":true}]}`))
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	defer srv.Close()

	m := tui.NewModelWithToken(srv.URL, "token")

	// Load live data (populates m.proxies via the dataLoadedMsg path).
	done := make(chan tea.Msg, 1)
	go func() { done <- m.Init()() }()
	var msg tea.Msg
	select {
	case msg = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for fetchData")
	}
	var updated tea.Model
	updated, _ = m.Update(msg)
	m = updated.(tui.Model)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("8")}) // PROXIES
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")}) // delete -> confirm
	m = updated.(tui.Model)

	mu.Lock()
	got := delReqs
	mu.Unlock()
	if got != 0 {
		t.Fatalf("expected no delete request after 'd', got %d", got)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("n")}) // cancel
	m = updated.(tui.Model)

	mu.Lock()
	got = delReqs
	mu.Unlock()
	if got != 0 {
		t.Fatalf("expected no delete request after 'n', got %d", got)
	}

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")}) // delete again
	m = updated.(tui.Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("y")}) // confirm
	m = updated.(tui.Model)

	mu.Lock()
	defer mu.Unlock()
	if delMethod != http.MethodDelete || delPath != "/api/v1/proxies/proxy_1" {
		t.Fatalf("expected DELETE /api/v1/proxies/proxy_1 after y, got %s %s", delMethod, delPath)
	}
}

// TestTUILiveRenderNoRecords verifies renderers show "No records" instead of
// hardcoded sample rows when no data has been fetched.
func TestTUILiveRenderNoRecords(t *testing.T) {
	m := tui.NewModel("http://127.0.0.1:8088")
	var updated tea.Model

	// PROXIES tab (key 8)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("8")})
	m = updated.(tui.Model)
	if view := m.View(); !strings.Contains(view, "No records") {
		t.Fatalf("expected 'No records' on empty PROXIES screen, got: %s", view)
	}

	// KEYS tab (key 5)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("5")})
	m = updated.(tui.Model)
	if view := m.View(); !strings.Contains(view, "No records") {
		t.Fatalf("expected 'No records' on empty KEYS screen, got: %s", view)
	}

	// ROUTING tab (key 7)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("7")})
	m = updated.(tui.Model)
	if view := m.View(); !strings.Contains(view, "No records") {
		t.Fatalf("expected 'No records' on empty ROUTING screen, got: %s", view)
	}
}
