package textinput

import (
	"image/color"
	"time"

	"mc/internal/event"
	"mc/internal/paint"
)

func DefaultStyles(isDark bool) Styles {
	lightDark := paint.LightDark(isDark)

	var s Styles
	s.Focused = StyleState{
		Placeholder: paint.NewStyle().Foreground(paint.Color("240")),
		Suggestion:  paint.NewStyle().Foreground(paint.Color("240")),
		Prompt:      paint.NewStyle().Foreground(paint.Color("7")),
		Text:        paint.NewStyle(),
	}
	s.Blurred = StyleState{
		Placeholder: paint.NewStyle().Foreground(paint.Color("240")),
		Suggestion:  paint.NewStyle().Foreground(paint.Color("240")),
		Prompt:      paint.NewStyle().Foreground(paint.Color("7")),
		Text:        paint.NewStyle().Foreground(lightDark(paint.Color("245"), paint.Color("7"))),
	}
	s.Cursor = CursorStyle{
		Color: paint.Color("7"),
		Shape: event.CursorBlock,
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
	Text        paint.Style
	Placeholder paint.Style
	Suggestion  paint.Style
	Prompt      paint.Style
}

type CursorStyle struct {
	Color      color.Color
	Shape      event.CursorShape
	Blink      bool
	BlinkSpeed time.Duration
}
