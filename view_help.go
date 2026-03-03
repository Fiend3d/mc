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

func makeDocs(docs []string, m *model, prefix string, text string) []string {
	base := &m.theme.baseStyle
	highlight := base.Bold(true).Foreground(m.theme.accentColor5)
	str := highlight.Render(prefix) + base.Render(text)
	docs = addParagraph(docs, m, str, lipgloss.Left, false)
	return docs
}

// TODO: optimize this madness
// or maybe it's okay and nobody cares
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

	normalDocs := helpTopic{header: " Normal Mode"}
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" q", " - Quit, returning the current directory.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" Q", " - Quit without returning anything.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" g", " - Enter Go mode.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" t", " - Duplicate the current tab.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" ]", " - Next tab.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" [", " - Previous tab.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" 1-0", " - Select tabs 1 to 10 (0 is tab 10).")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" space", " - Select.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" Ctrl+a", " - Select all.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" Ctrl+d", " - Deselect all.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" Ctrl+r", " - Toggle selection (invert all).")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" Ctrl+w", " - Close the current tab.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" T", " - Restore the last closed tab.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" d", " - Delete the selected items PERMANENTLY.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" r", " - Rename the selected items.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" y", " - Copy the selected items.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" x", " - Cut the selected items.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" p", " - Paste.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" u", " - Undo.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" U", " - Redo.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" j, down", " - Move the cursor down.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" k, up", " - Move the cursor up.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" l, right", " - Enter the selected directory.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" h, left", " - Enter the parent directory.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" Ctrl+b", " - Go back in history.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" Ctrl+f", " - Go forward in history.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" tab", " - Enter Jump mode. Jump mode allows jumping to items using their first letter as a shortcut.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" v", " - Enter Visual mode.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" f", " - Enter Filter mode. Filter mode filters the items in the current tab.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" c", " - Enter Copy mode to copy paths and names of selected items to the clipboard.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" B", " - Bookmark the directory.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" b", " - Browse bookmarks.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" esc", " - Exit Temp mode of the filtered items.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" ,", " - Enter Sort mode.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" a", " - Enter Create mode. You can create files and directories here.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" `", " - Enter Message mode. The message history can be viewed here.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" F3", " - Viewer tool (bat with less by default, configurable).")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" F4", " - Editor (Helix by default, configurable).")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" F5", " - Refresh current tab.")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" F6", " - File explorer (configurable).")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" F7", " - VS Code paths (configurable).")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" F8", " - VS Code directory (configurable).")
	normalDocs.docs = makeDocs(normalDocs.docs, m,
		" F9-F12", " - Unassigned (configurable).")

	goDocs := helpTopic{header: " Go Mode"}
	goDocs.docs = makeDocs(goDocs.docs, m,
		"", " Go mode is just a menu.")
	goDocs.docs = makeDocs(goDocs.docs, m,
		" g", " - Enter Path mode.")
	goDocs.docs = makeDocs(goDocs.docs, m,
		" t", " - Browse tabs.")
	goDocs.docs = makeDocs(goDocs.docs, m,
		" c", " - Open the settings directory. You can also find and delete bookmarks there, for example.")
	goDocs.docs = makeDocs(goDocs.docs, m,
		" C", " - Save settings to config.toml for editing.")

	pathDocs := helpTopic{header: " Path Mode"}
	pathDocs.docs = makeDocs(pathDocs.docs, m,
		"", " Path mode allows changing the directory by typing. You can enter this mode from Normal mode by typing gg (press g twice).")
	pathDocs.docs = makeDocs(pathDocs.docs, m,
		" Ctrl+a", " - Clear everything. An empty string is also a valid path; it lists the available drives (C:\\, D:\\, etc.).")
	pathDocs.docs = makeDocs(pathDocs.docs, m,
		" Ctrl+w", " - Delete the last word.")
	pathDocs.docs = makeDocs(pathDocs.docs, m,
		" Ctrl+e", " - Expand environment variables.")
	pathDocs.docs = makeDocs(pathDocs.docs, m,
		" Ctrl+n", " - Open the directory in a new tab.")
	pathDocs.docs = makeDocs(pathDocs.docs, m,
		" tab", " - Autocomplete.")
	pathDocs.docs = makeDocs(pathDocs.docs, m,
		" down/up", " - Next/Previous autocomplete.")

	docs = addTopic(docs, &normalDocs, m)
	docs = addTopic(docs, &goDocs, m)
	docs = addTopic(docs, &pathDocs, m)
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
		help = truncate(help, m.width)
		s.WriteString(base.Width(m.width).Render(help))
	case helpFilterMode:
		widget := m.input.View()
		text := empty.Width(m.width).Render(widget)
		s.WriteString(text)
	}

	return s.String()
}
