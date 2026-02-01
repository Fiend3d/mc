package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/truncate"
)

func (m model) Init() tea.Cmd {

	return m.readDir(m.tabs[0].dir)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case errorMsg:
		m.err = msg.err
		return m, nil

	case readDirMsg:
		page := m.pages[msg.dir]
		page.items = nil
		for i := range msg.entries {
			item, err := newItem(msg.entries[i], page.dir)
			if err != nil {
				return m, newErr(err)
			}
			page.items = append(page.items, item)
		}
		m.pages[msg.dir] = page
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			if m.mode == normal {
				return m, tea.Quit
			}
		// case "d":
		// 	return m, newErr(errors.New("EPIC FAIL"))
		case "j", "down":
			page := m.getPage()
			page.moveCursor(1, m.height)
			return m, nil
		case "k", "up":
			page := m.getPage()
			page.moveCursor(-1, m.height)
			return m, nil
		case "pgdown":
			page := m.getPage()
			page.moveCursor(3, m.height)
			return m, nil
		case "pgup":
			page := m.getPage()
			page.moveCursor(-3, m.height)
			return m, nil
		case "home":
			page := m.getPage()
			page.cursor = 0
			page.updateStart(m.height)
			return m, nil
		case "end":
			page := m.getPage()
			page.cursor = page.length() - 1
			page.updateStart(m.height)
			return m, nil
		case "h", "left":
			tab := m.tabs[m.currentTab]
			parent := filepath.Dir(tab.dir)
			tab.dir = parent
			_, exists := m.pages[parent] // not gonna update anything
			if exists {
				return m, nil
			}
			m.pages[parent] = &page{dir: parent}
			return m, m.readDir(parent)
		}
	}

	return m, nil
}

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

	var s strings.Builder

	base := &m.theme.baseStyle
	empty := &m.theme.emptyStyle

	page := m.getPage()

	// Header (directory)
	dirLine := lipgloss.PlaceHorizontal(m.width, lipgloss.Left, page.dir)
	s.WriteString(empty.Bold(true).Render(dirLine))
	s.WriteRune('\n')

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
		if i+page.start == page.cursor {
			style = &m.theme.cursorStyle
			s.WriteString(
				style.
					Bold(true).
					Foreground(m.theme.whiteColor).
					Render(" > "),
			)
		} else {
			s.WriteString(style.Render("   "))
		}

		item := page.items[i+page.start]

		// --- name block ---
		var nameBlock strings.Builder

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
			nameBlock.WriteString(style.Render(" -> "))
			nameBlock.WriteString(style.Render(item.symlink))
		}

		nameWidth := max(
			m.width-cursorWidth-sizeWidth-timeWidth-colGap*2+1, 1)

		name := nameBlock.String()

		if lipgloss.Width(name) > nameWidth {
			name = truncate.StringWithTail(
				name,
				uint(nameWidth),
				"…",
			)
		}

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
	padded := base.Padding(0, 1)
	mode_style := padded.
		Background(m.theme.accentColor5).
		Foreground(m.theme.blackColor).
		Bold(true)

	switch m.mode {
	case normal:
		modeStr = "NORMAL"
	case visual:
		mode_style = mode_style.Foreground(m.theme.accentColor4)
		modeStr = "VISUAL"
	default:
		mode_style = mode_style.Foreground(m.theme.whiteColor)
		modeStr = "NONE"
	}

	modeBlock := mode_style.Render(modeStr)
	modeWidth := lipgloss.Width(modeBlock)

	var itemName string
	var modeBitsStr string
	if page.cursor < len(page.items) {
		selected_item := page.items[page.cursor]
		itemName = selected_item.name
		modeBitsStr = selected_item.mode
	}

	modeBitsBlock := base.Foreground(m.theme.grayColor).Render(modeBitsStr)

	rightStr := fmt.Sprintf("[%d/%d]", page.cursor+1, len(page.items))
	rightBlock := padded.Render(rightStr)
	rightBlock = lipgloss.JoinHorizontal(lipgloss.Center, modeBitsBlock, rightBlock)
	rightWidth := lipgloss.Width(rightBlock)

	nameWidth := max(1, m.width-modeWidth-rightWidth)
	itemName = truncate.StringWithTail(
		itemName,
		uint(nameWidth-2),
		"…",
	)

	nameBlock := padded.
		Width(nameWidth).
		Render(itemName)

	statusBar := lipgloss.JoinHorizontal(
		lipgloss.Left,
		modeBlock,
		nameBlock,
		rightBlock,
	)

	statusBar = lipgloss.NewStyle().
		Width(m.width).
		MaxWidth(m.width).
		MaxHeight(1).
		Render(statusBar)

	s.WriteString(statusBar)
	s.WriteRune('\n')

	messageBar := empty.Width(m.width).Render()
	s.WriteString(messageBar)

	return s.String()
}

func main() {
	flag.Parse()
	dirs := flag.Args()
	if len(dirs) == 0 {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatalf("error: %s", err)
		}
		dirs = []string{wd}
	}

	p := tea.NewProgram(initialModel(dirs), tea.WithAltScreen())
	_, err := p.Run()
	if err != nil {
		log.Fatalf("failed to launch the program: %s", err)
	}
}
