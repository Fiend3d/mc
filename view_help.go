package main

import (
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func viewHelp(m *model) string {
	var s strings.Builder
	text := "This is a long text that needs precise wrapping control with custom width."

	base := &m.theme.baseStyle
	// empty := &m.theme.emptyStyle
	s.WriteString(base.Width(m.width).Render(ansi.Wordwrap(text, m.width, "")))
	s.WriteRune('\n')
	s.WriteString(base.Width(m.width).Render(ansi.Wordwrap(text, m.width, "")))
	s.WriteRune('\n')

	return s.String()
}
