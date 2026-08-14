package audit_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/myusuf1098/ai-proxy-centranity/internal/audit"
)

func TestAuditLog_Redaction(t *testing.T) {
	store := audit.NewMemoryStore()
	ctx := context.Background()

	event := audit.Event{
		ID:        "evt_001",
		Timestamp: time.Now().UTC(),
		Actor:     "admin",
		EventType: "CREATE_KEY",
		Target:    "key_123",
		Status:    "SUCCESS",
		Metadata: map[string]string{
			"raw_key":  "sk-pg-secret-token-12345",
			"password": "my_super_secret_password",
			"comment":  "Created production key",
		},
	}

	if err := store.Log(ctx, event); err != nil {
		t.Fatalf("failed to record audit event: %v", err)
	}

	events, err := store.List(ctx)
	if err != nil {
		t.Fatalf("failed to list audit events: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 audit event, got %d", len(events))
	}

	logged := events[0]
	for k, v := range logged.Metadata {
		if strings.Contains(strings.ToLower(k), "key") || strings.Contains(strings.ToLower(k), "password") || strings.Contains(strings.ToLower(k), "token") {
			if v != "[REDACTED]" {
				t.Errorf("expected secret field %s to be redacted, got: %s", k, v)
			}
		}
	}
}
