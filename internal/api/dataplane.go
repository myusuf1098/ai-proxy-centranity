package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/myusuf1098/ai-proxy-centranity/internal/auth"
	"github.com/myusuf1098/ai-proxy-centranity/internal/limiter"
	"github.com/myusuf1098/ai-proxy-centranity/internal/ninerouter"
	"github.com/myusuf1098/ai-proxy-centranity/internal/policy"
	"github.com/myusuf1098/ai-proxy-centranity/internal/routing"
)

// DataPlaneHandler serves OpenAI-compatible client endpoints
type DataPlaneHandler struct {
	adapter       ninerouter.NineRouterPort
	keyStore      auth.KeyStore
	policyEngine  *policy.Engine
	rateLimiter   limiter.RateLimiter
	routingEngine *routing.Engine
	logger        *slog.Logger
}

// NewDataPlaneHandler creates a basic DataPlaneHandler
func NewDataPlaneHandler(adapter ninerouter.NineRouterPort, logger *slog.Logger) *DataPlaneHandler {
	return NewDataPlaneHandlerWithRouting(adapter, nil, policy.NewEngine(), limiter.NewMemoryLimiter(), routing.NewEngine(nil), logger)
}

// NewDataPlaneHandlerWithPolicy creates a DataPlaneHandler with authentication and policy
func NewDataPlaneHandlerWithPolicy(
	adapter ninerouter.NineRouterPort,
	keyStore auth.KeyStore,
	policyEngine *policy.Engine,
	rateLimiter limiter.RateLimiter,
	logger *slog.Logger,
) *DataPlaneHandler {
	return NewDataPlaneHandlerWithRouting(adapter, keyStore, policyEngine, rateLimiter, routing.NewEngine(nil), logger)
}

// NewDataPlaneHandlerWithRouting creates a DataPlaneHandler with full routing and circuit breaker support
func NewDataPlaneHandlerWithRouting(
	adapter ninerouter.NineRouterPort,
	keyStore auth.KeyStore,
	policyEngine *policy.Engine,
	rateLimiter limiter.RateLimiter,
	routingEngine *routing.Engine,
	logger *slog.Logger,
) *DataPlaneHandler {
	if policyEngine == nil {
		policyEngine = policy.NewEngine()
	}
	if rateLimiter == nil {
		rateLimiter = limiter.NewMemoryLimiter()
	}
	if routingEngine == nil {
		routingEngine = routing.NewEngine(nil)
	}
	return &DataPlaneHandler{
		adapter:       adapter,
		keyStore:      keyStore,
		policyEngine:  policyEngine,
		rateLimiter:   rateLimiter,
		routingEngine: routingEngine,
		logger:        logger,
	}
}

// OpenAIModel represents an OpenAI-compatible model entry
type OpenAIModel struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	OwnedBy string `json:"owned_by"`
}

// OpenAIModelListResponse represents the list response format
type OpenAIModelListResponse struct {
	Object string        `json:"object"`
	Data   []OpenAIModel `json:"data"`
}

// OpenAIError represents standard OpenAI error format
type OpenAIError struct {
	Error struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error"`
}

