package api

import (
	"bytes"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/myusuf1098/ai-proxy-centranity/internal/ninerouter"
)

// DataPlaneHandler serves OpenAI-compatible client endpoints
type DataPlaneHandler struct {
	adapter ninerouter.NineRouterPort
	logger  *slog.Logger
}

// NewDataPlaneHandler creates a new DataPlaneHandler
func NewDataPlaneHandler(adapter ninerouter.NineRouterPort, logger *slog.Logger) *DataPlaneHandler {
	return &DataPlaneHandler{
		adapter: adapter,
		logger:  logger,
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

	// Forward to 9Router adapter
	resp, err := h.adapter.ForwardChatCompletion(r.Context(), bytes.NewReader(bodyBytes), r.Header)
	if err != nil {
		h.logger.Error("upstream forward failed", slog.Any("error", err), slog.String("request_id", GetRequestID(r.Context())))
		h.writeError(w, http.StatusBadGateway, "upstream gateway error: "+err.Error(), "upstream_error", "upstream_unavailable")
		return
	}
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
				// Client disconnected
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
