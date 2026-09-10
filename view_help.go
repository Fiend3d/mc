package main

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/x/ansi"
)

// helpEntry is one shortcut and what it does, kept unstyled so the key column
// can be sized across every topic before anything is rendered.
type helpEntry struct {
	key  string
	text string
}

type helpTopic struct {
	header string
	// blurb is prose describing the mode, taken from a leading entry that has
	// no shortcut. It sits under the header instead of in the key column.
	blurb      string
	blurbMatch bool
	entries    []helpEntry
}

const (
	helpMargin   = " "   // left margin shared by headers, descriptions and the footer
	helpIndent   = "   " // leading space before a shortcut
	helpGap      = "  "  // between the key column and the description
	helpKeyMin  = 6
	helpKeyMax  = 22
	helpTextMin = 20
)

func newHelpTopic(header string, data [][]string, m *model) helpTopic {
	topic := helpTopic{header: strings.TrimSpace(header)}
	for i := range data {
		key := strings.TrimSpace(data[i][0])
		text := strings.TrimSpace(data[i][1])
		if i == 0 && key == "" {
			// A leading shortcut-less entry describes the mode itself. Later
			// ones elaborate on the entry above them and stay in the list.
			topic.blurb, topic.blurbMatch = text, m.helpMatches(key, text)
			continue
		}
		if !m.helpMatches(key, text) {
			continue
		}
		topic.entries = append(topic.entries, helpEntry{key, text})
	}
	return topic
}

// helpMatches reports whether an entry survives the current filter. Key and
// description are tested separately so a filter can name either one.
func (m *model) helpMatches(key, text string) bool {
	if len(m.helpFilter) == 0 {
		return true
	}
	needle := strings.ToUpper(m.helpFilter)
	return strings.Contains(strings.ToUpper(key), needle) ||
		strings.Contains(strings.ToUpper(text), needle)
}

// helpKeyColumn is the width of the shortcut column, sized to the widest
// shortcut still visible so every description starts at the same place.
func helpKeyColumn(topics []*helpTopic) int {
	width := 0
	for _, topic := range topics {
		for _, e := range topic.entries {
			width = max(width, ansi.StringWidth(e.key))
		}
	}
	return min(max(width, helpKeyMin), helpKeyMax)
}

func addTopic(docs []string, topic *helpTopic, m *model, keyWidth int) []string {
	// The blurb keeps a topic alive on its own so a filter naming the mode
	// still explains what that mode is.
	if len(topic.entries) == 0 && !topic.blurbMatch {
		return docs
	}
	base := &m.theme.baseStyle
	if len(docs) > 0 {
		docs = append(docs, base.Width(m.width).Render())
	}
	header := base.Bold(true).Foreground(m.theme.accentColor3).Render(topic.header)
	docs = append(docs, base.Width(m.width).Render(helpMargin+header))
	if topic.blurb != "" {
		docs = m.addBlurb(docs, topic.blurb, len(topic.entries) > 0)
	}
	for i, e := range topic.entries {
		// A note elaborates on the entry above it, so it hangs in the
		// description column -- unless a filter left it with nothing above,
		// in which case it reads as prose instead of a stray indent.
		if e.key == "" && i == 0 {
			docs = m.addBlurb(docs, e.text, len(topic.entries) > 1)
			continue
		}
		docs = m.addEntry(docs, e, keyWidth)
	}
	return docs
}

// addBlurb lays a topic's description under its header, dimmed and spanning the
// full width rather than squeezed into the description column.
func (m *model) addBlurb(docs []string, text string, spaced bool) []string {
	base := &m.theme.baseStyle
	dim := base.Foreground(m.theme.grayColor)
	width := max(helpTextMin, m.width-len(helpMargin))
	for _, line := range strings.Split(ansi.Wordwrap(text, width, ""), "\n") {
		docs = append(docs, base.Width(m.width).Render(helpMargin+dim.Render(line)))
	}
	if spaced {
		docs = append(docs, base.Width(m.width).Render())
	}
	return docs
}