// ListModels handles GET /v1/models
func (h *DataPlaneHandler) ListModels(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	key := auth.GetAPIKey(ctx)
	if h.keyStore != nil && key == nil {
		h.writeError(w, http.StatusUnauthorized, "Authentication required", "auth_error", "unauthorized")
		return
	}

	models, err := h.adapter.ListModels(ctx)
	if err != nil {
		h.writeError(w, http.StatusBadGateway, "failed to retrieve upstream models: "+err.Error(), "upstream_error", "upstream_unavailable")
		return
	}

	response := OpenAIModelListResponse{
		Object: "list",
		Data:   make([]OpenAIModel, 0, len(models)),
	}

	now := time.Now().Unix()
	for _, m := range models {
		if key != nil && h.policyEngine != nil {
			decision := h.policyEngine.EvaluateModel(ctx, key, m.ID)
			if !decision.Allowed {
				continue
			}
		}
		response.Data = append(response.Data, OpenAIModel{
			ID:      m.ID,
			Object:  "model",
			Created: now,
			OwnedBy: m.OwnedBy,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_ = json.NewEncoder(w).Encode(response)
}

// ChatCompletions handles POST /v1/chat/completions
func (h *DataPlaneHandler) ChatCompletions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "method not allowed", "invalid_request_error", "method_not_allowed")
		return
	}

	key := auth.GetAPIKey(r.Context())
	if h.keyStore != nil && key == nil {
		h.writeError(w, http.StatusUnauthorized, "Authentication required", "auth_error", "unauthorized")
		return
	}

	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		h.writeError(w, http.StatusBadRequest, "failed to read request body", "invalid_request_error", "bad_request")
		return
	}
	defer r.Body.Close()

	if len(bodyBytes) == 0 {
		h.writeError(w, http.StatusBadRequest, "request body cannot be empty", "invalid_request_error", "empty_body")
		return
	}

	var parsed struct {
		Model    string `json:"model"`
		Stream   bool   `json:"stream"`
		Messages []any  `json:"messages"`
	}
	if err := json.Unmarshal(bodyBytes, &parsed); err != nil {
		h.writeError(w, http.StatusBadRequest, "invalid JSON payload: "+err.Error(), "invalid_request_error", "invalid_json")
		return
	}

	if parsed.Model == "" {
		h.writeError(w, http.StatusBadRequest, "field 'model' is required", "invalid_request_error", "missing_model")
		return
	}

	// 1. Resolve Model Alias & Routing Decision
	targetModel := parsed.Model
	if h.routingEngine != nil {
		decision, err := h.routingEngine.Resolve(r.Context(), parsed.Model)
		if err != nil {
			h.writeError(w, http.StatusServiceUnavailable, "Routing error: "+err.Error(), "routing_error", "routing_failed")
			return
		}
		targetModel = decision.TargetModel
	}

	// 2. Evaluate model policy
	if key != nil && h.policyEngine != nil {
		decision := h.policyEngine.EvaluateModel(r.Context(), key, targetModel)
		if !decision.Allowed {
			h.writeError(w, http.StatusForbidden, fmt.Sprintf("Access to model '%s' is denied by policy (%s)", targetModel, decision.Reason), "policy_error", "model_not_allowed")
			return
		}
	}

	// 3. Evaluate rate limits
	if key != nil && h.rateLimiter != nil && key.RPMLimit > 0 {
		allowed, remaining, retryAfter := h.rateLimiter.Allow(r.Context(), key.ID, key.RPMLimit)
		w.Header().Set("X-RateLimit-Limit-RPM", fmt.Sprintf("%d", key.RPMLimit))
		w.Header().Set("X-RateLimit-Remaining", fmt.Sprintf("%d", remaining))
		if !allowed {
			w.Header().Set("Retry-After", fmt.Sprintf("%d", int(retryAfter.Seconds())+1))
			h.writeError(w, http.StatusTooManyRequests, fmt.Sprintf("Rate limit exceeded (%d RPM). Retry after %v", key.RPMLimit, retryAfter), "rate_limit_error", "rate_limited")
			return
		}
	}

	// Rewrite model in payload to resolved targetModel
	var payloadMap map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &payloadMap); err == nil {
		payloadMap["model"] = targetModel
		if updatedBytes, err := json.Marshal(payloadMap); err == nil {
			bodyBytes = updatedBytes
		}
	}

	// 4. Forward to 9Router adapter
	resp, err := h.adapter.ForwardChatCompletion(r.Context(), bytes.NewReader(bodyBytes), r.Header)
	if err != nil {
		if h.routingEngine != nil {
			h.routingEngine.RecordResult(targetModel, false)
		}
		h.logger.Error("upstream forward failed", slog.Any("error", err), slog.String("request_id", GetRequestID(r.Context())))
		h.writeError(w, http.StatusBadGateway, "upstream gateway error: "+err.Error(), "upstream_error", "upstream_unavailable")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		if h.routingEngine != nil {
			h.routingEngine.RecordResult(targetModel, false)
		}
	} else {
		if h.routingEngine != nil {
			h.routingEngine.RecordResult(targetModel, true)
		}
	}

	contentType := resp.Header.Get("Content-Type")
	isStream := parsed.Stream || strings.Contains(contentType, "text/event-stream")

	// Copy upstream response headers
	for key, values := range resp.Header {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}

	if isStream {
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.WriteHeader(resp.StatusCode)

		flusher, ok := w.(http.Flusher)
		buf := make([]byte, 4096)

		for {
			select {
			case <-r.Context().Done():
				return
			default:
				n, readErr := resp.Body.Read(buf)
				if n > 0 {
					_, _ = w.Write(buf[:n])
					if ok {
						flusher.Flush()
					}
				}
				if readErr != nil {
					return
				}
			}
		}
	} else {
		w.WriteHeader(resp.StatusCode)
		_, _ = io.Copy(w, resp.Body)
	}
}

func (h *DataPlaneHandler) writeError(w http.ResponseWriter, statusCode int, message, errType, errCode string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	errObj := OpenAIError{}
	errObj.Error.Message = message
	errObj.Error.Type = errType
	errObj.Error.Code = errCode
	_ = json.NewEncoder(w).Encode(errObj)
}
