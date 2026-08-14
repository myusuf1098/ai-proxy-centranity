package api

import (
	"log/slog"
	"net/http"

	"github.com/myusuf1098/ai-proxy-centranity/internal/auth"
	"github.com/myusuf1098/ai-proxy-centranity/internal/config"
	"github.com/myusuf1098/ai-proxy-centranity/internal/health"
	"github.com/myusuf1098/ai-proxy-centranity/internal/ninerouter"
)

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
	return NewRouterWithDataPlane(cfg, healthHandler, dpHandler, logger)
}

// NewRouterWithDataPlane initializes routes with DataPlaneHandler and optional auth
func NewRouterWithDataPlane(cfg *config.Config, healthHandler *health.Handler, dpHandler *DataPlaneHandler, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	// Health routes
	if healthHandler != nil {
		mux.HandleFunc("GET /health/live", healthHandler.Live)
		mux.HandleFunc("GET /health/ready", healthHandler.Ready)
	}

	// Data Plane routes
	if dpHandler != nil {
		var listModelsHandler http.Handler = http.HandlerFunc(dpHandler.ListModels)
		var chatHandler http.Handler = http.HandlerFunc(dpHandler.ChatCompletions)

		if dpHandler.keyStore != nil {
			listModelsHandler = auth.AuthMiddleware(dpHandler.keyStore)(listModelsHandler)
			chatHandler = auth.AuthMiddleware(dpHandler.keyStore)(chatHandler)
		}

		mux.Handle("GET /v1/models", listModelsHandler)
		mux.Handle("POST /v1/chat/completions", chatHandler)
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
	handler = LoggingMiddleware(logger)(handler)
	handler = RequestIDMiddleware(handler)
	handler = CORSMiddleware(cfg.Admin.AllowedOrigins)(handler)
	handler = RecoveryMiddleware(handler)

	return handler
}
