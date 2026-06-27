package textinput

import (
	"image/color"
	"time"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

func DefaultStyles(isDark bool) Styles {
	lightDark := lipgloss.LightDark(isDark)

	var s Styles
	s.Focused = StyleState{
		Placeholder: lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		Suggestion:  lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		Prompt:      lipgloss.NewStyle().Foreground(lipgloss.Color("7")),
		Text:        lipgloss.NewStyle(),
	}
	s.Blurred = StyleState{
		Placeholder: lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		Suggestion:  lipgloss.NewStyle().Foreground(lipgloss.Color("240")),
		Prompt:      lipgloss.NewStyle().Foreground(lipgloss.Color("7")),
		Text:        lipgloss.NewStyle().Foreground(lightDark(lipgloss.Color("245"), lipgloss.Color("7"))),
	}
	s.Cursor = CursorStyle{
		Color: lipgloss.Color("7"),
		Shape: tea.CursorBlock,
		Blink: true,
	}
	return s
}

func DefaultLightStyles() Styles {
	return DefaultStyles(false)
}

func DefaultDarkStyles() Styles {
	return DefaultStyles(true)
}

type Styles struct {
	Focused StyleState
	Blurred StyleState
	Cursor  CursorStyle
}

type StyleState struct {
	Text        lipgloss.Style
	Placeholder lipgloss.Style
	Suggestion  lipgloss.Style
	Prompt      lipgloss.Style
}

type CursorStyle struct {
	Color      color.Color
	Shape      tea.CursorShape
	Blink      bool
	BlinkSpeed time.Duration
}
