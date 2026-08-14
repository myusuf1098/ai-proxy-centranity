package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"runtime"
	"time"

	"github.com/myusuf1098/ai-proxy-centranity/internal/audit"
	"github.com/myusuf1098/ai-proxy-centranity/internal/auth"
	"github.com/myusuf1098/ai-proxy-centranity/internal/ninerouter"
	"github.com/myusuf1098/ai-proxy-centranity/internal/proxy"
	"github.com/myusuf1098/ai-proxy-centranity/internal/routing"
)

var startTime = time.Now()

// ManagementHandler serves control plane and TUI administrative endpoints
type ManagementHandler struct {
	adapter     ninerouter.NineRouterPort
	keyStore    auth.KeyStore
	routeEngine *routing.Engine
	proxyStore  proxy.Store
	logger      *slog.Logger
	auditStore  audit.Store
}

// NewManagementHandler creates a new ManagementHandler
func NewManagementHandler(
	adapter ninerouter.NineRouterPort,
	keyStore auth.KeyStore,
	routeEngine *routing.Engine,
	proxyStore proxy.Store,
	logger *slog.Logger,
) *ManagementHandler {
	return &ManagementHandler{
		adapter:     adapter,
		keyStore:    keyStore,
		routeEngine: routeEngine,
		proxyStore:  proxyStore,
		logger:      logger,
	}
}

// SystemStatusResponse represents the /api/v1/system payload
type SystemStatusResponse struct {
	Name      string    `json:"name"`
	Version   string    `json:"version"`
	Status    string    `json:"status"`
	GoVersion string    `json:"go_version"`
	Arch      string    `json:"arch"`
	NumCPU    int       `json:"num_cpu"`
	UptimeSec float64   `json:"uptime_sec"`
	Timestamp time.Time `json:"timestamp"`
}

// OverviewResponse represents the /api/v1/overview payload
type OverviewResponse struct {
	Status       string    `json:"status"`
	ModelsCount  int       `json:"models_count"`
	RoutesCount  int       `json:"routes_count"`
	ProxiesCount int       `json:"proxies_count"`
	Timestamp    time.Time `json:"timestamp"`
}

