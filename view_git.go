package main

import (
	"fmt"
	"strings"
)

// viewGitList lists the files behind one tally, in the shape of the bookmarks
// and tabs overlays: a counted header, a cursor down the left, and the keys
// that act on the highlighted row along the bottom.
func viewGitList(m *model) string {
	var s strings.Builder

	base := &m.theme.baseStyle
	empty := &m.theme.emptyStyle
	list := &m.gitList

	header := fmt.Sprintf(" %d %s", len(list.entries), tallyName(list.tally))
	if len(list.entries) == 1 {
		header = fmt.Sprintf(" 1 %s file", tallyName(list.tally))
	}
	if list.branch != "" {
		header += " · " + list.branch
	}
	s.WriteString(empty.Width(m.width).Bold(true).Foreground(m.theme.accentColor3).Render(truncate(header, m.width)))
	s.WriteRune('\n')

	rows := max(1, m.height-2)
	list.keepCursor(m.height)
	drawn := 0
	for i := list.start; i < len(list.entries) && drawn < rows; i, drawn = i+1, drawn+1 {
		entry := list.entries[i]
		style := base
		cursor := "   "
		if i == list.cursor {
			style = &m.theme.cursorStyle
			cursor = " > "
		}
		if i == m.hoverGitIndex {
			style = &m.theme.selectionStyle
		}
		prefix := fmt.Sprintf("[%d] ", i+1)
		letter := style.Bold(true).Foreground(m.gitColor(entry.state)).Render(entry.state.letter() + " ")
		width := len(cursor) + len(prefix) + 2

		// Paths are shown from the repository root: the root itself is already
		// in the header, and the part that matters is where inside it the file
		// sits.
		text := entry.path
		if rest, ok := relativeTo(entry.path, list.root); ok {
			text = rest
		}
		text = truncate(text, max(1, m.width-width))

		s.WriteString(style.Bold(true).Render(cursor))
		s.WriteString(style.Foreground(m.theme.grayColor).Render(prefix))
		s.WriteString(letter)
		s.WriteString(style.Width(max(1, m.width-width)).Render(
			colorizeDir(text, *style, style.Foreground(m.theme.whiteColor), max(1, m.width-width))))
		s.WriteRune('\n')
	}
	for ; drawn < rows; drawn++ {
		s.WriteString(empty.Width(m.width).Render(" "))
		s.WriteRune('\n')
	}

	gray := empty.Foreground(m.theme.grayColor)
	help := gray.Render(" Keys:")
	help += empty.Render(" enter ")
	help += gray.Render("- jump to the file")
	help += empty.Render(" F2-F12 ")
	help += gray.Render("- tools")
	help += empty.Render(" Esc ")
	help += gray.Render("- close")
	s.WriteString(gray.Width(m.width).Render(truncate(help, m.width)))

	return s.String()
}

// gitListRowAtY maps a screen row to an entry, for hover and clicks. The first
// row is the header and the last is the key line, so neither is an entry.
func (m *model) gitListRowAtY(y int) int {
	index := y - 1 + m.gitList.start
	if y < 1 || y >= m.height-1 || index < 0 || index >= len(m.gitList.entries) {
		return -1
	}
	return index
}
