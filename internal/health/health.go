package health

import (
	"context"
	"encoding/json"
	"net/http"
	"time"
)

// HealthChecker interface for components to report readiness
type HealthChecker interface {
	Name() string
	Check(ctx context.Context) error
}

// Handler provides liveness and readiness HTTP handlers
type Handler struct {
	checkers []HealthChecker
}

// LiveResponse payload for liveness probe
type LiveResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// ReadyResponse payload for readiness probe
type ReadyResponse struct {
	Status    string            `json:"status"`
	Timestamp time.Time         `json:"timestamp"`
	Checks    map[string]string `json:"checks"`
}

// NewHandler creates a new health handler with optional component checkers
func NewHandler(checkers ...HealthChecker) *Handler {
	return &Handler{
		checkers: checkers,
	}
}

// Live handles liveness checks (returns 200 if process is running)
func (h *Handler) Live(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(LiveResponse{
		Status:    "ok",
		Timestamp: time.Now().UTC(),
	})
}

// Ready handles readiness checks (verifies dependencies are healthy)
func (h *Handler) Ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	checks := make(map[string]string)
	allHealthy := true

	for _, c := range h.checkers {
		if err := c.Check(ctx); err != nil {
			checks[c.Name()] = "unhealthy: " + err.Error()
			allHealthy = false
		} else {
			checks[c.Name()] = "healthy"
		}
	}

	status := "ok"
	statusCode := http.StatusOK
	if !allHealthy {
		status = "unhealthy"
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(ReadyResponse{
		Status:    status,
		Timestamp: time.Now().UTC(),
		Checks:    checks,
	})
}