// addEntry lays out one shortcut: the key in its column, the description
// wrapped with a hanging indent so continuation lines stay in that column.
func (m *model) addEntry(docs []string, e helpEntry, keyWidth int) []string {
	base := &m.theme.baseStyle
	column := len(helpIndent) + keyWidth + len(helpGap)
	hanging := strings.Repeat(" ", column)
	lines := strings.Split(ansi.Wordwrap(e.text, max(helpTextMin, m.width-column), ""), "\n")

	head := hanging // a note with no shortcut sits in the description column
	if e.key != "" {
		key := base.Bold(true).Foreground(m.theme.accentColor5).Render(e.key)
		if ansi.StringWidth(e.key) > keyWidth {
			// A shortcut too wide for the column takes a line of its own rather
			// than shoving its description out of alignment.
			docs = append(docs, base.Width(m.width).Render(helpIndent+key))
		} else {
			pad := strings.Repeat(" ", keyWidth-ansi.StringWidth(e.key))
			head = helpIndent + key + pad + helpGap
		}
	}
	for i := range lines {
		prefix := hanging
		if i == 0 {
			prefix = head
		}
		docs = append(docs, base.Width(m.width).Render(prefix+base.Render(lines[i])))
	}
	return docs
}

// addTitle centres the product banner over the whole help area. Centring it
// within the text width alone would leave it visibly off-centre by the columns
// reserved for the scrollbar.
func addTitle(docs []string, m *model, text string) []string {
	base := &m.theme.baseStyle
	for _, line := range strings.Split(ansi.Wordwrap(text, m.width, ""), "\n") {
		lead := max(0, (m.width+m.helpChrome-ansi.StringWidth(line))/2)
		docs = append(docs, base.Width(m.width).Render(strings.Repeat(" ", lead)+line))
	}
	return docs
}

