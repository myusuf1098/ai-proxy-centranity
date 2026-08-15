package tui

import (
	"strings"

	"github.com/charmbracelet/lipgloss"
)

// FormField is a single labeled input in a modal form.
type FormField struct {
	Label string
	// Secret masks the input value (e.g. proxy password).
	Secret bool
	// Value is the current input content.
	Value string
	// Focused indicates the field is the active cursor.
	Focused bool
}

// FormState is the modal form model.
type FormState struct {
	Title    string
	Fields   []FormField
	Focused  int
	OnSubmit func(values map[string]string)
}

// NewFormState creates a form. onSubmit is invoked with gathered values.
func NewFormState(title string, fields []FormField, onSubmit func(map[string]string)) *FormState {
	for i := range fields {
		if i == 0 {
			fields[i].Focused = true
		}
	}
	return &FormState{Title: title, Fields: fields, Focused: 0, OnSubmit: onSubmit}
}

// SetValue sets field i's value.
func (f *FormState) SetValue(i int, v string) {
	if i >= 0 && i < len(f.Fields) {
		f.Fields[i].Value = v
	}
}

// Submit gathers all values and invokes OnSubmit if set.
func (f *FormState) Submit() {
	if f.OnSubmit == nil {
		return
	}
	vals := make(map[string]string, len(f.Fields))
	for _, fl := range f.Fields {
		vals[fl.Label] = fl.Value
	}
	f.OnSubmit(vals)
}

// NextFocus moves focus to the next field.
func (f *FormState) NextFocus() {
	if len(f.Fields) == 0 {
		return
	}
	f.Fields[f.Focused].Focused = false
	f.Focused = (f.Focused + 1) % len(f.Fields)
	f.Fields[f.Focused].Focused = true
}

// PrevFocus moves focus to the previous field.
func (f *FormState) PrevFocus() {
	if len(f.Fields) == 0 {
		return
	}
	f.Fields[f.Focused].Focused = false
	f.Focused = (f.Focused - 1 + len(f.Fields)) % len(f.Fields)
	f.Fields[f.Focused].Focused = true
}

// FormView renders the modal overlay above the active screen.
func FormView(f *FormState) string {
	var b strings.Builder
	b.WriteString(lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("212")).Render(f.Title) + "\n\n")
	for _, fl := range f.Fields {
		cursor := "  "
		if fl.Focused {
			cursor = "> "
		}
		val := fl.Value
		if fl.Secret && val != "" {
			val = strings.Repeat("*", len(val))
		}
		b.WriteString(cursor + fl.Label + ": " + val + "\n")
	}
	b.WriteString("\n[Enter] Submit   [Tab] Next   [Esc] Cancel\n")
	return lipgloss.NewStyle().Border(lipgloss.RoundedBorder()).Padding(1).Render(b.String())
}
