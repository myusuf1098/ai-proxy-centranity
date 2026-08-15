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
	"github.com/myusuf1098/ai-proxy-centranity/internal/policy"
	"github.com/myusuf1098/ai-proxy-centranity/internal/proxy"
	"github.com/myusuf1098/ai-proxy-centranity/internal/routing"
)

var startTime = time.Now()

// ManagementHandler serves control plane and TUI administrative endpoints
type ManagementHandler struct {
	adapter      ninerouter.NineRouterPort
	keyStore     auth.KeyStore
	routeEngine  *routing.Engine
	policyEngine *policy.Engine
	proxyStore   proxy.Store
	logger       *slog.Logger
	auditStore   audit.Store
}

// NewManagementHandler creates a new ManagementHandler
func NewManagementHandler(
	adapter ninerouter.NineRouterPort,
	keyStore auth.KeyStore,
	routeEngine *routing.Engine,
	policyEngine *policy.Engine,
	proxyStore proxy.Store,
	logger *slog.Logger,
) *ManagementHandler {
	return &ManagementHandler{
		adapter:      adapter,
		keyStore:     keyStore,
		routeEngine:  routeEngine,
		policyEngine: policyEngine,
		proxyStore:   proxyStore,
		logger:       logger,
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
	Enabled  *bool  `json:"enabled"`
}

// CreateProxy handles POST /api/v1/proxies
func (h *ManagementHandler) CreateProxy(w http.ResponseWriter, r *http.Request) {
	var req ProxyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Name == "" || req.Type == "" || req.Host == "" || req.Port <= 0 {
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
		Enabled:   req.Enabled == nil || *req.Enabled, // default enabled on create
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
		h.writeJSONError(w, http.StatusInternalServerError, "proxy store unavailable")
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
		h.writeJSONError(w, http.StatusInternalServerError, "proxy store unavailable")
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
	if req.Enabled != nil {
		existing.Enabled = *req.Enabled
	}
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
		h.writeJSONError(w, http.StatusInternalServerError, "proxy store unavailable")
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

// KeyRequest is the write model for API keys.
type KeyRequest struct {
	Name              string   `json:"name"`
	Enabled           *bool    `json:"enabled"`
	RPMLimit          int      `json:"rpmlimit"`
	RPSLimit          int      `json:"rpslimit"`
	AllowedModels     []string `json:"allowed_models"`
	DeniedModels      []string `json:"denied_models"`
	AllowedProviders  []string `json:"allowed_providers"`
	DeniedProviders   []string `json:"denied_providers"`
	DailyTokenQuota   int64    `json:"daily_token_quota"`
	MonthlyTokenQuota int64    `json:"monthly_token_quota"`
}

// KeyResponse carries a raw key back once, on create only.
type KeyResponse struct {
	Key string `json:"key,omitempty"`
	*auth.APIKey
}

// ListKeys handles GET /api/v1/keys
func (h *ManagementHandler) ListKeys(w http.ResponseWriter, r *http.Request) {
	if h.keyStore == nil {
		h.writeJSONError(w, http.StatusInternalServerError, "key store unavailable")
		return
	}
	list, err := h.keyStore.List(r.Context())
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"keys": list})
}

// CreateKey handles POST /api/v1/keys
func (h *ManagementHandler) CreateKey(w http.ResponseWriter, r *http.Request) {
	if h.keyStore == nil {
		h.writeJSONError(w, http.StatusInternalServerError, "key store unavailable")
		return
	}
	var req KeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if req.Name == "" {
		h.writeJSONError(w, http.StatusBadRequest, "name required")
		return
	}
	rawKey, keyModel, err := auth.GenerateAPIKey(req.Name)
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if req.Enabled != nil {
		keyModel.Enabled = *req.Enabled
	}
	if req.RPMLimit > 0 {
		keyModel.RPMLimit = req.RPMLimit
	}
	if req.RPSLimit > 0 {
		keyModel.RPSLimit = req.RPSLimit
	}
	if req.AllowedModels != nil {
		keyModel.AllowedModels = req.AllowedModels
	}
	if req.DeniedModels != nil {
		keyModel.DeniedModels = req.DeniedModels
	}
	if req.AllowedProviders != nil {
		keyModel.AllowedProviders = req.AllowedProviders
	}
	if req.DeniedProviders != nil {
		keyModel.DeniedProviders = req.DeniedProviders
	}
	if req.DailyTokenQuota > 0 {
		keyModel.DailyTokenQuota = req.DailyTokenQuota
	}
	if req.MonthlyTokenQuota > 0 {
		keyModel.MonthlyTokenQuota = req.MonthlyTokenQuota
	}
	if err := h.keyStore.Create(r.Context(), keyModel); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.logAudit(r.Context(), audit.EventConfigChanged, "admin", r.URL.Path, "changed", map[string]string{"resource": "keys"})
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	_ = json.NewEncoder(w).Encode(KeyResponse{Key: rawKey, APIKey: keyModel})
}

// UpdateKey handles PUT /api/v1/keys/{id}
func (h *ManagementHandler) UpdateKey(w http.ResponseWriter, r *http.Request) {
	if h.keyStore == nil {
		h.writeJSONError(w, http.StatusInternalServerError, "key store unavailable")
		return
	}
	var req KeyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	list, err := h.keyStore.List(r.Context())
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var target *auth.APIKey
	for _, k := range list {
		if k.ID == r.PathValue("id") {
			target = k
			break
		}
	}
	if target == nil {
		h.writeJSONError(w, http.StatusNotFound, "key not found")
		return
	}
	if req.Name != "" {
		target.Name = req.Name
	}
	if req.Enabled != nil {
		target.Enabled = *req.Enabled
	}
	if req.RPMLimit > 0 {
		target.RPMLimit = req.RPMLimit
	}
	if req.RPSLimit > 0 {
		target.RPSLimit = req.RPSLimit
	}
	if req.AllowedModels != nil {
		target.AllowedModels = req.AllowedModels
	}
	if req.DeniedModels != nil {
		target.DeniedModels = req.DeniedModels
	}
	if req.AllowedProviders != nil {
		target.AllowedProviders = req.AllowedProviders
	}
	if req.DeniedProviders != nil {
		target.DeniedProviders = req.DeniedProviders
	}
	if req.DailyTokenQuota > 0 {
		target.DailyTokenQuota = req.DailyTokenQuota
	}
	if req.MonthlyTokenQuota > 0 {
		target.MonthlyTokenQuota = req.MonthlyTokenQuota
	}
	target.UpdatedAt = time.Now().UTC()
	if err := h.keyStore.Update(r.Context(), target); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.logAudit(r.Context(), audit.EventConfigChanged, "admin", r.URL.Path, "changed", map[string]string{"resource": "keys"})
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(target)
}

// DeleteKey handles DELETE /api/v1/keys/{id}
func (h *ManagementHandler) DeleteKey(w http.ResponseWriter, r *http.Request) {
	if h.keyStore == nil {
		h.writeJSONError(w, http.StatusInternalServerError, "key store unavailable")
		return
	}
	list, err := h.keyStore.List(r.Context())
	if err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	var hash string
	for _, k := range list {
		if k.ID == r.PathValue("id") {
			hash = k.Hash
			break
		}
	}
	if hash == "" {
		h.writeJSONError(w, http.StatusNotFound, "key not found")
		return
	}
	if err := h.keyStore.Delete(r.Context(), hash); err != nil {
		h.writeJSONError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.logAudit(r.Context(), audit.EventConfigChanged, "admin", r.URL.Path, "changed", map[string]string{"resource": "keys"})
	w.WriteHeader(http.StatusNoContent)
}

// PoliciesResponse represents global deny policy.
type PoliciesResponse struct {
	Models    []string `json:"models"`
	Providers []string `json:"providers"`
}

// GetPolicies handles GET /api/v1/policies
func (h *ManagementHandler) GetPolicies(w http.ResponseWriter, r *http.Request) {
	var models, providers []string
	if h.policyEngine != nil {
		models, providers = h.policyEngine.GetGlobalDeny()
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(PoliciesResponse{Models: models, Providers: providers})
}

// UpdatePolicies handles PUT /api/v1/policies
func (h *ManagementHandler) UpdatePolicies(w http.ResponseWriter, r *http.Request) {
	var req PoliciesResponse
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if h.policyEngine == nil {
		h.writeJSONError(w, http.StatusInternalServerError, "policy engine unavailable")
		return
	}
	h.policyEngine.SetGlobalDeny(req.Models, req.Providers)
	h.logAudit(r.Context(), audit.EventConfigChanged, "admin", r.URL.Path, "changed", map[string]string{"resource": "policies"})
	w.WriteHeader(http.StatusOK)
}

// ListRoutes handles GET /api/v1/routes
func (h *ManagementHandler) ListRoutes(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"routes": h.routeEngine.GetAliases()})
}

// UpdateRoute handles PUT /api/v1/routes/{alias}
func (h *ManagementHandler) UpdateRoute(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Targets []string `json:"targets"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeJSONError(w, http.StatusBadRequest, "invalid JSON: "+err.Error())
		return
	}
	if len(req.Targets) == 0 {
		h.writeJSONError(w, http.StatusBadRequest, "targets required")
		return
	}
	h.routeEngine.SetAlias(r.PathValue("alias"), req.Targets)
	h.logAudit(r.Context(), audit.EventConfigChanged, "admin", r.URL.Path, "changed", map[string]string{"resource": "routes"})
	w.WriteHeader(http.StatusOK)
}

// DeleteRoute handles DELETE /api/v1/routes/{alias}
func (h *ManagementHandler) DeleteRoute(w http.ResponseWriter, r *http.Request) {
	if err := h.routeEngine.DeleteAlias(r.PathValue("alias")); err != nil {
		h.writeJSONError(w, http.StatusNotFound, err.Error())
		return
	}
	h.logAudit(r.Context(), audit.EventConfigChanged, "admin", r.URL.Path, "changed", map[string]string{"resource": "routes"})
	w.WriteHeader(http.StatusNoContent)
}
