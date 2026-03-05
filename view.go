package main

import (
	"fmt"
	"strconv"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	overlay "github.com/rmhubbert/bubbletea-overlay"
)

func (m model) View() tea.View {
	var result tea.View
	result.AltScreen = true
	result.MouseMode = tea.MouseModeCellMotion
	result.WindowTitle = "Modal Commander"

	if m.hide {
		result.AltScreen = false
		result.MouseMode = tea.MouseModeNone
		return result
	}

	if m.err != nil {
		msg := fmt.Sprintf("Error: %s", m.err)
		result.SetContent(lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			msg,
		))
		return result
	}

	switch m.mode {
	case helpMode, helpFilterMode:
		result.SetContent(viewHelp(&m))
		return result
	case messagesMode:
		result.SetContent(viewMessages(&m))
		return result
	case bookmarksMode:
		result.SetContent(viewBookmarks(&m))
		return result
	case tabsMode:
		result.SetContent(viewTabs(&m))
		return result
	case searchMode:
		result.SetContent(viewSearch(&m))
		return result
	}

	base := &m.theme.baseStyle
	empty := &m.theme.emptyStyle
	var s strings.Builder

	page := m.getPage()
	settings := m.getTab().getPageSettings()

	// Header (directory)
	if m.mode != pathMode {
		tabsWidth := 0
		tabsWidget := ""
		if len(m.tabs) > 1 {
			tabsWidget = empty.
				Foreground(m.theme.accentColor3).
				Bold(true).
				Render(fmt.Sprintf(" [%d/%d] ", m.currentTab+1, len(m.tabs)))
			tabsWidth = lipgloss.Width(tabsWidget)
		}

		dir := m.getTab().dir
		if dir == "" {
			dir = empty.Bold(true).Foreground(m.theme.accentColor3).Render(" This PC")
		} else {
			dir = colorizeDir(dir,
				empty.Bold(true).Foreground(m.theme.whiteColor),
				empty.Bold(true).Foreground(m.theme.accentColor5),
				m.width-tabsWidth)
			dir = truncate(dir, m.width-tabsWidth)
		}

		s.WriteString(empty.Width(m.width - tabsWidth).Bold(true).Render(dir))
		if tabsWidth > 0 {
			s.WriteString(tabsWidget)
		}
		s.WriteRune('\n')
	} else {
		widget := m.pathInput.View()
		widget = truncate(widget, m.width)
		s.WriteString(empty.Width(m.width).Render(widget))
		s.WriteRune('\n')
	}

	const (
		cursorWidth = 3
	)

	countItems := 0

	items := page.getItems()

	for i := range items {
		if i+1 > m.height-3 || i+settings.start >= len(items) {
			break
		}

		style := base

		index := i + settings.start
		current := index == settings.cursor
		cursor := " "

		switch m.mode {
		case visualMode, confirmDialogVisualMode, copyVisualMode:
			start, end := m.getStartEnd()
			if index >= start && index <= end {
				style = &m.theme.cursorStyle
				switch index {
				case start, end:
					cursor = "="
				default:
					cursor = "|"
				}
			}
		default:
			if current {
				style = &m.theme.cursorStyle
				cursor = ">"
			}
		}

		item := items[i+settings.start] // it might crash here

		switch item.getAction() {
		case itemActionCopy:
			s.WriteString(style.Foreground(m.theme.accentColor4).Render("┃"))
		case itemActionCut:
			s.WriteString(style.Foreground(m.theme.accentColor2).Render("┃"))
		default:
			s.WriteString(style.Render(" "))
		}

		if m.mode == visualMode && current {
			s.WriteString(style.Bold(true).Foreground(m.theme.accentColor3).Render(cursor))
		} else {
			s.WriteString(style.Bold(true).Foreground(m.theme.whiteColor).Render(cursor))
		}
		if item.isSelected() {
			s.WriteString(style.Foreground(m.theme.accentColor5).Render("┃"))
		} else {
			s.WriteString(style.Render(" "))
		}

		item.render(&s, style, m.theme, m.width-cursorWidth)
		s.WriteRune('\n')
		countItems++
	}

	if items == nil {
		s.WriteString(empty.Width(m.width).Foreground(m.theme.grayColor).Render("   Loading..."))
		s.WriteRune('\n')
		countItems++
	}

	// render empty lines
	for i := countItems; i < m.height-3; i++ {
		s.WriteString(empty.Width(m.width).Render(" "))
		s.WriteRune('\n')
	}

	// status bar
	var modeStr string
	modeStyle := base.
		Background(m.theme.accentColor5).
		Foreground(m.theme.blackColor).
		Bold(true)

	switch m.mode {
	case normalMode:
		modeStr = " NORMAL "
	case visualMode:
		modeStyle = modeStyle.Background(m.theme.accentColor4)
		modeStr = " VISUAL "
	case jumpMode:
		modeStyle = modeStyle.Background(m.theme.accentColor1)
		modeStr = " JUMP "
	case filterMode:
		modeStyle = modeStyle.Background(m.theme.accentColor2)
		modeStr = " FILTER "
	case sortMode:
		modeStyle = modeStyle.Background(m.theme.accentColor2)
		modeStr = " SORT "
	case renameMode:
		modeStyle = modeStyle.Background(m.theme.greenColor)
		modeStr = " RENAME "
	case createMode:
		modeStyle = modeStyle.Background(m.theme.accentColor3)
		modeStr = " CREATE "
	case copyMode, copyVisualMode:
		modeStyle = modeStyle.Background(m.theme.whiteColor)
		modeStr = " COPY "
	case pathMode:
		modeStyle = modeStyle.Background(m.theme.grayColor)
		modeStr = " PATH "
	case goMode:
		modeStyle = modeStyle.Background(m.theme.grayColor)
		modeStr = " GO "
	default:
		modeStyle = modeStyle.Background(m.theme.whiteColor)
		modeStr = " NONE "
	}

	modeBlock := modeStyle.Render(modeStr)
	if page.isTemp() {
		modeBlock += modeStyle.Background(m.theme.accentColor1).Render(" TEMP ")

	}
	modeWidth := lipgloss.Width(modeBlock) + 2
	if m.jobs > 0 {
		modeBlock += m.spinner.View()
	} else {
		modeBlock += base.Render("  ")
	}

	var itemName string
	var extraStr string
	if settings.cursor < len(items) {
		selected_item := items[settings.cursor]
		itemName = selected_item.getName()
		extraStr = selected_item.getExtra()
	}

	extraBlock := base.Foreground(m.theme.grayColor).Render(extraStr)

	rightStr := fmt.Sprintf(" [%d/%d] ", settings.cursor+1, len(items))
	rightBlock := m.theme.baseStyle.Render(rightStr)
	rightBlock = extraBlock + rightBlock
	rightWidth := lipgloss.Width(rightBlock)

	nameWidth := max(1, m.width-modeWidth-rightWidth)
	itemName = truncate(itemName, nameWidth)

	nameBlock := base.
		Width(nameWidth).
		Render(itemName)

	statusBar := modeBlock + nameBlock + rightBlock

	s.WriteString(statusBar)
	s.WriteRune('\n')

	switch m.mode {
	case filterMode, renameMode, createMode:
		widget := m.input.View()
		widget = truncate(widget, m.width)
		text := empty.Width(m.width).Render(widget)
		s.WriteString(text)

	default:
		if m.ticks > 0 {
			logMsg := m.log[len(m.log)-1].render(m.theme, false)
			if lipgloss.Width(logMsg) > m.width {
				logMsg = truncate(logMsg, m.width)
			}
			s.WriteString(empty.Width(m.width).Render(logMsg))
		} else {
			s.WriteString(empty.Width(m.width).Render())
		}
	}

	ui := s.String()

	switch m.mode {
	case goMode:
		headers := []string{" Button ", " Description "}
		rows := [][]string{
			{"g", " Change path "},
			{"t", " View tabs "},
			{"c", " Open config directory "},
			{"C", " Save current config "},
		}

		ui = m.renderTableOverlay(headers, rows, ui)

	case sortMode:
		headers := []string{" Button ", " Description "}
		rows := [][]string{
			{"m", " Sort by modified time "},
			{"M", " Sort by modified time (reverse) "},
			{"a", " Sort alphabetically "},
			{"A", " Sort alphabetically (reverse) "},
			{"n", " Sort normally "},
			{"N", " Sort normally (reverse) "},
			{"e", " Sort by extension "},
			{"E", " Sort by extension (reverse) "},
			{"s", " Sort by size "},
			{"S", " Sort by size (reverse) "},
			{"r", " Sort randomly "},
		}

		ui = m.renderTableOverlay(headers, rows, ui)

	case pathMode:
		headers := []string{" Hotkey ", " Description "}
		rows := [][]string{
			{" ctrl+u ", " Clear all "},
			{" ctrl+w ", " Delete a word "},
			{" tab ", " Autocomplete "},
			{" up/down ", " Next/previous autocomplete "},
			{" ctrl+e ", " Expand environment variables "},
			{" ctrl+n ", " Open the path in a new tab "},
		}

		ui = m.renderTableOverlay(headers, rows, ui)

	case copyMode, copyVisualMode:
		headers := []string{" Button ", " Description "}
		rows := [][]string{
			{" c/C ", " Copy the file path/Forward "},
			{" d/D ", " Copy the directory/Forward "},
			{" f ", " Copy the filename "},
			{" n ", " Copy the filename without extension "},
			{" a/A ", " Copy the file paths as arguments/Forward "},
			{" s ", " Copy the filenames as arguments "},
			{" q/Q ", " Copy the file paths as array/Forward "},
			{" w ", " Copy the filenames as array "},
		}

		ui = m.renderTableOverlay(headers, rows, ui)

	case confirmDialogMode, confirmDialogVisualMode:
		cell := base.
			Border(lipgloss.NormalBorder()).
			BorderBackground(m.theme.baseStyle.GetBackground()).
			BorderForeground(m.theme.grayColor)

		enable := base.Background(m.theme.greenColor).Foreground(m.theme.blackColor).Bold(true)
		disable := empty.Foreground(m.theme.grayColor).Bold(true)

		if m.yes {
			enable, disable = disable, enable
		}

		header := " Are you sure? It can't be undone. "
		yes := " Yes "
		no := " No "

		buttons := base.Render(strings.Repeat(" ",
			len(header)/2-len(yes))) + disable.Render(yes) + base.Render(" ") + enable.Render(no) + base.Render(strings.Repeat(" ", len(header)/2-len(no)))

		content := lipgloss.JoinVertical(
			lipgloss.Center,
			header,
			buttons,
		)

		window := cell.Render(content)

		ui = overlay.Composite(window, ui, overlay.Center, overlay.Center, 0, 0)
	}

	result.SetContent(ui)
	return result
}

func (m *model) renderTableOverlay(headers []string, rows [][]string, ui string) string {
	tStyle := m.theme.baseStyle

	t := table.New().
		Border(lipgloss.NormalBorder()).
		BorderStyle(tStyle.Foreground(m.theme.grayColor)).
		Headers(headers...).
		Rows(rows...).
		StyleFunc(func(row, col int) lipgloss.Style {
			if row == table.HeaderRow {
				return tStyle.Foreground(m.theme.grayColor)
			}

			if col == 0 {
				return tStyle.
					Foreground(m.theme.accentColor2).
					Align(lipgloss.Center)
			} else {
				return tStyle
			}

		})

	return overlay.Composite(t.Render(), ui, overlay.Center, overlay.Center, 0, 0)
}

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
			if lipgloss.Width(logMsg) > m.width-numbersLength {
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
		style := base.
			BorderStyle(lipgloss.NormalBorder()).
			BorderForeground(m.theme.grayColor).
			BorderBackground(base.GetBackground())

		messages = overlay.Composite(style.Render(" The log is empty! "), messages, overlay.Center, overlay.Center, 0, 0)
	}

	return messages
}
