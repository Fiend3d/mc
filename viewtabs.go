package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

func viewTabs(m *model) string {
	var s strings.Builder

	base := &m.theme.baseStyle
	empty := &m.theme.emptyStyle

	header := fmt.Sprintf(" %d tabs", len(m.tabs))

	s.WriteString(empty.Width(m.width).Render(header))
	s.WriteRune('\n')

	for i := range m.tabs {
		if i+1 > m.height-2 || i+m.tabsStart >= len(m.tabs) {
			break
		}

		index := i + m.tabsStart
		current := index == m.tabsCursor

		style := base

		if current {
			style = &m.theme.cursorStyle
		}

		if index == m.currentTab {
			if current {
				s.WriteString(style.Foreground(m.theme.grayColor).Render("["))
				s.WriteString(style.Bold(true).Render(">"))
				s.WriteString(style.Foreground(m.theme.grayColor).Render("] "))
			} else {
				s.WriteString(style.Foreground(m.theme.grayColor).Render("[ ] "))
			}
		} else {
			if current {
				s.WriteString(style.Render(" >  "))
			} else {
				s.WriteString(style.Render("    "))
			}
		}

		text := ansi.Truncate(m.tabs[index].dir, m.width-4, "…")
		s.WriteString(style.Width(m.width - 4).Render(text))
		s.WriteRune('\n')
	}

	return s.String()
}
