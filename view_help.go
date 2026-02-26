package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

func addParagraph(docs []string, m *model, text string, position lipgloss.Position) []string {
	base := &m.theme.baseStyle
	if len(docs) > 0 { // spacing
		docs = append(docs, base.Width(m.width).Render())
	}
	wrap := ansi.Wordwrap(text, m.width, "")
	lines := strings.Split(wrap, "\n")
	for i := range lines {
		docs = append(docs, base.Width(m.width).Align(position).Render(lines[i]))
	}
	return docs
}

func makeHelpHeader(docs []string, m *model, text string) []string {
	if len(m.helpFilter) > 0 {
		return docs
	}
	base := &m.theme.baseStyle
	highlight := base.Bold(true).Foreground(m.theme.accentColor3)
	str := highlight.Render(text)
	docs = addParagraph(docs, m, str, lipgloss.Left)
	return docs
}

func makeDocs(docs []string, m *model, prefix string, text string) []string {
	base := &m.theme.baseStyle
	highlight := base.Bold(true).Foreground(m.theme.accentColor5)
	str := highlight.Render(prefix) + base.Render(text)
	docs = addParagraph(docs, m, str, lipgloss.Left)
	return docs
}

func viewHelp(m *model) string {
	base := &m.theme.baseStyle
	empty := &m.theme.emptyStyle

	docs := make([]string, 0)

	header := fmt.Sprintf(
		" %s (%s) [%s]",
		Version,
		GitCommit,
		BuildTime,
	)

	docs = addParagraph(
		docs,
		m,
		base.Foreground(m.theme.accentColor2).Bold(true).Render("Modal Commander")+base.Render(header),
		lipgloss.Center,
	)

	docs = makeHelpHeader(docs, m, " Normal Mode")
	docs = makeDocs(docs, m, " q", " - Quit, returning the current directory.")
	docs = makeDocs(docs, m, " Q", " - Quit without returning anything.")

	var s strings.Builder

	for i := 0; i < m.height-1; i++ {
		index := i + m.help
		if index < len(docs) {
			s.WriteString(docs[index])
		} else {
			s.WriteString(empty.Width(m.width).Render())
		}
		s.WriteRune('\n')
	}

	help := base.Foreground(m.theme.grayColor).Render("Press ")
	help += base.Foreground(m.theme.accentColor2).Render("F")
	help += base.Foreground(m.theme.grayColor).Render(" to filter")
	s.WriteString(base.Width(m.width).Render(help))

	return s.String()
}
