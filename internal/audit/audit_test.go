package audit_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/myusuf1098/ai-proxy-centranity/internal/audit"
)

func TestMemoryStore_CapEnforced(t *testing.T) {
	const cap = 5
	store := audit.NewMemoryStoreWithLimit(cap)
	ctx := context.Background()

	for i := 0; i < cap+3; i++ {
		if err := store.Log(ctx, audit.Event{ID: string(rune('a' + i)), EventType: "TEST", Timestamp: time.Now().UTC()}); err != nil {
			t.Fatalf("failed to record event %d: %v", i, err)
		}
	}

	events, err := store.List(ctx)
	if err != nil {
		t.Fatalf("failed to list audit events: %v", err)
	}
	if len(events) != cap {
		t.Fatalf("expected %d events, got %d", cap, len(events))
	}
	// Oldest (a, b, c) dropped; newest cap retained
	wantFirst := "d"
	if events[0].ID != wantFirst {
		t.Errorf("expected first retained event %q, got %q", wantFirst, events[0].ID)
	}
	wantLast := string(rune('a' + cap + 3 - 1))
	if events[len(events)-1].ID != wantLast {
		t.Errorf("expected last event %q, got %q", wantLast, events[len(events)-1].ID)
	}
}

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
