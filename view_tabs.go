package main

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

func viewTabs(m *model) string {
	var s strings.Builder

	base := &m.theme.baseStyle
	empty := &m.theme.emptyStyle

	tabs := "tabs"
	if len(m.tabs) == 1 {
		tabs = "tab"
	}
	header := fmt.Sprintf(" %d %s", len(m.tabs), tabs)

	s.WriteString(empty.Width(m.width).Bold(true).Foreground(m.theme.accentColor3).Render(header))
	s.WriteRune('\n')

	countLines := 0

	for i := range m.tabs {
		if i+1 > m.height-2 || i+m.tabsStart >= len(m.tabs) {
			break
		}

		index := i + m.tabsStart

		style := base
		cursorWidth := 3

		cursor := "   "
		current := index == m.currentTab

		if index == m.tabsCursor {
			style = &m.theme.cursorStyle
			cursor = " > "
		}

		prefix := fmt.Sprintf("[%d] ", i+m.tabsStart+1)
		prefixWidth := cursorWidth + len(prefix)

		s.WriteString(style.Bold(true).Render(cursor))
		if current {
			s.WriteString(style.Render(prefix))
		} else {
			s.WriteString(style.Foreground(m.theme.grayColor).Render(prefix))
		}

		text := m.tabs[index].dir
		tag := ""
		if current {
			tag += style.Foreground(m.theme.grayColor).Render(" [current] ")
		}
		tagLength := lipgloss.Width(tag)
		text = truncate(text, m.width-prefixWidth-tagLength)

		textLength := lipgloss.Width(text)
		var coloredText string
		if current {
			coloredText = colorizeDir(text, *style, style.Foreground(m.theme.accentColor5), textLength)
		} else {
			coloredText = colorizeDir(text, *style, style.Foreground(m.theme.accentColor4), textLength)
		}
		s.WriteString(style.Width(m.width - prefixWidth - tagLength).Render(coloredText))
		if tagLength > 0 {
			s.WriteString(tag)
		}
		s.WriteRune('\n')
		countLines++
	}

	for i := countLines; i < m.height-2; i++ {
		s.WriteString(empty.Width(m.width).Render(" "))
		s.WriteRune('\n')
	}

	gray := empty.Foreground(m.theme.grayColor)

	help := gray.Render("Keys:")
	help += empty.Render(" d ")
	help += gray.Render("- close")
	help += empty.Render(" c ")
	help += gray.Render("- copy path")
	if len(m.tabs) > 1 {
		help += empty.Render(" a ")
		help += gray.Render("- close all")
	}
	if m.tabsCursor > 0 {
		help += empty.Render(" K ")
		help += gray.Render("- move up")
	}
	if m.tabsCursor < len(m.tabs)-1 {
		help += empty.Render(" J ")
		help += gray.Render("- move down")
	}
	if len(m.closedTabs) > 0 {
		help += empty.Render(" u ")
		help += gray.Render("- restore")
	}

	help = truncate(help, m.width)

	s.WriteString(empty.Foreground(m.theme.grayColor).Width(m.width).Render(help))

	return s.String()
}
