package main

import (
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	overlay "github.com/rmhubbert/bubbletea-overlay"
)

func viewMessages(m *model) string {
	var s strings.Builder

	base := &m.theme.baseStyle
	empty := &m.theme.emptyStyle

	length := len(m.log)
	last := length - 1 - m.logStart
	numbersLength := numberOfDigits(min(m.height, length)+m.logStart) + 1

	for i := 0; i < m.height; i++ {
		if last >= 0 && last < length {
			s.WriteString(base.Width(numbersLength).Foreground(m.theme.accentColor4).Render(
				strconv.Itoa(i + 1 + m.logStart)))
			s.WriteString(
				empty.Width(m.width - numbersLength).Render(
					m.log[last].render(&m.theme, true)))
		} else {
			s.WriteString(base.Width(numbersLength).Render())
			s.WriteString(empty.Width(m.width - numbersLength).Render())
		}
		if i != m.height-1 {
			s.WriteRune('\n')
		}
		last--
	}

	messages := s.String()

	if length == 0 {
		style := base.
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(m.theme.grayColor).
			BorderBackground(base.GetBackground())

		messages = overlay.Composite(style.Render(" The log is empty! "), messages, overlay.Center, overlay.Center, 0, 0)
	}

	return messages
}
