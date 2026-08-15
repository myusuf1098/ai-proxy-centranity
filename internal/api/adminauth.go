package api

import (
	"crypto/subtle"
	"encoding/json"
	"net/http"
)

// AdminAuthMiddleware protects Management API routes with a shared admin token.
// Fail-closed: empty or mismatched token -> 401.
func AdminAuthMiddleware(token string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !validAdminToken(token, r.Header.Get("Authorization")) {
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(http.StatusUnauthorized)
				_ = json.NewEncoder(w).Encode(map[string]interface{}{
					"error": map[string]string{
						"message": "Unauthorized",
						"type":    "auth_error",
						"code":    "unauthorized",
					},
				})
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}

func validAdminToken(configured, header string) bool {
	if configured == "" {
		return false // fail-closed
	}
	const prefix = "Bearer "
	if len(header) <= len(prefix) || header[:len(prefix)] != prefix {
		return false
	}
	got := header[len(prefix):]
	return subtle.ConstantTimeCompare([]byte(got), []byte(configured)) == 1
}
