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
	if h.auditStore == nil {
		return
	}
	_ = h.auditStore.Log(ctx, audit.Event{
		ID:        GenerateAuditID(),
		Timestamp: time.Now().UTC(),
		Actor:     actor,
		EventType: eventType,
		Target:    target,
		Status:    status,
		Metadata:  meta,
	})
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
