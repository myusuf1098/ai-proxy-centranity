package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/myusuf1098/ai-proxy-centranity/internal/audit"
	"github.com/myusuf1098/ai-proxy-centranity/internal/auth"
	"github.com/myusuf1098/ai-proxy-centranity/internal/limiter"
	"github.com/myusuf1098/ai-proxy-centranity/internal/ninerouter"
	"github.com/myusuf1098/ai-proxy-centranity/internal/policy"
	"github.com/myusuf1098/ai-proxy-centranity/internal/quota"
	"github.com/myusuf1098/ai-proxy-centranity/internal/routing"
	"github.com/myusuf1098/ai-proxy-centranity/internal/telemetry"
)

// DataPlaneHandler serves OpenAI-compatible client endpoints
type DataPlaneHandler struct {
	adapter       ninerouter.NineRouterPort
	keyStore      auth.KeyStore
	policyEngine  *policy.Engine
	rateLimiter   limiter.RateLimiter
	routingEngine *routing.Engine
	logger        *slog.Logger
	auditStore    audit.Store
	quotaStore    quota.QuotaStore
	metrics       *telemetry.Metrics
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
		quotaStore:    quota.NewMemoryQuota(),
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
	startTime := time.Now()

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
	var decision *routing.RouteDecision
	if h.routingEngine != nil {
		decision, err = h.routingEngine.Resolve(r.Context(), parsed.Model)
		if err != nil {
			h.writeError(w, http.StatusServiceUnavailable, "Routing error: "+err.Error(), "routing_error", "routing_failed")
			return
		}
		targetModel = decision.TargetModel
	}
	if decision != nil {
		h.logAudit(r.Context(), audit.EventRouteResolved, keyIDOrUnknown(r), targetModel, "resolved",
			map[string]string{"requested": parsed.Model, "strategy": decision.Reason})
	}

	// 2. Evaluate model policy
	if key != nil && h.policyEngine != nil {
		decision := h.policyEngine.EvaluateModel(r.Context(), key, targetModel)
		if !decision.Allowed {
			h.logAudit(r.Context(), audit.EventPolicyDeny, keyIDOrUnknown(r), "chat.completions", "forbidden",
				map[string]string{"model": targetModel})
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
			h.logAudit(r.Context(), audit.EventRateLimited, keyIDOrUnknown(r), "chat.completions", "rate_limited",
				map[string]string{"limit": "rpm"})
			h.writeError(w, http.StatusTooManyRequests, fmt.Sprintf("Rate limit exceeded (%d RPM). Retry after %v", key.RPMLimit, retryAfter), "rate_limit_error", "rate_limited")
			return
		}
	}

	// 3b. Enforce token quotas
	if key != nil && h.quotaStore != nil {
		// Estimate tokens from messages if possible, else conservative 0 (no limit applied)
		estimated := estimateTokens(parsed.Messages)
		allowed, dailyRem, monthlyRem := h.quotaStore.Allow(r.Context(), key.ID, key.DailyTokenQuota, key.MonthlyTokenQuota, estimated)
		w.Header().Set("X-RateLimit-Quota-Daily-Remaining", fmt.Sprintf("%d", dailyRem))
		w.Header().Set("X-RateLimit-Quota-Monthly-Remaining", fmt.Sprintf("%d", monthlyRem))
		if !allowed {
			h.writeError(w, http.StatusTooManyRequests, "Token quota exceeded", "quota_error", "quota_exceeded")
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

	// 4. Forward to 9Router adapter, with fallback on upstream failure
	targets := []string{targetModel}
	if decision != nil && len(decision.FallbackChain) > 0 {
		targets = append(targets, decision.FallbackChain...)
	}

	var lastResp *http.Response
	var lastErr error
	forwarded := false
	successModel := targetModel

	for _, t := range targets {
		// Rewrite model per target
		payloadMap["model"] = t
		if updatedBytes, err := json.Marshal(payloadMap); err == nil {
			bodyBytes = updatedBytes
		}

		resp, err := h.adapter.ForwardChatCompletion(r.Context(), bytes.NewReader(bodyBytes), r.Header)
		if err != nil {
			if h.routingEngine != nil {
				h.routingEngine.RecordResult(t, false)
			}
			h.logger.Warn("upstream forward failed, trying fallback", slog.Any("error", err), slog.String("model", t), slog.String("request_id", GetRequestID(r.Context())))
			lastErr = err
			continue
		}

		if resp.StatusCode >= 500 {
			if h.routingEngine != nil {
				h.routingEngine.RecordResult(t, false)
			}
			h.logger.Warn("upstream 5xx, trying fallback", slog.Int("status", resp.StatusCode), slog.String("model", t), slog.String("request_id", GetRequestID(r.Context())))
			resp.Body.Close()
			lastErr = fmt.Errorf("upstream returned %d for %s", resp.StatusCode, t)
			continue
		}

		if h.routingEngine != nil {
			h.routingEngine.RecordResult(t, true)
		}
		lastResp = resp
		forwarded = true
		successModel = t
		break
	}

	if !forwarded {
		if h.metrics != nil {
			h.metrics.UpstreamErrors.WithLabelValues("9router", "upstream_unavailable").Inc()
		}
		if lastErr != nil {
			h.writeError(w, http.StatusBadGateway, "upstream gateway error: "+lastErr.Error(), "upstream_error", "upstream_unavailable")
		} else {
			h.writeError(w, http.StatusBadGateway, "upstream gateway error", "upstream_error", "upstream_unavailable")
		}
		return
	}

	// Record token usage + latency for the model that actually succeeded
	if h.metrics != nil {
		h.metrics.TokensTotal.WithLabelValues(keyIDOrUnknown(r), successModel, "output").Add(float64(estimateTokens(parsed.Messages)))
		h.metrics.RequestDuration.WithLabelValues("/v1/chat/completions", successModel).Observe(time.Since(startTime).Seconds())
	}

	// Record token usage for the model that actually succeeded
	if key != nil && h.quotaStore != nil {
		h.quotaStore.Record(r.Context(), key.ID, estimateTokens(parsed.Messages))
	}

	resp := lastResp
	defer resp.Body.Close()

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

// estimateTokens: rough heuristic, ponytail: chars/4, upgrade when usage API lands
func estimateTokens(messages []any) int64 {
	if len(messages) == 0 {
		return 0
	}
	// conservative: count roughly 4 chars per token across serialized messages
	b, err := json.Marshal(messages)
	if err != nil {
		return 0
	}
	return int64(len(b) / 4)
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

// SetAuditStore injects the audit trail store
func (h *DataPlaneHandler) SetAuditStore(s audit.Store) { h.auditStore = s }

// SetMetrics injects the telemetry metrics collectors
func (h *DataPlaneHandler) SetMetrics(m *telemetry.Metrics) { h.metrics = m }

// logAudit emits an audit event when an audit store is configured
func (h *DataPlaneHandler) logAudit(ctx context.Context, eventType, actor, target, status string, meta map[string]string) {
	emitAudit(ctx, h.auditStore, eventType, actor, target, status, meta)
}

// emitAudit writes an audit event to the store, ignoring nil stores and write errors
func emitAudit(ctx context.Context, store audit.Store, eventType, actor, target, status string, meta map[string]string) {
	if store == nil {
		return
	}
	_ = store.Log(ctx, audit.Event{
		ID:        GenerateAuditID(),
		Timestamp: time.Now().UTC(),
		Actor:     actor,
		EventType: eventType,
		Target:    target,
		Status:    status,
		Metadata:  meta,
	})
}

// keyIDOrUnknown returns the authenticated key ID or "unknown" for unauthenticated actors
func keyIDOrUnknown(r *http.Request) string {
	if key := auth.GetAPIKey(r.Context()); key != nil {
		return key.ID
	}
	return "unknown"
}
