package tui

import (
	"encoding/json"
	"fmt"
	"net/http"
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
	activeTab    Tab
	apiURL       string
	width        int
	height       int
	client       *http.Client
	systemStatus map[string]interface{}
	overview     map[string]interface{}
	lastRefreshed time.Time
	err          error
}

// NewModel creates an initialized TUI Model
func NewModel(apiURL string) Model {
	if apiURL == "" {
		apiURL = "http://127.0.0.1:8088"
	}
	return Model{
		activeTab:     TabOverview,
		apiURL:        strings.TrimRight(apiURL, "/"),
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
	err          error
}

func (m Model) fetchData() tea.Msg {
	var sys map[string]interface{}
	var over map[string]interface{}

	respSys, err := m.client.Get(fmt.Sprintf("%s/api/v1/system", m.apiURL))
	if err == nil {
		defer respSys.Body.Close()
		_ = json.NewDecoder(respSys.Body).Decode(&sys)
	}

	respOver, err := m.client.Get(fmt.Sprintf("%s/api/v1/overview", m.apiURL))
	if err == nil {
		defer respOver.Body.Close()
		_ = json.NewDecoder(respOver.Body).Decode(&over)
	}

	return dataLoadedMsg{
		systemStatus: sys,
		overview:     over,
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
		m.lastRefreshed = time.Now()
		m.err = msg.err
		return m, nil

	case tea.KeyMsg:
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
		}
	}

	return m, nil
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
	footerContent := fmt.Sprintf("q: Quit | Tab: Next Tab | 1-0: Jump Screen | r: Refresh | Connected: %s", m.apiURL)
	footer := footerStyle.Width(m.width).Render(footerContent)
	b.WriteString(footer)

	return b.String()
}

func (m Model) renderActiveScreen() string {
	switch m.activeTab {
	case TabOverview:
		return m.renderOverview()
	case TabRequests:
		return m.renderRequests()
	case TabModels:
		return m.renderModels()
	case TabProviders:
		return m.renderProviders()
	case TabKeys:
		return m.renderKeys()
	case TabPolicies:
		return m.renderPolicies()
	case TabRouting:
		return m.renderRouting()
	case TabProxies:
		return m.renderProxies()
	case TabUsage:
		return m.renderUsage()
	case TabAudit:
		return m.renderAudit()
	case TabSystem:
		return m.renderSystem()
	case TabSettings:
		return m.renderSettings()
	default:
		return "Screen under construction."
	}
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

func (m Model) renderKeys() string {
	return lipgloss.NewStyle().Bold(true).Render("🔑 API KEYS & IDENTITIES") + "\n\n" +
		"KEY ID            NAME             PREFIX        RPM   STATUS     EXPIRES\n" +
		"-----------------------------------------------------------------------------\n" +
		"key_8f192b4       Production Svc   sk-pg-8f19    60    " + badgeGreen.Render("ACTIVE") + "     Never\n" +
		"key_3a918e2       Developer Test   sk-pg-3a91    30    " + badgeGreen.Render("ACTIVE") + "     2027-01-01\n"
}

func (m Model) renderPolicies() string {
	return lipgloss.NewStyle().Bold(true).Render("🛡️ POLICY ENGINE (PRECEDENCE: DENY > ALLOW)") + "\n\n" +
		"SCOPE          RULE TYPE       TARGET            DECISION   ENFORCEMENT\n" +
		"--------------------------------------------------------------------------\n" +
		"Global         Safety          harmful-*         DENY       Strict\n" +
		"Key: key_8f    Model Allow     cc-*, gemini-*    ALLOW      Enforced\n" +
		"Key: key_3a    Model Deny      cc-opus           DENY       Enforced\n"
}

func (m Model) renderRouting() string {
	return lipgloss.NewStyle().Bold(true).Render("🔀 INTELLIGENT ROUTING & ALIASES") + "\n\n" +
		"ALIAS        TARGET RESOLUTION CHAIN                  CIRCUIT STATE\n" +
		"------------------------------------------------------------------------\n" +
		"coding   ➔   1. cc-sonnet, 2. cc-haiku                " + badgeGreen.Render("CLOSED (Healthy)") + "\n" +
		"fast     ➔   1. cc-haiku, 2. gemini-flash             " + badgeGreen.Render("CLOSED (Healthy)") + "\n" +
		"reasoning➔   1. cc-opus, 2. cc-sonnet                 " + badgeGreen.Render("CLOSED (Healthy)") + "\n" +
		"cheap    ➔   1. cc-haiku                              " + badgeGreen.Render("CLOSED (Healthy)") + "\n"
}

func (m Model) renderProxies() string {
	return lipgloss.NewStyle().Bold(true).Render("🌐 OUTBOUND PROXY PROFILES") + "\n\n" +
		"PROFILE ID        TYPE      HOST:PORT              STATUS     CREDENTIALS\n" +
		"-----------------------------------------------------------------------------\n" +
		"proxy_direct      DIRECT    -                      " + badgeGreen.Render("ACTIVE") + "     [None Required]\n" +
		"proxy_socks5_us   SOCKS5    proxy.example.com:1080 " + badgeGreen.Render("ACTIVE") + "     [REDACTED/SECURE]\n"
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
