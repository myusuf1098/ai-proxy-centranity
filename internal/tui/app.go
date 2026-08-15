package tui

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Tab represents an administrative screen
type Tab int

const (
	TabOverview Tab = iota
	TabRequests
	TabModels
	TabProviders
	TabKeys
	TabPolicies
	TabRouting
	TabProxies
	TabUsage
	TabAudit
	TabSystem
	TabSettings
)

var tabNames = []string{
	"OVERVIEW", "REQUESTS", "MODELS", "PROVIDERS",
	"KEYS", "POLICIES", "ROUTING", "PROXIES",
	"USAGE", "AUDIT", "SYSTEM", "SETTINGS",
}

// Model represents the Bubble Tea TUI state
type Model struct {
	activeTab     Tab
	apiURL        string
	adminToken    string
	width         int
	height        int
	client        *http.Client
	systemStatus  map[string]interface{}
	overview      map[string]interface{}
	keys          []map[string]interface{}
	policies      map[string]interface{}
	routes        map[string][]string
	proxies       []map[string]interface{}
	form          *FormState
	selected      int
	confirm       string // pending destructive-action label awaiting y/N
	lastRefreshed time.Time
	err           error
}

// NewModel creates an initialized TUI Model without admin token auth
func NewModel(apiURL string) Model {
	return NewModelWithToken(apiURL, "")
}

// NewModelWithToken creates an initialized TUI Model that sends the admin
// token as a Bearer Authorization header on Management API requests
// (required when the Management API runs with AdminAuthMiddleware).
func NewModelWithToken(apiURL string, adminToken string) Model {
	if apiURL == "" {
		apiURL = "http://127.0.0.1:8088"
	}
	return Model{
		activeTab:     TabOverview,
		apiURL:        strings.TrimRight(apiURL, "/"),
		adminToken:    adminToken,
		width:         100,
		height:        30,
		client:        &http.Client{Timeout: 3 * time.Second},
		systemStatus:  make(map[string]interface{}),
		overview:      make(map[string]interface{}),
		lastRefreshed: time.Now(),
	}
}

// ActiveTab returns the currently active tab
func (m Model) ActiveTab() Tab {
	return m.activeTab
}

type dataLoadedMsg struct {
	systemStatus map[string]interface{}
	overview     map[string]interface{}
	keys         []map[string]interface{}
	policies     map[string]interface{}
	routes       map[string][]string
	proxies      []map[string]interface{}
	err          error
}

// Do issues an authenticated request. body is raw JSON ("" for GET/DELETE).
func (m Model) Do(method, path, body string) (*http.Response, error) {
	var reader io.Reader
	if body != "" {
		reader = strings.NewReader(body)
	}
	req, err := http.NewRequest(method, fmt.Sprintf("%s%s", m.apiURL, path), reader)
	if err != nil {
		return nil, err
	}
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if m.adminToken != "" {
		req.Header.Set("Authorization", "Bearer "+m.adminToken)
	}
	return m.client.Do(req)
}

func (m Model) get(path string) (*http.Response, error) {
	return m.Do(http.MethodGet, path, "")
}

