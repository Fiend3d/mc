package main

import (
	"fmt"
	"strings"

	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type helpTopic struct {
	header string
	docs   []string
}

func newHelpTopic(header string, data [][]string, m *model) helpTopic {
	result := helpTopic{header: header}
	for i := range data {
		result.docs = makeDocs(result.docs, m, data[i][0], data[i][1])
	}
	return result
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

	normalDocsData := [][]string{
		{" q", " - Quit, returning the current directory."},
		{" Q", " - Quit without returning anything."},
		{" g", " - Enter Go mode."},
		{" Ctrl+h", " - Hide/Unhide TUI."},
		{" t", " - Duplicate the current tab."},
		{" ]", " - Next tab."},
		{" [", " - Previous tab."},
		{" 1-0", " - Select tabs 1 to 10 (0 is tab 10)."},
		{" space", " - Select."},
		{" Ctrl+a", " - Select all."},
		{" Ctrl+d", " - Deselect all."},
		{" Ctrl+r", " - Toggle selection (invert all)."},
		{" Ctrl+w", " - Close the current tab."},
		{" T", " - Restore the last closed tab."},
		{" d", " - Delete the selected items PERMANENTLY."},
		{" r", " - Rename the selected items."},
		{" y", " - Copy the selected items."},
		{" x", " - Cut the selected items."},
		{" p", " - Paste."},
		{" u", " - Undo."},
		{" U", " - Redo."},
		{" j, down", " - Move the cursor down."},
		{" k, up", " - Move the cursor up."},
		{" l, right", " - Enter the selected directory."},
		{" h, left", " - Enter the parent directory."},
		{" Ctrl+b", " - Go back in history."},
		{" Ctrl+f", " - Go forward in history."},
		{" tab", " - Enter Jump mode. Jump mode allows jumping to items using their first letter as a shortcut."},
		{" v", " - Enter Visual mode."},
		{" f", " - Enter Filter mode. Filter mode filters the items in the current tab."},
		{" c", " - Enter Copy mode to copy paths and names of selected items to the clipboard."},
		{" B", " - Bookmark the directory."},
		{" b", " - Browse bookmarks."},
		{" esc", " - Exit Temp mode of the filtered items."},
		{" ,", " - Enter Sort mode."},
		{" a", " - Enter Create mode. You can create files and directories here."},
		{" `", " - Enter Message mode. The message history can be viewed here."},
		{" s", " - Enter Search mode."},
		{" :", " - Enter Shell mode."},
		{" F3", " - Viewer tool (bat with less by default, configurable)."},
		{" F4", " - Editor (Helix by default, configurable)."},
		{" F5", " - Refresh current tab."},
		{" F6", " - File explorer (configurable)."},
		{" F7", " - VS Code paths (configurable)."},
		{" F8", " - VS Code directory (configurable)."},
		{" F9-F12", " - Unassigned (configurable)."},
	}
	normalDocs := newHelpTopic(" Normal Mode", normalDocsData, m)

	goDocsData := [][]string{
		{"", " Go mode is just a menu."},
		{" g", " - Enter Path mode."},
		{" t", " - Browse tabs."},
		{" c", " - Open the settings directory. You can also find and delete bookmarks there, for example."},
		{" C", " - Save settings to config.toml for editing."},
		{" s", " - Calculate size for the selected directories."},
	}
	goDocs := newHelpTopic(" Go Mode", goDocsData, m)

	pathDocsData := [][]string{
		{"", " Path mode allows changing the directory by typing." +
			" You can enter this mode from Normal mode by typing gg (press g twice)." +
			" An empty string is also a valid path; it lists the available drives (C:\\, D:\\, etc.)."},
		{" Ctrl+u", " - Clear everything left of cursor."},
		{" Ctrl+k", " - Clear everything right of cursor."},
		{" Ctrl+w", " - Delete the last word."},
		{" Ctrl+e", " - Expand environment variables."},
		{" Ctrl+n", " - Open the directory in a new tab."},
		{" tab", " - Autocomplete."},
		{" down/up", " - Next/Previous autocomplete."},
	}
	pathDocs := newHelpTopic(" Path Mode", pathDocsData, m)

	searchDocsData := [][]string{
		{" F1", " - Toggle .gitignore filtering."},
		{" F5", " - Start or restart search."},
		{" F3", " - Open selected line with 'less' or F3 command from config."},
		{" Esc", " - Exit Search mode or cancel searching."},
		{" h", " - Hide or show lines."},
		{" tab", " - Cycle focus (filename -> text -> results)."},
		{" enter", " - Focus on filename or text: start search; focus on results: show file/directory in Normal mode."},
	}
	searchDocs := newHelpTopic(" Search Mode", searchDocsData, m)

	shellDocsData := [][]string{
		{"", " Reminder: UI's visibility can be toggled by pressing Ctrl+h in Normal mode."},
		{" #sl", " - Macro that replaces #sl with selected files/directories in Shell mode."},
		{" #dir", " - Macro that replaces #dir with current tab directory in Shell mode."},
		{" Ctrl+b", " - Go back in shell history."},
		{" Ctrl+f", " - Go forward in shell history."},
	}
	shellDocs := newHelpTopic(" Shell Mode", shellDocsData, m)

	docs = addTopic(docs, &normalDocs, m)
	docs = addTopic(docs, &goDocs, m)
	docs = addTopic(docs, &pathDocs, m)
	docs = addTopic(docs, &searchDocs, m)
	docs = addTopic(docs, &shellDocs, m)

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
