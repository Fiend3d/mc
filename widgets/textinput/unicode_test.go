package textinput

import (
	"mc/internal/event"
	"testing"
)

func TestGraphemeEditing(t *testing.T) {
	for _, cluster := range []string{"é", "👩‍💻", "क्ष", "க்ஷ"} {
		t.Run(cluster, func(t *testing.T) {
			m := New()
			m.Focus()
			m.SetValue("a" + cluster + "b")
			m.CursorEnd()
			m, _ = m.Update(event.KeyMsg{Name: "left"})
			m, _ = m.Update(event.KeyMsg{Name: "backspace"})
			if m.Value() != "ab" {
				t.Fatalf("backspace split grapheme: %q", m.Value())
			}
			m.SetValue("a" + cluster + "b")
			m.SetCursor(1)
			m, _ = m.Update(event.KeyMsg{Name: "delete"})
			if m.Value() != "ab" {
				t.Fatalf("delete split grapheme: %q", m.Value())
			}
			m.SetValue("a" + cluster + "b")
			m.CursorEnd()
			m.SetWidth(3)
			_ = m.View()
			m.CursorStart()
			_ = m.View()
		})
	}
}