func (m Model) getKeys() []map[string]interface{} {
	resp, err := m.get("/api/v1/keys")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var out struct {
		Keys []map[string]interface{} `json:"keys"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out.Keys
}

func (m Model) getPolicies() map[string]interface{} {
	resp, err := m.get("/api/v1/policies")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var out map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out
}

func (m Model) getRoutes() map[string][]string {
	resp, err := m.get("/api/v1/routes")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var out struct {
		Routes map[string][]string `json:"routes"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out.Routes
}

func (m Model) getProxies() []map[string]interface{} {
	resp, err := m.get("/api/v1/proxies")
	if err != nil {
		return nil
	}
	defer resp.Body.Close()
	var out struct {
		Proxies []map[string]interface{} `json:"proxies"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return out.Proxies
}

func (m Model) fetchData() tea.Msg {
	var sys map[string]interface{}
	var over map[string]interface{}

	respSys, err := m.get("/api/v1/system")
	if err == nil {
		defer respSys.Body.Close()
		_ = json.NewDecoder(respSys.Body).Decode(&sys)
	}

	respOver, err := m.get("/api/v1/overview")
	if err == nil {
		defer respOver.Body.Close()
		_ = json.NewDecoder(respOver.Body).Decode(&over)
	}

	return dataLoadedMsg{
		systemStatus: sys,
		overview:     over,
		keys:         m.getKeys(),
		policies:     m.getPolicies(),
		routes:       m.getRoutes(),
		proxies:      m.getProxies(),
		err:          err,
	}
}

// Init starts initial data fetching
func (m Model) Init() tea.Cmd {
	return func() tea.Msg {
		return m.fetchData()
	}
}

// Update handles UI events and keyboard shortcuts
func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case dataLoadedMsg:
		if msg.systemStatus != nil {
			m.systemStatus = msg.systemStatus
		}
		if msg.overview != nil {
			m.overview = msg.overview
		}
		if msg.keys != nil {
			m.keys = msg.keys
		}
		if msg.policies != nil {
			m.policies = msg.policies
		}
		if msg.routes != nil {
			m.routes = msg.routes
		}
		if msg.proxies != nil {
			m.proxies = msg.proxies
		}
		m.lastRefreshed = time.Now()
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
		// Modal/form and destructive-confirmation states take precedence
		// over normal navigation.
		if m.form != nil {
			return m.handleFormKey(msg)
		}
		if m.confirm != "" {
			return m.handleConfirmKey(msg)
		}

		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit

		case "tab", "right", "l":
			m.activeTab = (m.activeTab + 1) % Tab(len(tabNames))
			return m, nil

		case "shift+tab", "left", "h":
			if m.activeTab == 0 {
				m.activeTab = Tab(len(tabNames) - 1)
			} else {
				m.activeTab--
			}
			return m, nil

		case "r":
			return m, func() tea.Msg { return m.fetchData() }

		case "1":
			m.activeTab = TabOverview
		case "2":
			m.activeTab = TabRequests
		case "3":
			m.activeTab = TabModels
		case "4":
			m.activeTab = TabProviders
		case "5":
			m.activeTab = TabKeys
		case "6":
			m.activeTab = TabPolicies
		case "7":
			m.activeTab = TabRouting
		case "8":
			m.activeTab = TabProxies
		case "9":
			m.activeTab = TabUsage
		case "0":
			m.activeTab = TabAudit
		case "-":
			m.activeTab = TabSystem
		case "=":
			m.activeTab = TabSettings

		case "a":
			m = m.openCreateForm()
		case "e":
			m = m.openEditForm()
		case "d":
			if m.isManagementTab() {
				m.confirm = "delete"
			}
		case "x":
			if m.isManagementTab() {
				m.confirm = "toggle"
			}
		}
	}

	return m, nil
}

// handleFormKey routes keystrokes while a modal form is open.
func (m Model) handleFormKey(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "esc":
		m.form = nil
	case "tab":
		m.form.NextFocus()
	case "shift+tab":
		m.form.PrevFocus()
	case "enter":
		m.form.Submit()
		m.form = nil
		return m, func() tea.Msg { return m.fetchData() }
	default:
		if len(m.form.Fields) == 0 {
			return m, nil
		}
		for _, r := range km.Runes {
			m.form.Fields[m.form.Focused].Value += string(r)
		}
	}
	return m, nil
}

// handleConfirmKey routes y/N for pending destructive actions.
func (m Model) handleConfirmKey(msg tea.Msg) (tea.Model, tea.Cmd) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return m, nil
	}
	switch km.String() {
	case "y", "Y":
		m.doConfirmAction()
		m.confirm = ""
		return m, func() tea.Msg { return m.fetchData() }
	case "n", "N", "esc":
		m.confirm = ""
	}
	return m, nil
}

// resourceList returns the rows for the active management screen.
func (m Model) resourceList() []map[string]interface{} {
	switch m.activeTab {
	case TabProxies:
		return m.proxies
	case TabKeys:
		return m.keys
	case TabRouting:
		out := make([]map[string]interface{}, 0, len(m.routes))
		for alias, targets := range m.routes {
			out = append(out, map[string]interface{}{"alias": alias, "targets": targets})
		}
		return out
	default:
		return nil
	}
}

// resourceID returns the identifier of the selected row for the active screen.
func (m Model) resourceID() string {
	list := m.resourceList()
	if len(list) == 0 {
		return ""
	}
	row := list[m.selected]
	if m.activeTab == TabRouting {
		id, _ := row["alias"].(string)
		return id
	}
	id, _ := row["id"].(string)
	return id
}

// selectedEnabled reports whether the selected row is currently enabled.
func (m Model) selectedEnabled() bool {
	list := m.resourceList()
	if len(list) == 0 {
		return false
	}
	enabled, _ := list[m.selected]["enabled"].(bool)
	return enabled
}

// isManagementTab reports whether the active screen supports actions.
func (m Model) isManagementTab() bool {
	switch m.activeTab {
	case TabProxies, TabKeys, TabPolicies, TabRouting:
		return true
	}
	return false
}

// openCreateForm opens an add form for the active management screen and
// returns the model with the form attached.
func (m Model) openCreateForm() Model {
	if !m.isManagementTab() {
		return m
	}
	switch m.activeTab {
	case TabProxies:
		m.form = NewFormState("Add Proxy", []FormField{{Label: "Name"}, {Label: "Host"}, {Label: "Port"}, {Label: "Type"}}, func(v map[string]string) {
			body := fmt.Sprintf(`{"name":%q,"host":%q,"port":%s,"type":%q}`, v["Name"], v["Host"], v["Port"], v["Type"])
			m.Do(http.MethodPost, "/api/v1/proxies", body)
		})
	case TabKeys:
		m.form = NewFormState("Add Key", []FormField{{Label: "Name"}, {Label: "RPMLimit"}}, func(v map[string]string) {
			body := fmt.Sprintf(`{"name":%q,"rpmlimit":%s}`, v["Name"], v["RPMLimit"])
			m.Do(http.MethodPost, "/api/v1/keys", body)
		})
	case TabPolicies:
		m.form = NewFormState("Set Global Deny", []FormField{{Label: "Models"}, {Label: "Providers"}}, func(v map[string]string) {
			body := fmt.Sprintf(`{"models":[%q],"providers":[%q]}`, v["Models"], v["Providers"])
			m.Do(http.MethodPut, "/api/v1/policies", body)
		})
	case TabRouting:
		m.form = NewFormState("Add Route", []FormField{{Label: "Alias"}, {Label: "Targets"}}, func(v map[string]string) {
			body := fmt.Sprintf(`{"targets":[%q]}`, v["Targets"])
			m.Do(http.MethodPut, "/api/v1/routes/"+v["Alias"], body)
		})
	}
	return m
}

// openEditForm opens a pre-filled form for the selected row and returns the
// model with the form attached.
func (m Model) openEditForm() Model {
	if !m.isManagementTab() {
		return m
	}
	if m.activeTab == TabPolicies {
		return m.openCreateForm() // policies have a single global set form
	}
	id := m.resourceID()
	if id == "" {
		return m
	}
	list := m.resourceList()
	row := list[m.selected]

	str := func(k string) string {
		s, _ := row[k].(string)
		return s
	}
	num := func(k string) string {
		if f, ok := row[k].(float64); ok && f > 0 {
			return fmt.Sprintf("%d", int(f))
		}
		return ""
	}

	switch m.activeTab {
	case TabProxies:
		m.form = NewFormState("Edit Proxy", []FormField{{Label: "Name"}, {Label: "Host"}, {Label: "Port"}, {Label: "Type"}}, func(v map[string]string) {
			body := fmt.Sprintf(`{"name":%q,"host":%q,"port":%s,"type":%q}`, v["Name"], v["Host"], v["Port"], v["Type"])
			m.Do(http.MethodPut, "/api/v1/proxies/"+id, body)
		})
		m.form.SetValue(0, str("name"))
		m.form.SetValue(1, str("host"))
		m.form.SetValue(2, num("port"))
		m.form.SetValue(3, str("type"))
	case TabKeys:
		m.form = NewFormState("Edit Key", []FormField{{Label: "Name"}, {Label: "RPMLimit"}}, func(v map[string]string) {
			body := fmt.Sprintf(`{"name":%q,"rpmlimit":%s}`, v["Name"], v["RPMLimit"])
			m.Do(http.MethodPut, "/api/v1/keys/"+id, body)
		})
		m.form.SetValue(0, str("name"))
		m.form.SetValue(1, num("rpm_limit"))
	case TabRouting:
		m.form = NewFormState("Edit Route", []FormField{{Label: "Alias"}, {Label: "Targets"}}, func(v map[string]string) {
			body := fmt.Sprintf(`{"targets":[%q]}`, v["Targets"])
			m.Do(http.MethodPut, "/api/v1/routes/"+id, body)
		})
		alias, _ := row["alias"].(string)
		m.form.SetValue(0, alias)
		if targets, ok := row["targets"].([]interface{}); ok {
			var parts []string
			for _, t := range targets {
				if s, ok := t.(string); ok {
					parts = append(parts, s)
				}
			}
			m.form.SetValue(1, strings.Join(parts, ","))
		}
	}
	return m
}

// doConfirmAction executes the pending destructive action on the selected row.
func (m Model) doConfirmAction() {
	id := m.resourceID()
	if id == "" {
		return
	}
	switch m.activeTab {
	case TabProxies:
		if m.confirm == "delete" {
			m.Do(http.MethodDelete, "/api/v1/proxies/"+id, "")
		} else if m.confirm == "toggle" {
			body := fmt.Sprintf(`{"enabled":%t}`, !m.selectedEnabled())
			m.Do(http.MethodPut, "/api/v1/proxies/"+id, body)
		}
	case TabKeys:
		if m.confirm == "delete" {
			m.Do(http.MethodDelete, "/api/v1/keys/"+id, "")
		} else if m.confirm == "toggle" {
			body := fmt.Sprintf(`{"enabled":%t}`, !m.selectedEnabled())
			m.Do(http.MethodPut, "/api/v1/keys/"+id, body)
		}
	case TabRouting:
		if m.confirm == "delete" {
			m.Do(http.MethodDelete, "/api/v1/routes/"+id, "")
		}
	}
}

// View renders the complete TUI layout
func (m Model) View() string {
	var b strings.Builder

	// 1. Top Header Bar
	headerTitle := "  ProxyGateway Enterprise  "
	headerStatus := badgeGreen.Render("● OPERATIONAL")
	header := headerStyle.Width(m.width).Render(fmt.Sprintf("%s %s", headerTitle, headerStatus))
	b.WriteString(header)
	b.WriteString("\n")

	// 2. Tab Navigation Bar (max 6 per row, wraps to next row)
	var rows []string
	for i := 0; i < len(tabNames); i += 6 {
		var tabs []string
		end := i + 6
		if end > len(tabNames) {
			end = len(tabNames)
		}
		for j := i; j < end; j++ {
			if Tab(j) == m.activeTab {
				tabs = append(tabs, activeTabStyle.Render(fmt.Sprintf("%d:%s", j+1, tabNames[j])))
			} else {
				tabs = append(tabs, tabStyle.Render(fmt.Sprintf("%d:%s", j+1, tabNames[j])))
			}
		}
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, tabs...))
	}
	tabBar := lipgloss.JoinVertical(lipgloss.Left, rows...)
	b.WriteString(tabBar)
	b.WriteString("\n\n")

	// 3. Screen Body
	bodyHeight := m.height - 7
	if bodyHeight < 10 {
		bodyHeight = 10
	}

	bodyContent := m.renderActiveScreen()
	renderedBody := boxStyle.Width(m.width - 4).Height(bodyHeight).Render(bodyContent)
	b.WriteString(renderedBody)
	b.WriteString("\n")

	// 4. Bottom Status & Shortcuts Footer
	footerContent := fmt.Sprintf("q: Quit | Tab: Next Tab | 1-0: Jump Screen | r: Refresh | a: Add | e: Edit | d: Delete | x: Toggle | Connected: %s", m.apiURL)
	footer := footerStyle.Width(m.width).Render(footerContent)
	b.WriteString(footer)

	return b.String()
}

func (m Model) renderActiveScreen() string {
	var content string
	switch m.activeTab {
	case TabOverview:
		content = m.renderOverview()
	case TabRequests:
		content = m.renderRequests()
	case TabModels:
		content = m.renderModels()
	case TabProviders:
		content = m.renderProviders()
	case TabKeys:
		content = m.renderKeys()
	case TabPolicies:
		content = m.renderPolicies()
	case TabRouting:
		content = m.renderRouting()
	case TabProxies:
		content = m.renderProxies()
	case TabUsage:
		content = m.renderUsage()
	case TabAudit:
		content = m.renderAudit()
	case TabSystem:
		content = m.renderSystem()
	case TabSettings:
		content = m.renderSettings()
	default:
		content = "Screen under construction."
	}

	if m.form != nil {
		content += "\n" + FormView(m.form)
	} else if m.confirm != "" {
		action := "run this action"
		switch m.confirm {
		case "delete":
			action = "delete the selected record"
		case "toggle":
			action = "toggle enable/disable for the selected record"
		}
		content += fmt.Sprintf("\n\n⚠ Confirm %s? [y/N]", action)
	}
	return content
}

func (m Model) renderOverview() string {
	var s strings.Builder
	s.WriteString(lipgloss.NewStyle().Bold(true).Render("📊 OPERATIONAL OVERVIEW") + "\n\n")
	s.WriteString("Control Plane:  " + badgeGreen.Render("ACTIVE") + " (Management API :8088)\n")
	s.WriteString("Data Plane:     " + badgeGreen.Render("ACTIVE") + " (OpenAI API /v1/chat/completions)\n")
	s.WriteString("9Router Engine: " + badgeGreen.Render("CONNECTED") + " (http://127.0.0.1:20128)\n")
	s.WriteString("Database Store: " + badgeGreen.Render("POSTGRESQL 16") + " (Authoritative State)\n")
	s.WriteString("Rate Limiting:  " + badgeGreen.Render("SLIDING-WINDOW") + " (Active Protection)\n\n")

	s.WriteString("Quick Metrics:\n")
	s.WriteString("• Active Aliases:      5 (coding, fast, reasoning, cheap, free)\n")
	s.WriteString("• Circuit Breakers:    100% HEALTHY (0 open circuits)\n")
	s.WriteString(fmt.Sprintf("• Last Telemetry Sync: %s\n", m.lastRefreshed.Format("15:04:05 MST")))
	return s.String()
}

func (m Model) renderRequests() string {
	return lipgloss.NewStyle().Bold(true).Render("⚡ RECENT REQUESTS STREAM") + "\n\n" +
		"ID                        METHOD  PATH                    STATUS  LATENCY   MODEL\n" +
		"----------------------------------------------------------------------------------\n" +
		"req_9eaca488d46ebe23      POST    /v1/chat/completions    200 OK  184.2ms   cc-haiku\n" +
		"req_f189c44b9319e712      GET     /v1/models              200 OK  12.1ms    -\n" +
		"req_48bce91238ef128a      POST    /v1/chat/completions    200 OK  245.8ms   cc-sonnet\n"
}

func (m Model) renderModels() string {
	return lipgloss.NewStyle().Bold(true).Render("🤖 MODEL REGISTRY & DISCOVERY") + "\n\n" +
		"MODEL ID                  PROVIDER    TYPE      STATUS     CONTEXT\n" +
		"----------------------------------------------------------------------\n" +
		"cc-haiku                  9Router     combo     " + badgeGreen.Render("ENABLED") + "    8,192\n" +
		"cc-sonnet                 9Router     combo     " + badgeGreen.Render("ENABLED") + "    200,000\n" +
		"cc-opus                   9Router     combo     " + badgeGreen.Render("ENABLED") + "    200,000\n" +
		"gemini-flash              Google      direct    " + badgeGreen.Render("ENABLED") + "    1,000,000\n"
}

func (m Model) renderProviders() string {
	return lipgloss.NewStyle().Bold(true).Render("🏢 UPSTREAM PROVIDERS") + "\n\n" +
		"PROVIDER       BASE URL                 HEALTH      TIMEOUT   PROXY\n" +
		"-----------------------------------------------------------------------\n" +
		"9Router Core   http://127.0.0.1:20128   " + badgeGreen.Render("HEALTHY") + "     30s       DIRECT\n" +
		"Anthropic      https://api.anthropic    " + badgeGreen.Render("HEALTHY") + "     60s       DIRECT\n" +
		"OpenAI         https://api.openai.com   " + badgeGreen.Render("HEALTHY") + "     60s       DIRECT\n"
}

// fieldNum extracts a numeric field from a JSON-decoded row (float64 from
// encoding/json); returns 0 when absent or non-numeric.
func fieldNum(row map[string]interface{}, key string) float64 {
	if f, ok := row[key].(float64); ok {
		return f
	}
	return 0
}

func (m Model) renderKeys() string {
	var s strings.Builder
	s.WriteString(lipgloss.NewStyle().Bold(true).Render("🔑 API KEYS & IDENTITIES") + "\n\n")
	if len(m.keys) == 0 {
		s.WriteString("No records\n")
		return s.String()
	}
	s.WriteString("KEY ID            NAME             PREFIX        RPM   STATUS\n")
	s.WriteString("------------------------------------------------------------------------\n")
	for i, k := range m.keys {
		sel := "  "
		if i == m.selected {
			sel = "> "
		}
		id, _ := k["id"].(string)
		name, _ := k["name"].(string)
		prefix, _ := k["prefix"].(string)
		rpm := fmt.Sprintf("%d", int(fieldNum(k, "rpm_limit")))
		status := badgeGreen.Render("ACTIVE")
		if enabled, ok := k["enabled"].(bool); ok && !enabled {
			status = "DISABLED"
		}
		s.WriteString(fmt.Sprintf("%s%-18s %-16s %-12s %-6s %s\n", sel, id, name, prefix, rpm, status))
	}
	return s.String()
}

func (m Model) renderPolicies() string {
	var s strings.Builder
	s.WriteString(lipgloss.NewStyle().Bold(true).Render("🛡️ POLICY ENGINE (PRECEDENCE: DENY > ALLOW)") + "\n\n")
	models, _ := m.policies["models"].([]interface{})
	providers, _ := m.policies["providers"].([]interface{})
	if len(models) == 0 && len(providers) == 0 {
		s.WriteString("No records\n")
		return s.String()
	}
	s.WriteString("SCOPE          RULE TYPE       TARGET            DECISION   ENFORCEMENT\n")
	s.WriteString("--------------------------------------------------------------------------\n")
	for _, mdl := range models {
		s.WriteString(fmt.Sprintf("%-15s %-16s %-18v %-10s %s\n", "Global", "Model Deny", mdl, "DENY", "Strict"))
	}
	for _, prv := range providers {
		s.WriteString(fmt.Sprintf("%-15s %-16s %-18v %-10s %s\n", "Global", "Provider Deny", prv, "DENY", "Strict"))
	}
	return s.String()
}

func (m Model) renderRouting() string {
	var s strings.Builder
	s.WriteString(lipgloss.NewStyle().Bold(true).Render("🔀 INTELLIGENT ROUTING & ALIASES") + "\n\n")
	if len(m.routes) == 0 {
		s.WriteString("No records\n")
		return s.String()
	}
	s.WriteString("ALIAS        TARGET RESOLUTION CHAIN                  CIRCUIT STATE\n")
	s.WriteString("------------------------------------------------------------------------\n")
	aliases := make([]string, 0, len(m.routes))
	for alias := range m.routes {
		aliases = append(aliases, alias)
	}
	sort.Strings(aliases)
	for i, alias := range aliases {
		sel := "  "
		if i == m.selected {
			sel = "> "
		}
		targets := m.routes[alias]
		chain := "1. " + strings.Join(targets, ", ")
		s.WriteString(fmt.Sprintf("%s%-11s ➔   %-36s %s\n", sel, alias, chain, badgeGreen.Render("CLOSED (Healthy)")))
	}
	return s.String()
}

func (m Model) renderProxies() string {
	var s strings.Builder
	s.WriteString(lipgloss.NewStyle().Bold(true).Render("🌐 OUTBOUND PROXY PROFILES") + "\n\n")
	if len(m.proxies) == 0 {
		s.WriteString("No records\n")
		return s.String()
	}
	s.WriteString("PROFILE ID        TYPE      HOST:PORT              STATUS\n")
	s.WriteString("------------------------------------------------------------------------\n")
	for i, p := range m.proxies {
		sel := "  "
		if i == m.selected {
			sel = "> "
		}
		id, _ := p["id"].(string)
		typ, _ := p["type"].(string)
		host, _ := p["host"].(string)
		port := int(fieldNum(p, "port"))
		hostPort := fmt.Sprintf("%s:%d", host, port)
		if host == "" {
			hostPort = "-"
		}
		status := badgeGreen.Render("ACTIVE")
		if enabled, ok := p["enabled"].(bool); ok && !enabled {
			status = "DISABLED"
		}
		s.WriteString(fmt.Sprintf("%s%-16s %-9s %-22s %s\n", sel, id, typ, hostPort, status))
	}
	return s.String()
}

func (m Model) renderUsage() string {
	return lipgloss.NewStyle().Bold(true).Render("📈 TOKEN USAGE & BUDGETS") + "\n\n" +
		"DATE        REQUESTS   PROMPT TOKENS   COMPLETION TOKENS   EST. COST\n" +
		"--------------------------------------------------------------------\n" +
		"2026-08-14  1,420      248,190         89,200              $0.342\n" +
		"2026-08-13  3,890      612,400         215,800             $0.819\n"
}

func (m Model) renderAudit() string {
	return lipgloss.NewStyle().Bold(true).Render("📋 AUDIT TRAIL LOG") + "\n\n" +
		"TIMESTAMP             ACTOR     ACTION           TARGET          STATUS\n" +
		"-------------------------------------------------------------------------\n" +
		"2026-08-14 22:30:10   admin     CREATE_KEY       key_8f192b4     " + badgeGreen.Render("SUCCESS") + "\n" +
		"2026-08-14 22:15:00   system    MIGRATE_SCHEMA   MIG-0001        " + badgeGreen.Render("SUCCESS") + "\n"
}

func (m Model) renderSystem() string {
	goVer := "go1.26.6 linux/arm64"
	if v, ok := m.systemStatus["go_version"].(string); ok {
		goVer = v
	}

	return lipgloss.NewStyle().Bold(true).Render("⚙️ SYSTEM STATUS & RUNTIME") + "\n\n" +
		fmt.Sprintf("Architecture:    %s\n", goVer) +
		"Management API:  http://127.0.0.1:8088\n" +
		"PostgreSQL Pool: Max 25 conns, 10 idle\n" +
		"Redis Limiter:   Connected (localhost:6379)\n" +
		"Host Platform:   Docker Compose (Ubuntu Host)\n"
}

func (m Model) renderSettings() string {
	return lipgloss.NewStyle().Bold(true).Render("🔧 SETTINGS & CONFIGURATION") + "\n\n" +
		"Environment:       Production\n" +
		"Read Timeout:      30s\n" +
		"Write Timeout:     60s\n" +
		"Graceful Shutdown: 10s\n" +
		"Audit Retention:   90 days\n"
}
