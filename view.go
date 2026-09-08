package main

import (
	"mc/internal/paint"
	"strconv"
	"strings"
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
			logMsg := m.log[last].render(m.theme, true)
			if paint.Width(logMsg) > m.width-numbersLength {
				logMsg = truncate(logMsg, m.width-numbersLength)
			}
			s.WriteString(
				empty.Width(m.width - numbersLength).Render(logMsg))
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
		return base.Render(" The log is empty! ")
	}

	return messages
}
