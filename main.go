package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/lipgloss/table"
	"github.com/charmbracelet/x/ansi"
	overlay "github.com/rmhubbert/bubbletea-overlay"
)

func (m model) Init() tea.Cmd {
	return m.readDir(m.tabs[0].dir)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case errorMsg:
		m.err = msg.err
		return m, nil

	case tickMsg:
		if m.ticks > 0 {
			m.ticks--
			return m, tick()
		}

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
		if m.submode == noSubmode {
			switch m.mode {
			case normal, jump, visual:
				switch msg.Type {
				case tea.KeyCtrlA:
					page := m.getPage()
					for i := range page.items {
						page.items[i].selected = true
					}
					return m, nil
				case tea.KeyCtrlR:
					page := m.getPage()
					for i := range page.items {
						page.items[i].selected = !page.items[i].selected
					}
					return m, nil
				}
				switch msg.String() {
				case "down":
					page := m.getPage()
					page.moveCursor(1, m.height)
					return m, nil
				case "up":
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
				case " ":
					page := m.getPage()
					if m.mode == visual {
						start, end := page.getStartEnd()
						for i := start; i <= end; i++ {
							item := page.items[i]
							item.selected = !item.selected
						}
					} else {
						selectedItem := page.items[page.cursor]
						selectedItem.selected = !selectedItem.selected
						page.moveCursor(1, m.height)
					}
					return m, nil
				}
			}

			switch m.mode {
			case normal, jump:
				switch msg.String() {
				case "left":
					return m.left()
				case "right", "enter":
					return m.right()
				}
			}
		}

		switch m.mode {
		case normal:
			switch m.submode {
			case goMode:
				switch msg.String() {
				case "esc":
					m.submode = noSubmode
					return m, nil
				case "g":
					m.submode = noSubmode
					m.mode = path
					page := m.getPage()
					m.pathInput.Reset()
					m.pathInput.SetValue(page.dir)
					m.pathInput.Focus()
					return m, textinput.Blink
				}

			case noSubmode:
				switch msg.String() {
				case "Q":
					return m, tea.Quit
				case "q":
					m.result = m.getTab().dir
					return m, tea.Quit
				case "g":
					m.submode = goMode
					return m, nil
				// case "d":
				// 	return m, newErr(errors.New("EPIC FAIL"))
				case "j":
					page := m.getPage()
					page.moveCursor(1, m.height)
					return m, nil
				case "k":
					page := m.getPage()
					page.moveCursor(-1, m.height)
					return m, nil
				case "h":
					return m.left()
				case "l":
					return m.right()
				case "tab":
					m.mode = jump
					return m, nil
				case "v":
					page := m.getPage()
					page.visual = page.cursor
					m.mode = visual
					return m, nil
				case "f":
					m.mode = filter
					m.filterInput.Reset()
					m.filterInput.Focus()
					return m, textinput.Blink
				case "`":
					m.mode = messages
					m.logStart = 0
					return m, nil
				}
			}

		case jump:
			switch msg.String() {
			case "esc", "tab":
				m.mode = normal
				return m, nil
			default:
				if len(msg.Runes) > 0 { // just in case, I dunno
					r := unicode.ToUpper(msg.Runes[0])
					page := m.getPage()
					var matches []int
					for i := range page.items {
						runes := []rune(page.items[i].name)
						if len(runes) == 0 {
							continue
						}
						if unicode.ToUpper(runes[0]) == r {
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
						page.updateStart(m.height)
					}
				}
				return m, nil
			}

		case visual:
			switch msg.String() {
			case "esc":
				m.mode = normal
				return m, nil
			case "v":
				m.mode = normal
				return m, nil
			}

		case filter:
			switch msg.String() {
			case "esc":
				m.mode = normal
				return m, nil
			}

		case path:
			switch msg.String() {
			case "esc":
				m.mode = normal
				return m, nil
			case "enter":
				// TODO: handle ~ and env vars
				dir := m.pathInput.Value()
				if strings.TrimSpace(dir) == "" {
					return m.addMessage(msgError, "empty path")
				}
				if strings.HasSuffix(dir, ":") {
					dir += "\\" // windows...
				}
				dir = filepath.Clean(dir)
				if !dirExists(dir) {
					return m.addMessage(msgError, fmt.Sprintf("directory \"%s\" doesn't exists", dir))
				}
				tab := m.getTab()
				m.mode = normal
				if tab.dir == dir {
					return m, nil
				}
				tab.dir = dir
				_, exists := tab.pages[dir]
				if exists {
					return m, nil
				}
				tab.pages[dir] = &page{dir: dir}
				return m, m.readDir(dir)
			case "ctrl+w":
				path := m.pathInput.Value()
				parent := filepath.Dir(path)
				m.pathInput.SetValue(parent)
				return m, nil
			case "ctrl+a":
				m.pathInput.SetValue("")
				return m, nil
			}

		case messages:
			switch msg.String() {
			case "esc", "`":
				m.mode = normal
				return m, nil
			case "j", "down":
				m.logStart += 1
				m.logStart = min(len(m.log)-1, m.logStart)
				return m, nil
			case "k", "up":
				m.logStart -= 1
				m.logStart = max(m.logStart, 0)
				return m, nil
			case "pgdown":
				m.logStart += 3
				m.logStart = min(len(m.log)-1, m.logStart)
				return m, nil
			case "pgup":
				m.logStart -= 3
				m.logStart = max(m.logStart, 0)
				return m, nil
			case "home":
				m.logStart = 0
				return m, nil
			case "end":
				m.logStart = len(m.log) - 1
				return m, nil
			case "Q":
				return m, tea.Quit
			case "q":
				m.result = m.getTab().dir
				return m, tea.Quit
			}
		}
	}

	var cmds []tea.Cmd

	switch m.mode {
	case filter:
		var cmd tea.Cmd
		m.filterInput, cmd = m.filterInput.Update(msg)
		cmds = append(cmds, cmd)
	case path:
		var cmd tea.Cmd
		m.pathInput, cmd = m.pathInput.Update(msg)
		fillAutocomplete(&m)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
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

	if m.mode == messages {
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
		return s.String()
	}

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
		switch item.action {
		case none:
			s.WriteString(style.Render(" "))
		case copy:
			s.WriteString(m.theme.copiedStyle.Render(" "))
		case cut:
			s.WriteString(m.theme.cutStyle.Render(" "))
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
	case jump:
		mode_style = mode_style.Background(m.theme.accentColor1)
		modeStr = "JUMP"
	case filter:
		mode_style = mode_style.Background(m.theme.accentColor2)
		modeStr = "FILTER"
	case path:
		mode_style = mode_style.Background(m.theme.grayColor)
		modeStr = "PATH"
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
	itemName = ansi.Truncate(itemName, nameWidth-2, "…")

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
		if finalModel.result != "" {
			err := os.WriteFile(tempFile, []byte(finalModel.result), 0644)
			if err != nil {
				log.Fatalf("error: %s\n", err)
			}
		}
	} else {
		fmt.Println(finalModel.result)
	}
}
