package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/ansi"
)

type helpTopic struct {
	header string
	docs   []string
}

func addTopic(docs []string, topic *helpTopic, m *model) []string {
	if len(topic.docs) == 0 {
		return docs
	}
	base := &m.theme.baseStyle
	if len(docs) > 0 { // spacing
		docs = append(docs, base.Width(m.width).Render())
	}
	highlight := base.Bold(true).Foreground(m.theme.accentColor3)
	str := highlight.Render(topic.header)
	docs = addParagraph(docs, m, str, lipgloss.Left, true)
	docs = append(docs, base.Width(m.width).Render())
	for i := range topic.docs {
		docs = append(docs, topic.docs[i])
	}
	return docs
}

func addParagraph(docs []string, m *model, text string, position lipgloss.Position, keep bool) []string {
	if !keep && len(m.helpFilter) > 0 {
		if !strings.Contains(
			strings.ToUpper(text),
			strings.ToUpper(m.helpFilter),
		) {
			return docs
		}
	}
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
	base := &m.theme.baseStyle
	highlight := base.Bold(true).Foreground(m.theme.accentColor3)
	str := highlight.Render(text)
	docs = addParagraph(docs, m, str, lipgloss.Left, true)
	return docs
}

func makeDocs(docs []string, m *model, prefix string, text string) []string {
	base := &m.theme.baseStyle
	highlight := base.Bold(true).Foreground(m.theme.accentColor5)
	str := highlight.Render(prefix) + base.Render(text)
	docs = addParagraph(docs, m, str, lipgloss.Left, false)
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
		true,
	)

	nd := helpTopic{header: " Normal Mode"}
	nd.docs = makeDocs(nd.docs, m, " q", " - Quit, returning the current directory.")
	nd.docs = makeDocs(nd.docs, m, " Q", " - Quit without returning anything.")
	nd.docs = makeDocs(nd.docs, m, " g", " - Enter Go mode.")
	nd.docs = makeDocs(nd.docs, m, " t", " - Duplicate the current tab.")
	nd.docs = makeDocs(nd.docs, m, " ]", " - Next tab.")
	nd.docs = makeDocs(nd.docs, m, " [", " - Previous tab.")
	nd.docs = makeDocs(nd.docs, m, " 1-0", " - Select tabs 1 to 10 (0 is tab 10).")
	nd.docs = makeDocs(nd.docs, m, " space", " - Select.")
	nd.docs = makeDocs(nd.docs, m, " Ctrl+a", " - Select all.")
	nd.docs = makeDocs(nd.docs, m, " Ctrl+d", " - Deselect all.")
	nd.docs = makeDocs(nd.docs, m, " Ctrl+r", " - Toggle selection (invert all).")
	nd.docs = makeDocs(nd.docs, m, " Ctrl+w", " - Close the current tab.")
	nd.docs = makeDocs(nd.docs, m, " T", " - Restore the last closed tab.")
	nd.docs = makeDocs(nd.docs, m, " d", " - Delete the selected items PERMANENTLY.")
	nd.docs = makeDocs(nd.docs, m, " r", " - Rename the selected items.")
	nd.docs = makeDocs(nd.docs, m, " y", " - Copy the selected items.")
	nd.docs = makeDocs(nd.docs, m, " x", " - Cut the selected items.")
	nd.docs = makeDocs(nd.docs, m, " p", " - Paste.")
	nd.docs = makeDocs(nd.docs, m, " u", " - Undo.")
	nd.docs = makeDocs(nd.docs, m, " U", " - Redo.")
	nd.docs = makeDocs(nd.docs, m, " j, down", " - Move the cursor down.")
	nd.docs = makeDocs(nd.docs, m, " k, up", " - Move the cursor up.")
	nd.docs = makeDocs(nd.docs, m, " l, right", " - Enter the selected directory.")
	nd.docs = makeDocs(nd.docs, m, " h, left", " - Enter the parent directory.")
	nd.docs = makeDocs(nd.docs, m, " tab", " - Enter Jump mode. Jump mode allows jumping to items using their first letter as a shortcut.")
	nd.docs = makeDocs(nd.docs, m, " v", " - Enter Visual mode.")
	nd.docs = makeDocs(nd.docs, m, " f", " - Enter Filter mode. Filter mode filters the items in the current tab.")
	nd.docs = makeDocs(nd.docs, m, " ,", " - Enter Sort mode.")
	nd.docs = makeDocs(nd.docs, m, " a", " - Enter Create mode. You can create files and directories here.")
	nd.docs = makeDocs(nd.docs, m, " `", " - Enter Message mode. The message history can be viewed here.")
	nd.docs = makeDocs(nd.docs, m, " F5", " - Refresh the current tab.")

	pd := helpTopic{header: " Path Mode"}
	pd.docs = makeDocs(pd.docs, m, "", " Path mode allows changing the directory by typing. You can enter this mode from Normal mode by typing gg (press g twice).")
	pd.docs = makeDocs(pd.docs, m, " Ctrl+a", " - Clear everything. An empty string is also a valid path; it lists the available drives (C:\\, D:\\, etc.).")
	pd.docs = makeDocs(pd.docs, m, " Ctrl+w", " - Delete the last word.")
	pd.docs = makeDocs(pd.docs, m, " Ctrl+e", " - Expand environment variables.")
	pd.docs = makeDocs(pd.docs, m, " Ctrl+n", " - Open the directory in a new tab.")
	pd.docs = makeDocs(pd.docs, m, " tab", " - Autocomplete.")
	pd.docs = makeDocs(pd.docs, m, " down/up", " - Next/Previous autocomplete.")

	docs = addTopic(docs, &nd, m)
	docs = addTopic(docs, &pd, m)
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

	switch m.mode {
	case helpMode:
		help := base.Foreground(m.theme.grayColor).Render("Press ")
		help += base.Foreground(m.theme.accentColor2).Render("F")
		help += base.Foreground(m.theme.grayColor).Render(" to filter")
		if len(m.helpFilter) > 0 {
			help += base.Foreground(m.theme.grayColor).Render(" (filter:")
			help += base.Render(m.helpFilter)
			help += base.Foreground(m.theme.grayColor).Render(")")
		}
		help = ansi.Truncate(help, m.width, "…")
		s.WriteString(base.Width(m.width).Render(help))
	case helpFilterMode:
		widget := m.input.View()
		text := empty.Width(m.width).Render(widget)
		s.WriteString(text)
	}

	return s.String()
}
