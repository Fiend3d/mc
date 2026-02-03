package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/muesli/reflow/truncate"
)

func (m *model) left() (tea.Model, tea.Cmd) {
	tab := m.getTab()
	parent := filepath.Dir(tab.dir)
	tab.dir = parent
	_, exists := tab.pages[parent] // not gonna update anything
	if exists {
		return m, nil
	}
	tab.pages[parent] = &page{dir: parent}
	return m, m.readDir(parent)
}
func (m *model) right() (tea.Model, tea.Cmd) {
	tab := m.getTab()
	currentPage := tab.getPage()
	if currentPage.cursor > len(currentPage.items)-1 {
		return m, nil
	}
	selectedItem := currentPage.items[currentPage.cursor]
	if !selectedItem.isDir {
		return m, nil
	}
	dir := filepath.Join(tab.dir, selectedItem.name)
	tab.dir = dir
	_, exists := tab.pages[dir] // not gonna update
	if exists {
		return m, nil
	}
	tab.pages[dir] = &page{dir: dir}
	return m, m.readDir(dir)
}

func (m model) Init() tea.Cmd {

	return m.readDir(m.tabs[0].dir)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case errorMsg:
		m.err = msg.err
		return m, nil

	case readDirMsg:
		tab := m.getTab()
		page := tab.pages[msg.dir]
		page.items = nil
		for i := range msg.entries {
			item, err := newItem(msg.entries[i], page.dir)
			if err != nil {
				return m, newErr(err)
			}
			page.items = append(page.items, item)
		}
		tab.pages[msg.dir] = page
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch m.mode {
		case normal:
			switch msg.String() {
			case "q":
				if m.mode == normal {
					m.result = m.getTab().dir
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
				return m.left()
			case "l", "right":
				return m.right()
			case "tab":
				m.mode = dash
				return m, nil
			}
		case dash:
			switch msg.String() {
			case "esc", "tab":
				m.mode = normal
				return m, nil
			case "left":
				return m.left()
			case "right":
				return m.right()
			default:
				if len(msg.Runes) > 0 { // just in case, I dunno
					r := msg.Runes[0]
					page := m.getPage()
					var matches []int
					for i := range page.items {
						runes := []rune(page.items[i].name)
						if len(runes) == 0 {
							continue
						}
						if runes[0] == r {
							matches = append(matches, i)
						}
					}

					if len(matches) > 0 {
						if slices.Contains(matches, page.cursor) {
							index := slices.Index(matches, page.cursor)
							next := (index + 1) % len(matches)
							page.cursor = matches[next]
						} else {
							page.cursor = matches[0]
						}
					}
				}
				return m, nil
			}
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
			nameBlock.WriteString(
				style.Foreground(m.theme.accentColor2).Render(" -> "))
			nameBlock.WriteString(
				style.Foreground(m.theme.accentColor3).Render(item.symlink))
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
		mode_style = mode_style.Background(m.theme.accentColor4)
		modeStr = "VISUAL"
	case dash:
		mode_style = mode_style.Background(m.theme.accentColor1)
		modeStr = "DASH"
	default:
		mode_style = mode_style.Background(m.theme.whiteColor)
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
	tempFileFlag := flag.String("tf", "output.tmp", "temp file for output")
	outputFlag := flag.Bool("o", false, "enable temp file output")
	flag.Parse()
	dirs := flag.Args()
	tempFile := *tempFileFlag
	output := *outputFlag

	if len(dirs) == 0 {
		wd, err := os.Getwd()
		if err != nil {
			log.Fatalf("error: %s", err)
		}
		dirs = []string{wd}
	}

	p := tea.NewProgram(initialModel(dirs), tea.WithAltScreen())
	m, err := p.Run()
	if err != nil {
		log.Fatalf("failed to launch the program: %s", err)
	}

	finalModel := m.(model)

	if output {
		err := os.WriteFile(tempFile, []byte(finalModel.result), 0644)
		if err != nil {
			log.Fatalf("error: %s\n", err)
		}
	} else {
		fmt.Println(finalModel.result)
	}
}
