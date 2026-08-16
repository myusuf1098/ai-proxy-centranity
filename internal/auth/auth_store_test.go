package auth

import (
	"context"
	"testing"
)

func TestMemoryKeyStoreListUpdateDelete(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryKeyStore()
	k1 := &APIKey{ID: "key_1", Name: "one", Prefix: "sk-pg-1", Hash: "h1", Enabled: true}
	if err := s.Create(ctx, k1); err != nil {
		t.Fatalf("create k1: %v", err)
	}
	list, err := s.List(ctx)
	if err != nil || len(list) != 1 {
		t.Fatalf("List len=1 got %d err=%v", len(list), err)
	}
	k1.Enabled = false
	if err := s.Update(ctx, k1); err != nil {
		t.Fatalf("update: %v", err)
	}
	got, _ := s.GetByHash(ctx, "h1")
	if got == nil || got.Enabled {
		t.Fatalf("expected disabled after update, got %+v", got)
	}
	if err := s.Delete(ctx, "h1"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := s.GetByHash(ctx, "h1"); err == nil {
		t.Fatal("expected not found after delete")
	}
}

func TestMemoryKeyStoreListIsCopy(t *testing.T) {
	ctx := context.Background()
	s := NewMemoryKeyStore()
	_ = s.Create(ctx, &APIKey{ID: "k1", Hash: "h", Prefix: "sk"})
	list, _ := s.List(ctx)
	list[0].Name = "mutated"
	got, _ := s.GetByHash(ctx, "h")
	if got.Name == "mutated" {
		t.Fatal("List must return a copy, not the internal slice")
	}
}
