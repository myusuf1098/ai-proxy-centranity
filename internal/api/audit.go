package api

import (
	"crypto/rand"
	"encoding/hex"
)

// GenerateAuditID returns a cryptographically random 128-bit hex audit event ID
func GenerateAuditID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
