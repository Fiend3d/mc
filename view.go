package main

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/x/ansi"
	overlay "github.com/rmhubbert/bubbletea-overlay"
)

func (m model) View() string {
	if m.err != nil {
		msg := fmt.Sprintf("Error: %s", m.err)
		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			msg,
		)
	}

	switch m.mode {
	case messagesMode:
		return viewMessages(&m)
	case tabsMode:
		return viewTabs(&m)
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
				Foreground(m.theme.accentColor5).
				Bold(true).
				Render(fmt.Sprintf(" [%d/%d] ", m.currentTab+1, len(m.tabs)))
			tabsWidth = lipgloss.Width(tabsWidget)
		}
		dir := ansi.Truncate(m.getTab().dir, m.width-tabsWidth, "…")
		s.WriteString(empty.Width(m.width - tabsWidth).Bold(true).Render(dir))
		if tabsWidth > 0 {
			s.WriteString(tabsWidget)
		}
		s.WriteRune('\n')
	} else {
		widget := m.pathInput.View()
		widget = ansi.Truncate(widget, m.width, "…")
		s.WriteString(empty.Width(m.width).Render(widget))
		s.WriteRune('\n')
	}

	const (
		cursorWidth = 3
		sizeWidth   = 8
		timeWidth   = 16
		colGap      = 1
	)

	countItems := 0

	for i := range page.items {
		if i+1 > m.height-3 || i+settings.start >= len(page.items) {
			break
		}

		style := base

		index := i + settings.start
		current := index == settings.cursor
		cursor := " "

		switch m.mode {
		case visualMode, confirmDialogVisualMode:
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

		item := page.items[i+settings.start] // it might crash here

		switch item.action {
		case itemActionCopy:
			s.WriteString(m.theme.copiedStyle.Render(" "))
		case itemActionCut:
			s.WriteString(m.theme.cutStyle.Render(" "))
		default:
			s.WriteString(style.Render(" "))
		}

		if m.mode == visualMode && current {
			s.WriteString(style.Bold(true).Foreground(m.theme.accentColor3).Render(cursor))
		} else {
			s.WriteString(style.Bold(true).Foreground(m.theme.whiteColor).Render(cursor))
		}
		if item.selected {
			s.WriteString(m.theme.selectedStyle.Render(" "))
		} else {
			s.WriteString(style.Render(" "))
		}

		// name block
		var nameBlock strings.Builder

		nameWidth := max(
			m.width-cursorWidth-sizeWidth-timeWidth-colGap*2+1, 1)

		if item.isDir {
			nameBlock.WriteString(
				style.Foreground(m.theme.accentColor4).Render(item.name),
			)
			nameBlock.WriteString(style.Bold(true).Render("/"))
		} else {
			if strings.HasSuffix(strings.ToLower(item.name), ".exe") {
				nameBlock.WriteString(
					style.Foreground(m.theme.greenColor).Render(item.name),
				)
			} else {
				nameBlock.WriteString(
					style.Foreground(m.theme.whiteColor).Render(item.name),
				)
			}
		}

		if item.isSymlink {
			nameBlock.WriteString(
				style.Foreground(m.theme.accentColor2).Render(" -> "))
			nameBlock.WriteString(
				style.Foreground(m.theme.accentColor3).Render(item.symlink))
		}

		name := nameBlock.String()

		name = ansi.Truncate(name, nameWidth, "…")

		s.WriteString(name)
		nameLen := lipgloss.Width(name)
		if nameLen < nameWidth {
			s.WriteString(style.Width(nameWidth - nameLen).Render(" "))
		}

		// time column
		timeStyle := style.Foreground(m.theme.grayColor)
		s.WriteString(timeStyle.Render(item.modTime))

		s.WriteString(style.Render(" "))

		// size column
		s.WriteString(style.Render(
			lipgloss.PlaceHorizontal(sizeWidth, lipgloss.Center, item.size)))
		s.WriteString(style.Render(item.size))

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
	case createMode:
		modeStyle = modeStyle.Background(m.theme.accentColor3)
		modeStr = " CREATE "
	case pathMode:
		modeStyle = modeStyle.Background(m.theme.grayColor)
		modeStr = " PATH "
	default:
		modeStyle = modeStyle.Background(m.theme.whiteColor)
		modeStr = " NONE "
	}

	modeBlock := modeStyle.Render(modeStr)
	modeWidth := lipgloss.Width(modeBlock) + 2
	if m.jobs > 0 {
		modeBlock += m.spinner.View()
	} else {
		modeBlock += base.Render("  ")
	}

	var itemName string
	var modeBitsStr string
	if settings.cursor < len(page.items) {
		selected_item := page.items[settings.cursor]
		itemName = selected_item.name
		modeBitsStr = selected_item.mode
	}

	modeBitsBlock := base.Foreground(m.theme.grayColor).Render(modeBitsStr)

	rightStr := fmt.Sprintf(" [%d/%d] ", settings.cursor+1, len(page.items))
	rightBlock := m.theme.baseStyle.Render(rightStr)
	rightBlock = modeBitsBlock + rightBlock
	rightWidth := lipgloss.Width(rightBlock)

	nameWidth := max(1, m.width-modeWidth-rightWidth)
	itemName = ansi.Truncate(itemName, nameWidth, "…")

	nameBlock := base.
		Width(nameWidth).
		Render(itemName)

	statusBar := modeBlock + nameBlock + rightBlock

	s.WriteString(statusBar)
	s.WriteRune('\n')

	switch m.mode {
	case filterMode, createMode:
		widget := m.input.View()
		text := empty.Width(m.width).Render(widget)
		s.WriteString(text)

	default:
		if m.ticks > 0 {
			logMsg := m.log[len(m.log)-1].render(&m.theme, false)
			if lipgloss.Width(logMsg) > m.width {
				logMsg = ansi.Truncate(logMsg, m.width, "…")
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
		}

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

		ui = overlay.Composite(t.Render(), ui, overlay.Center, overlay.Center, 0, 0)

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

	return ui
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
			logMsg := m.log[last].render(&m.theme, true)
			if lipgloss.Width(logMsg) > m.width-numbersLength {
				logMsg = ansi.Truncate(logMsg, m.width-numbersLength, "…")
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
