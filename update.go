package main

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

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

	case commandDoneMsg:
		m.jobs--
		if msg.err != nil {
			return m, m.addMessage(
				msgFail,
				fmt.Sprintf("command \"%s\" failed: %s", msg.message, msg.message))
		} else {
			return m, tea.Batch(m.addMessage(msgDone, msg.message), m.update(msg.dir))
		}

	case readDirMsg:
		err := m.fillPage(msg.tab, msg.entries)
		if err != nil {
			return m, newErr(err)
		}
		settings := m.tabs[msg.tab].getPageSettings()
		length := len(msg.entries)
		if settings.cursor >= length {
			settings.cursor = length - 1
		}
		if settings.start >= length {
			settings.start = length - 1
		}
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
				case tea.KeyCtrlD:
					page := m.getPage()
					for i := range page.items {
						page.items[i].selected = false
					}
				}
				switch msg.String() {
				case "down":
					m.moveCursor(1)
					return m, nil
				case "up":
					m.moveCursor(-1)
					return m, nil
				case "pgdown":
					m.moveCursor(3)
					return m, nil
				case "pgup":
					m.moveCursor(-3)
					return m, nil
				case "home":
					settings := m.getTab().getPageSettings()
					settings.cursor = 0
					m.updateStart()
					return m, nil
				case "end":
					tab := m.getTab()
					settings := tab.getPageSettings()
					settings.cursor = tab.page.length() - 1
					m.updateStart()
					return m, nil
				case " ":
					tab := m.getTab()
					if m.mode == visual {
						start, end := m.getStartEnd()
						for i := start; i <= end; i++ {
							item := tab.page.items[i]
							item.selected = !item.selected
						}
					} else {
						settings := tab.getPageSettings()
						selectedItem := tab.page.items[settings.cursor]
						selectedItem.selected = !selectedItem.selected
						m.moveCursor(1)
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
					m.pathInput.Reset()
					m.pathInput.SetValue(m.getTab().dir)
					m.pathInput.Focus()
					m.pathInputDir = "nope"
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
					m.moveCursor(1)
					return m, nil
				case "k":
					m.moveCursor(-1)
					return m, nil
				case "h":
					return m.left()
				case "l":
					return m.right()
				case "tab":
					m.mode = jump
					return m, nil
				case "v":
					settings := m.getTab().getPageSettings()
					m.visual = settings.cursor
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
				case "f5":
					return m, tea.Batch(
						m.addMessage(msgInfo, fmt.Sprintf("tab %d updated", m.currentTab)),
						m.update(m.getTab().dir))
				case "y":
					msg := m.copyCut(false)
					return m, tea.Batch(m.addMessage(msgInfo, msg), m.update(m.getTab().dir))
				case "x":
					msg := m.copyCut(true)
					return m, tea.Batch(m.addMessage(msgInfo, msg), m.update(m.getTab().dir))
				case "u":
					if !m.cm.canUndo() {
						return m, m.addMessage(msgWarning, "nothing to undo")
					}
					m.jobs++
					return m, tea.Batch(
						m.addMessage(msgInfo, "undo"),
						m.spinner.Tick,
						m.undo())

				case "p":
					paths, op, err := getClipboardFiles()
					if err != nil {
						return m, m.addMessage(msgWarning, "nothing to paste")
					}
					m.jobs++
					var cmd command
					switch op {
					case OpCopy:
						cmd = newCopyCommand(paths, m.getTab().dir, false)
					}
					return m, tea.Batch(
						m.addMessage(msgInfo, fmt.Sprintf("command: %s", cmd)),
						m.spinner.Tick,
						m.execute(cmd, m.getTab().dir))
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
						settings := m.getTab().getPageSettings()
						if slices.Contains(matches, settings.cursor) {
							index := slices.Index(matches, settings.cursor)
							next := (index + 1) % len(matches)
							settings.cursor = matches[next]
						} else {
							settings.cursor = matches[0]
						}
						m.updateStart()
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
			case "y":
				msg := m.copyCut(false)
				return m, tea.Batch(m.addMessage(msgInfo, msg), m.update(m.getTab().dir))
			case "x":
				msg := m.copyCut(true)
				return m, tea.Batch(m.addMessage(msgInfo, msg), m.update(m.getTab().dir))
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
				dir := m.pathInput.Value()
				if strings.TrimSpace(dir) == "" {
					return m, m.addMessage(msgError, "empty path")
				}
				if strings.HasSuffix(dir, ":") {
					dir += "\\" // windows...
				}
				dir, err := expandWindowsEnv(dir)
				if err != nil {
					return m, m.addMessage(msgError, fmt.Sprintf("failed to expand Windows env:%s", err))
				}
				dir = filepath.Clean(dir)
				if !dirExists(dir) {
					return m, m.addMessage(msgError, fmt.Sprintf("directory \"%s\" doesn't exists", dir))
				}
				dir, err = realWindowsPath(dir)
				if err != nil {
					return m, m.addMessage(msgError, fmt.Sprintf("failed to get the real Windows path:%s", err))
				}
				tab := m.getTab()
				m.mode = normal
				if tab.dir == dir {
					return m, nil
				}
				tab.dir = dir
				tab.page = &page{}
				return m, m.readDir(m.currentTab, dir)
			case "ctrl+w":
				path := m.pathInput.Value()
				parent := filepath.Dir(path)
				m.pathInput.SetValue(parent)
				return m, nil
			case "ctrl+a":
				m.pathInput.SetValue("")
				return m, nil
			case "ctrl+e":
				dir := m.pathInput.Value()
				dir, err := expandWindowsEnv(dir)
				if err != nil {
					return m, m.addMessage(msgError, fmt.Sprintf("failed to expand Windows env:%s", err))
				}
				m.pathInput.Reset()
				m.pathInput.SetValue(dir)
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

	if m.jobs > 0 {
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

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
