package tui_test

import (
	"strings"
	"testing"

	"github.com/myusuf1098/ai-proxy-centranity/internal/tui"
)

func TestFormRendersFieldsAndNavigates(t *testing.T) {
	f := tui.NewFormState("Add Proxy", []tui.FormField{
		{Label: "Name"},
		{Label: "Host"},
		{Label: "Port"},
	}, nil)
	view := tui.FormView(f)
	if !strings.Contains(view, "Name") || !strings.Contains(view, "Host") {
		t.Fatalf("form should render all field labels, got: %s", view)
	}
}

func TestFormSubmitGathersValues(t *testing.T) {
	var submitted map[string]string
	f := tui.NewFormState("Add", []tui.FormField{{Label: "Name"}}, func(v map[string]string) {
		submitted = v
	})
	f.SetValue(0, "myproxy")
	f.Submit()
	if submitted["Name"] != "myproxy" {
		t.Fatalf("expected submit to gather value, got %v", submitted)
	}
}
