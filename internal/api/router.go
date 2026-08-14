package api

import (
	"log/slog"
	"net/http"

	"github.com/myusuf1098/ai-proxy-centranity/internal/config"
	"github.com/myusuf1098/ai-proxy-centranity/internal/health"
)

// NewRouter initializes and wires up HTTP routes with standard middleware
func NewRouter(cfg *config.Config, healthHandler *health.Handler, logger *slog.Logger) http.Handler {
	mux := http.NewServeMux()

	// Health routes
	if healthHandler != nil {
		mux.HandleFunc("GET /health/live", healthHandler.Live)
		mux.HandleFunc("GET /health/ready", healthHandler.Ready)
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

	// Wrap mux with standard middleware stack (Recovery -> CORS -> RequestID -> Logging)
	var handler http.Handler = mux
	handler = LoggingMiddleware(logger)(handler)
	handler = RequestIDMiddleware(handler)
	handler = CORSMiddleware(cfg.Admin.AllowedOrigins)(handler)
	handler = RecoveryMiddleware(handler)

	return handler
}
