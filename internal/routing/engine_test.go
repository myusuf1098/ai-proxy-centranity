package routing

import (
	"testing"
)

func TestEngineGetAliasesAndDelete(t *testing.T) {
	e := NewEngine(nil)
	e.SetAlias("testalias", []string{"cc-haiku", "cc-sonnet"})
	aliases := e.GetAliases()
	if len(aliases["testalias"]) != 2 {
		t.Fatalf("expected 2 targets, got %v", aliases["testalias"])
	}
	aliases["testalias"][0] = "mutated"
	aliases2 := e.GetAliases()
	if aliases2["testalias"][0] == "mutated" {
		t.Fatal("GetAliases must return a copy")
	}
	if err := e.DeleteAlias("TESTALIAS"); err != nil { // case-insensitive
		t.Fatalf("delete alias: %v", err)
	}
	if _, ok := e.GetAliases()["testalias"]; ok {
		t.Fatal("alias should be deleted")
	}
	if err := e.DeleteAlias("missing"); err == nil {
		t.Fatal("deleting missing alias should error")
	}
}
