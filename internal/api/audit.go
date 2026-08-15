package api

import (
	"crypto/rand"
	"encoding/hex"
	"strconv"
	"time"
)

// GenerateAuditID returns a cryptographically random 128-bit hex audit event ID.
// On rand failure it falls back to a nanosecond-timestamp hex to avoid an all-zero ID.
func GenerateAuditID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return strconv.FormatInt(time.Now().UnixNano(), 16)
	}
	return hex.EncodeToString(b)
}
