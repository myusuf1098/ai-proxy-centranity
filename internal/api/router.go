package api

import (
	"log/slog"
	"net/http"

	"github.com/myusuf1098/ai-proxy-centranity/internal/audit"
	"github.com/myusuf1098/ai-proxy-centranity/internal/auth"
	"github.com/myusuf1098/ai-proxy-centranity/internal/config"
	"github.com/myusuf1098/ai-proxy-centranity/internal/health"
	"github.com/myusuf1098/ai-proxy-centranity/internal/ninerouter"
	"github.com/myusuf1098/ai-proxy-centranity/internal/telemetry"
)

// auditAuthFailures wraps a handler and emits an AUTH_FAILURE audit event when it responds 401
func auditAuthFailures(store audit.Store) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(rw, r)
			if rw.statusCode == http.StatusUnauthorized {
				emitAudit(r.Context(), store, audit.EventAuthFailure, "unknown", r.URL.Path, "unauthorized", nil)
			}
		})
	}
}

// NewRouter initializes HTTP routes without an active upstream adapter (fallback)
func NewRouter(cfg *config.Config, healthHandler *health.Handler, logger *slog.Logger) http.Handler {
	return NewRouterWithAdapter(cfg, healthHandler, nil, logger)
}

// NewRouterWithAdapter initializes routes with 9Router adapter
func NewRouterWithAdapter(cfg *config.Config, healthHandler *health.Handler, adapter ninerouter.NineRouterPort, logger *slog.Logger) http.Handler {
	var dpHandler *DataPlaneHandler
	if adapter != nil {
		dpHandler = NewDataPlaneHandler(adapter, logger)
	}
	return NewRouterWithManagement(cfg, healthHandler, dpHandler, nil, logger)
}

// NewRouterWithDataPlane initializes routes with DataPlaneHandler
func NewRouterWithDataPlane(cfg *config.Config, healthHandler *health.Handler, dpHandler *DataPlaneHandler, logger *slog.Logger) http.Handler {
	return NewRouterWithManagement(cfg, healthHandler, dpHandler, nil, logger)
}

// NewRouterWithManagement initializes routes with DataPlaneHandler and ManagementHandler
func NewRouterWithManagement(
	cfg *config.Config,
	healthHandler *health.Handler,
	dpHandler *DataPlaneHandler,
	mgmtHandler *ManagementHandler,
	logger *slog.Logger,
) http.Handler {
	return NewRouterWithTelemetry(cfg, healthHandler, dpHandler, mgmtHandler, telemetry.NewMetrics(), logger)
}

// NewRouterWithTelemetry initializes routes with DataPlaneHandler, ManagementHandler, and Telemetry
func NewRouterWithTelemetry(
	cfg *config.Config,
	healthHandler *health.Handler,
	dpHandler *DataPlaneHandler,
	mgmtHandler *ManagementHandler,
	metrics *telemetry.Metrics,
	logger *slog.Logger,
) http.Handler {
	mux := http.NewServeMux()

	// Health routes
	if healthHandler != nil {
		mux.HandleFunc("GET /health/live", healthHandler.Live)
		mux.HandleFunc("GET /health/ready", healthHandler.Ready)
	}

	// Prometheus Metrics Exporter
	if metrics != nil {
		mux.Handle("GET /metrics", metrics.Handler())
	}

	// Data Plane routes (/v1/*)
	if dpHandler != nil {
		dpHandler.SetMetrics(metrics)

		var listModelsHandler http.Handler = http.HandlerFunc(dpHandler.ListModels)
		var chatHandler http.Handler = http.HandlerFunc(dpHandler.ChatCompletions)

		if dpHandler.keyStore != nil {
			listModelsHandler = auth.AuthMiddleware(dpHandler.keyStore)(listModelsHandler)
			chatHandler = auth.AuthMiddleware(dpHandler.keyStore)(chatHandler)
		}
		if dpHandler.auditStore != nil {
			listModelsHandler = auditAuthFailures(dpHandler.auditStore)(listModelsHandler)
			chatHandler = auditAuthFailures(dpHandler.auditStore)(chatHandler)
		}

		mux.Handle("GET /v1/models", listModelsHandler)
		mux.Handle("POST /v1/chat/completions", chatHandler)
	}

	// Control Plane / Management API routes (/api/v1/*)
	if mgmtHandler != nil {
		adminAuth := AdminAuthMiddleware(cfg.Admin.ManagementToken)
		systemHandler := adminAuth(http.HandlerFunc(mgmtHandler.GetSystem))
		overviewHandler := adminAuth(http.HandlerFunc(mgmtHandler.GetOverview))
		proxiesHandler := adminAuth(http.HandlerFunc(mgmtHandler.GetProxies))
		createProxyHandler := adminAuth(http.HandlerFunc(mgmtHandler.CreateProxy))
		getProxyHandler := adminAuth(http.HandlerFunc(mgmtHandler.GetProxy))
		updateProxyHandler := adminAuth(http.HandlerFunc(mgmtHandler.UpdateProxy))
		deleteProxyHandler := adminAuth(http.HandlerFunc(mgmtHandler.DeleteProxy))

		if mgmtHandler.auditStore != nil {
			auditFailures := auditAuthFailures(mgmtHandler.auditStore)
			systemHandler = auditFailures(systemHandler)
			overviewHandler = auditFailures(overviewHandler)
			proxiesHandler = auditFailures(proxiesHandler)
			createProxyHandler = auditFailures(createProxyHandler)
			getProxyHandler = auditFailures(getProxyHandler)
			updateProxyHandler = auditFailures(updateProxyHandler)
			deleteProxyHandler = auditFailures(deleteProxyHandler)
		}

		mux.Handle("GET /api/v1/system", systemHandler)
		mux.Handle("GET /api/v1/overview", overviewHandler)
		mux.Handle("GET /api/v1/proxies", proxiesHandler)
		mux.Handle("POST /api/v1/proxies", createProxyHandler)
		mux.Handle("GET /api/v1/proxies/{id}", getProxyHandler)
		mux.Handle("PUT /api/v1/proxies/{id}", updateProxyHandler)
		mux.Handle("DELETE /api/v1/proxies/{id}", deleteProxyHandler)
	}

	// Root info endpoint
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"name":"ProxyGateway Enterprise","version":"2.0","status":"operational"}`))
	})

	// Standard middleware stack
	var handler http.Handler = mux
	if metrics != nil {
		handler = metrics.Middleware()(handler)
	}
	handler = LoggingMiddleware(logger)(handler)
	handler = RequestIDMiddleware(handler)
	handler = CORSMiddleware(cfg.Admin.AllowedOrigins)(handler)
	handler = RecoveryMiddleware(handler)

	return handler
}
