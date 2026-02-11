package main

import (
	"fmt"
	"slices"
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

	if m.mode == messages {
		return viewMessages(&m)
	}

	base := &m.theme.baseStyle
	empty := &m.theme.emptyStyle
	var s strings.Builder

	page := m.getPage()

	// Header (directory)
	if m.mode != path {
		s.WriteString(empty.Width(m.width).Bold(true).Render(page.dir))
		s.WriteRune('\n')
	} else {
		widget := m.pathInput.View()
		s.WriteString(empty.Width(m.width).Render(widget))
		s.WriteRune('\n')
	}

	const (
		cursorWidth = 3
		sizeWidth   = 8
		timeWidth   = 16
		colGap      = 1
	)

	for i := range page.items {
		if i+1 > m.height-3 {
			break
		}

		style := base

		index := i + page.start
		current := index == page.cursor
		cursor := " "
		if m.mode == visual {
			start, end := page.getStartEnd()
			if index >= start && index <= end {
				style = &m.theme.cursorStyle
				switch index {
				case start, end:
					cursor = "="
				default:
					cursor = "|"
				}
			}
		} else {
			if current {
				style = &m.theme.cursorStyle
				cursor = ">"
			}
		}

		item := page.items[i+page.start]
		switch m.action {
		case noAction:
			s.WriteString(style.Render(" "))
		case copy:
			if slices.Contains(m.actionPaths, item.fullPath) {
				s.WriteString(m.theme.copiedStyle.Render(" "))
			} else {
				s.WriteString(style.Render(" "))
			}
		case cut:
			if slices.Contains(m.actionPaths, item.fullPath) {
				s.WriteString(m.theme.cutStyle.Render(" "))
			} else {
				s.WriteString(style.Render(" "))
			}
		}

		if m.mode == visual && current {
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
			nameBlock.WriteString(
				style.Foreground(m.theme.whiteColor).Render(item.name),
			)
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
	}

	// render empty lines
	for i := len(page.items); i < m.height-3; i++ {
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
	case normal:
		modeStr = " NORMAL "
	case visual:
		modeStyle = modeStyle.Background(m.theme.accentColor4)
		modeStr = " VISUAL "
	case jump:
		modeStyle = modeStyle.Background(m.theme.accentColor1)
		modeStr = " JUMP "
	case filter:
		modeStyle = modeStyle.Background(m.theme.accentColor2)
		modeStr = " FILTER "
	case path:
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

	switch m.action {
	case noAction:
		var itemName string
		var modeBitsStr string
		if page.cursor < len(page.items) {
			selected_item := page.items[page.cursor]
			itemName = selected_item.name
			modeBitsStr = selected_item.mode
		}

		modeBitsBlock := base.Foreground(m.theme.grayColor).Render(modeBitsStr)

		rightStr := fmt.Sprintf(" [%d/%d] ", page.cursor+1, len(page.items))
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

	case copy:
		s.WriteString(modeBlock)
		statusMsg := fmt.Sprintf("%d paths copied", len(m.actionPaths))
		s.WriteString(base.Width(m.width - modeWidth).Foreground(m.theme.accentColor3).Render(statusMsg))
		s.WriteRune('\n')

	case cut:
		s.WriteString(modeBlock)
		statusMsg := fmt.Sprintf("%d paths cut", len(m.actionPaths))
		s.WriteString(base.Width(m.width - modeWidth).Foreground(m.theme.accentColor1).Render(statusMsg))
		s.WriteRune('\n')
	}

	switch m.mode {
	case filter:
		widget := m.filterInput.View()
		text := empty.Width(m.width).Render(widget)
		s.WriteString(text)

	default:
		if m.ticks > 0 {
			s.WriteString(empty.Width(m.width).Render(m.log[len(m.log)-1].render(&m.theme, false)))
		} else {
			s.WriteString(empty.Width(m.width).Render())
		}
	}

	ui := s.String()

	switch m.submode {
	case goMode:
		headers := []string{"Button", "Description"}
		rows := [][]string{
			{"g", "Go to path"},
			{"b", "Go to bookmarks"},
			{"r", "Reset action"},
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
			s.WriteString(
				empty.Width(m.width - numbersLength).Render(
					m.log[last].render(&m.theme, true)))
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
