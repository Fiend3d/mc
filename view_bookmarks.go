package main

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
)

func viewBookmarks(m *model) string {
	var s strings.Builder

	base := &m.theme.baseStyle
	empty := &m.theme.emptyStyle

	bookmarks := "bookmarks"
	if len(m.bm.dirs) == 1 {
		bookmarks = "bookmark"
	}
	header := fmt.Sprintf(" %d %s", len(m.bm.dirs), bookmarks)

	s.WriteString(empty.Width(m.width).Bold(true).Foreground(m.theme.accentColor3).Render(header))
	s.WriteRune('\n')

	countLines := 0

	for i := range m.bm.dirs {
		if i+1 > m.height-2 || i+m.bm.start >= len(m.bm.dirs) {
			break
		}

		index := i + m.bm.start

		style := base
		cursorWidth := 3

		cursor := "   "

		if index == m.bm.cursor {
			style = &m.theme.cursorStyle
			cursor = " > "
		}

		prefix := fmt.Sprintf("[%d] ", i+m.bm.start+1)
		prefixWidth := cursorWidth + len(prefix)

		s.WriteString(style.Bold(true).Render(cursor))
		s.WriteString(style.Foreground(m.theme.grayColor).Render(prefix))

		text := m.bm.dirs[index]
		text = truncate(text, m.width-prefixWidth)

		textLength := lipgloss.Width(text)
		coloredText := colorizeDir(text, *style, style.Foreground(m.theme.accentColor4), textLength)
		s.WriteString(style.Width(m.width - prefixWidth).Render(coloredText))
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
	help += gray.Render("- delete")
	if m.bm.cursor > 0 {
		help += empty.Render(" K ")
		help += gray.Render("- move up")
	}
	if m.bm.cursor < len(m.bm.dirs)-1 {
		help += empty.Render(" J ")
		help += gray.Render("- move down")
	}

	help = truncate(help, m.width)

	s.WriteString(empty.Foreground(m.theme.grayColor).Width(m.width).Render(help))

	return s.String()
}
