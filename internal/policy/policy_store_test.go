package policy

import (
	"testing"
)

func TestGetGlobalDenyReturnsCopy(t *testing.T) {
	e := NewEngine()
	e.SetGlobalDeny([]string{"cc-opus"}, []string{"openai"})
	m, p := e.GetGlobalDeny()
	m[0] = "mutated"
	m2, _ := e.GetGlobalDeny()
	if m2[0] == "mutated" {
		t.Fatal("GetGlobalDeny must return a copy")
	}
	if len(m) != 1 || len(p) != 1 || m2[0] != "cc-opus" {
		t.Fatalf("unexpected deny: models=%v providers=%v", m2, p)
	}
}