// TODO: optimize this madness
// or maybe it's okay and nobody cares
func viewHelp(m *model) string {
	base := &m.theme.baseStyle
	empty := &m.theme.emptyStyle

	docs := make([]string, 0)

	header := " " + Version
	if GitCommit != "" {
		header += fmt.Sprintf(" (%s)", GitCommit)
	}
	if BuildTime != "" {
		header += fmt.Sprintf(" [%s]", BuildTime)
	}

	docs = addTitle(docs, m, base.Foreground(m.theme.accentColor2).Bold(true).Render("Modal Commander")+base.Render(header))

	normalDocsData := [][]string{
		{"q", "Quit and return the current directory."},
		{"Q", "Quit without returning a directory."},
		{"g", "Enter Go mode."},
		{"Ctrl+h", "Hide the interface; press it again to bring it back."},
		{"t", "Duplicate the current tab."},
		{"Ctrl+n", "Open the selected directory in a new tab."},
		{"]", "Next tab."},
		{"[", "Previous tab."},
		{"1-0", "Switch to tabs 1 to 10; 0 is the tenth."},
		{"Space / Insert", "Toggle the selection and move down."},
		{"Shift+Up/Down", "Extend or shrink the selected range."},
		{"Shift+Home/End", "Select everything up to the first or last item."},
		{"Ctrl+click", "Select everything up to the clicked item. Shift+click may select terminal text instead."},
		{"Ctrl+a", "Select everything."},
		{"Ctrl+d", "Clear the selection."},
		{"Ctrl+r", "Invert the selection."},
		{"Ctrl+w", "Close the current tab. Closing the last one leaves the pane empty."},
		{"T", "Reopen the last closed tab."},
		{"d", "Delete the selected items for good; they do not go to the Recycle Bin."},
		{"r", "Rename the selected items."},
		{"y", "Copy the selected items."},
		{"x", "Cut the selected items."},
		{"p", "Paste."},
		{"P", "Paste over what is already there, asking first if names collide."},
		{"u", "Undo."},
		{"U", "Redo."},
		{"j, down", "Move the cursor down."},
		{"k, up", "Move the cursor up."},
		{"l, right", "Open the selected directory."},
		{"h, left", "Go up to the parent directory."},
		{"Ctrl+b", "Go back in history."},
		{"Ctrl+f", "Go forward in history."},
		{"Shift+Tab", "Enter Jump mode, where each item's first letter becomes a shortcut."},
		{"Tab", "Switch the focused pane."},
		{"Ctrl+Left/Right", "Move the current tab to the left or right pane and follow it."},
		{"", "An empty pane keeps its half of the screen; T, b or gg fill it again."},
		{"Shift+Left/Right", "Copy the current tab to the left or right pane, staying where you are."},
		{"Y / X", "Copy or move to the other pane, with an editable destination."},
		{"w", "Show background tasks; press c to cancel the highlighted one."},
		{"f", "Enter Filter mode, which narrows the current tab to matching items."},
		{"c", "Enter Copy mode, which copies the paths and names of the selected items to the clipboard."},
		{"B", "Bookmark the current directory."},
		{"b", "Browse bookmarks."},
		{"esc", "Clear the active filter, keeping the cursor on the same item."},
		{", (comma)", "Enter Sort mode."},
		{"` (backtick)", "Enter Message mode to read back the message history."},
		{"a", "Enter Create mode to make new files and directories."},
		{"s", "Enter Search mode."},
		{":", "Enter Shell mode."},
		{";", "Rerun the last shell command against the current selection."},
		{"F2", "Dependency walker (deps by default, configurable)."},
		{"F3", "Viewer tool (koneko by default, configurable)."},
		{"F4", "Editor (Helix by default, configurable)."},
		{"F5", "Refresh now; directories also refresh on their own."},
		{"F6", "File explorer (configurable)."},
		{"F7", "VS Code paths (configurable)."},
		{"F8", "VS Code directory (configurable)."},
		{"F9-F12", "Unassigned (configurable)."},
	}
	normalDocs := newHelpTopic("Normal Mode", normalDocsData, m)

	goDocsData := [][]string{
		{"", "Go mode is just a menu."},
		{"g", "Enter Path mode."},
		{"t", "Browse tabs."},
		{"T", "Set the theme."},
		{"c", "Open the settings directory, where bookmarks live too."},
		{"C", "Save the settings to config.toml so you can edit them."},
		{"s", "Calculate the size of the selected directories. This runs in the background; press w to watch it or cancel it."},
	}
	goDocs := newHelpTopic("Go Mode", goDocsData, m)

	pathDocsData := [][]string{
		{"", "Change directory by typing a path. Enter this mode from Normal mode with gg." +
			" An empty path is valid too: it lists the available drives (C:\\, D:\\, and so on)."},
		{"Ctrl+u", "Delete everything left of the cursor."},
		{"Ctrl+k", "Delete everything right of the cursor."},
		{"Ctrl+w", "Delete the previous word."},
		{"Ctrl+e", "Expand environment variables."},
		{"Ctrl+n", "Open the directory in a new tab."},
		{"tab", "Complete the path."},
		{"down/up", "Next or previous completion."},
	}
	pathDocs := newHelpTopic("Path Mode", pathDocsData, m)

	searchDocsData := [][]string{
		{"F1", "Toggle .gitignore filtering."},
		{"F2", "Toggle case sensitivity."},
		{"F5", "Start the search, or restart it."},
		{"F3", "Open the selected line with less, or the F3 command from your config."},
		{"Esc", "Leave Search mode, or stop a search that is running."},
		{"h", "Hide or show the matching lines."},
		{"tab", "Cycle the focus: filename, then text, then results."},
		{"enter", "In the filename or text field, start the search. In the results, open the item in Normal mode."},
	}
	searchDocs := newHelpTopic("Search Mode", searchDocsData, m)

	shellDocsData := [][]string{
		{"", "Remember that Ctrl+h in Normal mode hides the interface and brings it back."},
		{"#sl", "Stands in for the selected files and directories."},
		{"Ctrl+b", "Go back in shell history."},
		{"Ctrl+f", "Go forward in shell history."},
	}
	shellDocs := newHelpTopic("Shell Mode", shellDocsData, m)

	topics := []*helpTopic{&normalDocs, &goDocs, &pathDocs, &searchDocs, &shellDocs}
	keyWidth := helpKeyColumn(topics)
	for _, topic := range topics {
		docs = addTopic(docs, topic, m, keyWidth)
	}

	// The docs are built during rendering, so this is also where the scroll
	// offset gets its bounds and the scrollbar its content length.
	m.helpLines = len(docs)
	m.help = min(m.help, m.maxHelpScroll())

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
		help += base.Foreground(m.theme.grayColor).Render(" to filter, drag the scrollbar or scroll with the wheel")
		if len(m.helpFilter) > 0 {
			help += base.Foreground(m.theme.grayColor).Render(" (filter:")
			help += base.Render(m.helpFilter)
			help += base.Foreground(m.theme.grayColor).Render(")")
		}
		help = truncate(help, max(1, m.width-len(helpMargin)))
		s.WriteString(base.Width(m.width).Render(helpMargin + help))
	case helpFilterMode:
		widget := m.input.View()
		text := empty.Width(m.width).Render(widget)
		s.WriteString(text)
	}

	return s.String()
}