// GetSystem handles GET /api/v1/system
func (h *ManagementHandler) GetSystem(w http.ResponseWriter, r *http.Request) {
	resp := SystemStatusResponse{
		Name:      "ProxyGateway Enterprise",
		Version:   "2.0",
		Status:    "operational",
		GoVersion: runtime.Version(),
		Arch:      runtime.GOARCH,
		NumCPU:    runtime.NumCPU(),
		UptimeSec: time.Since(startTime).Seconds(),
		Timestamp: time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// GetOverview handles GET /api/v1/overview
func (h *ManagementHandler) GetOverview(w http.ResponseWriter, r *http.Request) {
	modelsCount := 0
	if h.adapter != nil {
		if models, err := h.adapter.ListModels(r.Context()); err == nil {
			modelsCount = len(models)
		}
	}

	proxiesCount := 0
	if h.proxyStore != nil {
		if list, err := h.proxyStore.List(r.Context()); err == nil {
			proxiesCount = len(list)
		}
	}

	resp := OverviewResponse{
		Status:       "ok",
		ModelsCount:  modelsCount,
		RoutesCount:  5, // Standard built-in aliases
		ProxiesCount: proxiesCount,
		Timestamp:    time.Now().UTC(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(resp)
}

// SetAuditStore injects the audit trail store
func (h *ManagementHandler) SetAuditStore(s audit.Store) { h.auditStore = s }

// logAudit emits an audit event when an audit store is configured
func (h *ManagementHandler) logAudit(ctx context.Context, eventType, actor, target, status string, meta map[string]string) {
	emitAudit(ctx, h.auditStore, eventType, actor, target, status, meta)
}

// GetProxies handles GET /api/v1/proxies
func (h *ManagementHandler) GetProxies(w http.ResponseWriter, r *http.Request) {
	var list []*proxy.Profile
	if h.proxyStore != nil {
		var err error
		list, err = h.proxyStore.List(r.Context())
		if err != nil {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusInternalServerError)
			_ = json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
			return
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"proxies": list,
	})
}

// ProxyRequest is the write model; credentials allowed on input only
type ProxyRequest struct {
	Name     string `json:"name"`
	Type     string `json:"type"`
	Host     string `json:"host"`
	Port     int    `json:"port"`
	Username string `json:"username"`
	Password string `json:"password"`
	Enabled  bool   `json:"enabled"`
}

// CreateProxy handles POST /api/v1/proxies
func (h *ManagementHandler) CreateProxy(w http.ResponseWriter, r *http.Request) {
	var req ProxyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Type == "" || req.Host == "" || req.Port <= 0 {
		h.writeJSONError(w, http.StatusBadRequest, "name, type, host, port required")
		return
	}
	if h.proxyStore == nil {
		h.writeJSONError(w, http.StatusInternalServerError, "proxy store unavailable")
		return
	}
	profile := &proxy.Profile{
		ID:        "proxy_" + generateID(),
		Name:      req.Name,
		Type:      proxy.Type(req.Type),
		Host:      req.Host,
		Port:      req.Port,
		Username:  req.Username,
		Password:  req.Password,
		Enabled:   req.Enabled,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	}
	if err := h.proxyStore.Save(r.Context(), profile); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.logAudit(r.Context(), audit.EventConfigChanged, "admin", r.URL.Path, "changed", map[string]string{"resource": "proxies"})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(profile) // json:"-" redacts creds
}

// GetProxy handles GET /api/v1/proxies/{id}
func (h *ManagementHandler) GetProxy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if h.proxyStore == nil {
		h.writeJSONError(w, http.StatusNotFound, "proxy store unavailable")
		return
	}
	p, err := h.proxyStore.Get(r.Context(), id)
	if err != nil {
		h.writeJSONError(w, http.StatusNotFound, "proxy not found")
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(p)
}

// UpdateProxy handles PUT /api/v1/proxies/{id}
func (h *ManagementHandler) UpdateProxy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if h.proxyStore == nil {
		h.writeJSONError(w, http.StatusNotFound, "proxy store unavailable")
		return
	}
	existing, err := h.proxyStore.Get(r.Context(), id)
	if err != nil {
		h.writeJSONError(w, http.StatusNotFound, "proxy not found")
		return
	}
	var req ProxyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	// Apply non-empty request fields; zero values are left untouched
	if req.Name != "" {
		existing.Name = req.Name
	}
	if req.Type != "" {
		existing.Type = proxy.Type(req.Type)
	}
	if req.Host != "" {
		existing.Host = req.Host
	}
	if req.Port > 0 {
		existing.Port = req.Port
	}
	if req.Username != "" {
		existing.Username = req.Username
	}
	if req.Password != "" {
		existing.Password = req.Password
	}
	existing.Enabled = req.Enabled
	existing.UpdatedAt = time.Now().UTC()
	if err := h.proxyStore.Save(r.Context(), existing); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.logAudit(r.Context(), audit.EventConfigChanged, "admin", r.URL.Path, "changed", map[string]string{"resource": "proxies"})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(existing)
}

// DeleteProxy handles DELETE /api/v1/proxies/{id}
func (h *ManagementHandler) DeleteProxy(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if h.proxyStore == nil {
		h.writeJSONError(w, http.StatusNotFound, "proxy store unavailable")
		return
	}
	if err := h.proxyStore.Delete(r.Context(), id); err != nil {
		h.writeJSONError(w, http.StatusNotFound, "proxy not found")
		return
	}
	h.logAudit(r.Context(), audit.EventConfigChanged, "admin", r.URL.Path, "changed", map[string]string{"resource": "proxies"})
	w.WriteHeader(http.StatusNoContent)
}

func (h *ManagementHandler) writeJSONError(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
