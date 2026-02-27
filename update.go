package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case errorMsg:
		// m.err = msg.err
		// return m, nil
		return m, m.addMessage(msgError, msg.err.Error())

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
				fmt.Sprintf("command \"%s\" failed: %s", msg.message, msg.err))
		} else {
			return m, tea.Batch(
				m.addMessage(msgDone, fmt.Sprintf("command: %s", msg.message)),
				m.update(msg.dir))
		}

	case readDirMsg:
		tab := m.tabs[msg.tab]
		if tab.dir != msg.dir {
			return m, nil
		}
		err := m.fillPage(msg.tab, msg.items)
		if err != nil {
			return m, m.addMessage(msgError, err.Error())
		}
		settings := tab.getPageSettings()
		length := len(msg.items)
		if settings.cursor >= length {
			settings.cursor = length - 1
		}
		if settings.cursor < 0 {
			settings.cursor = 0
		}
		if settings.start >= length {
			settings.start = length - 1
		}
		if settings.start < 0 {
			settings.start = 0
		}
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.MouseMsg:
		switch msg.Action {
		case tea.MouseActionPress:
			switch msg.Button {
			case tea.MouseButtonWheelUp:
				return m.handleWheel(-3)
			case tea.MouseButtonWheelDown:
				return m.handleWheel(3)
			}
		case tea.MouseActionRelease:
			m.click = newClick(msg.X, msg.Y, msg.Button, &m.click)
			switch msg.Button {
			case tea.MouseButtonLeft:
				switch m.mode {
				case normalMode, visualMode:
					if m.click.y == 0 {
						if m.mode == visualMode {
							return m, nil
						}
						tab := m.getTab()
						diskExp := regexp.MustCompile(`^([a-zA-Z]+:\\)`)
						matches := diskExp.FindStringSubmatch(tab.dir)
						start := 0
						if len(matches) > 1 {
							start = len(matches[1])
							if m.click.x < start {
								tab.dir = matches[1]
								tab.page = &page{}
								return m, m.readDir(m.currentTab, matches[1])
							}
						}
						runes := []rune(tab.dir)
						if m.click.x < len(runes) && m.click.x >= start {
							var history []string
							current := tab.dir
							for {
								history = append(history, current)
								parent := filepathDir(current)
								if parent == current {
									break
								}
								current = parent
							}
							slices.Reverse(history)

							clickedPath := string(runes[:m.click.x+1])
							clickedDir := filepathDir(clickedPath)

							index := slices.Index(history, clickedDir)

							// return m, m.addMessage(msgInfo, fmt.Sprintf("%#v %d %v", history, index, clickedDir))
							tab.dir = history[index+1]
							tab.page = &page{}
							return m, m.readDir(m.currentTab, tab.dir)
						}
					} else if m.click.y < m.height {
						tab := m.getTab()
						settings := tab.getPageSettings()
						if m.click.y-1 < len(tab.page.items)-settings.start {
							settings.cursor = m.click.y - 1 + settings.start
							if m.click.doubleClick && m.mode != visualMode {
								return m.right()
							}
						}
					}
				case tabsMode:
					if m.click.y > 0 &&
						m.click.y < m.height &&
						m.click.y-1 < len(m.tabs)-m.tabsStart {
						m.tabsCursor = m.click.y - 1 + m.tabsStart
						if m.click.doubleClick {
							m.mode = normalMode
							m.currentTab = m.tabsCursor
						}
					}
				}
				return m, nil
			}
			return m, nil
		}

	case tea.KeyMsg:
		switch m.mode {
		case normalMode, jumpMode, visualMode:
			switch msg.String() {
			case "ctrl+a":
				page := m.getPage()
				for i := range page.items {
					page.items[i].setSelected(true)
				}
				return m, nil
			case "ctrl+r":
				page := m.getPage()
				for i := range page.items {
					page.items[i].setSelected(!page.items[i].isSelected())
				}
				return m, nil
			case "ctrl+d":
				page := m.getPage()
				for i := range page.items {
					page.items[i].setSelected(false)
				}
				return m, nil

			case "down":
				m.moveCursor(1)
				return m, nil
			case "up":
				m.moveCursor(-1)
				return m, nil
			case "pgdown":
				m.moveCursor((m.height - 3) / 2)
				return m, nil
			case "pgup":
				m.moveCursor(-(m.height - 3) / 2)
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
				if m.mode == visualMode {
					start, end := m.getStartEnd()
					for i := start; i <= end; i++ {
						item := tab.page.items[i]
						item.setSelected(!item.isSelected())
					}
					m.mode = normalMode
				} else {
					settings := tab.getPageSettings()
					selectedItem := tab.page.items[settings.cursor]
					selectedItem.setSelected(!selectedItem.isSelected())
					m.moveCursor(1)
				}
				return m, nil
			}
		}

		switch m.mode {
		case normalMode, jumpMode:
			switch msg.String() {
			case "left":
				return m.left()
			case "right", "enter":
				return m.right()
			}
		}

		switch m.mode {
		case goMode:
			switch msg.String() {
			case "esc":
				m.mode = normalMode
				return m, nil
			case "g":
				m.mode = pathMode
				m.pathInput.Reset()
				m.pathInput.SetValue(m.getTab().dir)
				m.pathInput.Focus()
				m.pathInputDir = "nope"
				return m, textinput.Blink
			case "t":
				m.mode = tabsMode
				m.tabsCursor = m.currentTab
				m.updateTabsStart()
				return m, nil
			}

		case helpMode:
			switch msg.String() {
			case "esc":
				m.mode = normalMode
				return m, nil
			case "j", "down":
				m.help++
				return m, nil
			case "k", "up":
				m.help--
				m.help = max(0, m.help)
				return m, nil
			case "home":
				m.help = 0
				return m, nil
			case "f":
				m.mode = helpFilterMode
				m.input.Placeholder = "e.g., undo"
				m.input.Reset()
				m.input.Focus()
				return m, textinput.Blink
			}

		case helpFilterMode:
			switch msg.String() {
			case "esc":
				m.mode = helpMode
				return m, nil
			case "enter":
				m.mode = helpMode
				m.helpFilter = m.input.Value()
				return m, nil
			}

		case confirmDialogMode, confirmDialogVisualMode:
			return m.handleConfirm(msg)

		case normalMode:
			switch msg.String() {
			case "Q":
				return m.handleQuit(false)
			case "q":
				return m.handleQuit(true)
			case "f1":
				m.mode = helpMode
				m.help = 0
				m.helpFilter = ""
				return m, nil
			case "g":
				m.mode = goMode
				return m, nil
			case "t":
				tabCopy := *m.getTab()
				m.tabs = append(m.tabs, &tabCopy)
				m.currentTab = len(m.tabs) - 1
				return m, m.addMessage(msgInfo, "tab added")
			case "]":
				m.currentTab = (m.currentTab + 1) % len(m.tabs)
				return m, m.addMessage(msgInfo, fmt.Sprintf("tab %d", m.currentTab+1))
			case "[":
				m.currentTab = m.currentTab - 1
				if m.currentTab < 0 {
					m.currentTab = len(m.tabs) - 1
				}
				return m, m.addMessage(msgInfo, fmt.Sprintf("tab %d", m.currentTab+1))
			case "1", "2", "3", "4", "5", "6", "7", "8", "9", "0":
				index, _ := strconv.Atoi(msg.String()) // it shouldn't ever err
				if index == 0 {
					index = 9
				} else {
					index--
				}
				if index >= len(m.tabs) {
					return m, m.addMessage(msgWarning, fmt.Sprintf("tab %d doesn't exist", index+1))
				}
				m.currentTab = index
				return m, m.addMessage(msgInfo, fmt.Sprintf("tab %d", index+1))
			case "ctrl+w":
				if len(m.tabs) == 1 {
					return m, m.addMessage(msgWarning, "can't close the last tab")
				}
				m.closedTabs = append(m.closedTabs, m.getTab().dir)
				m.tabs = slices.Delete(m.tabs, m.currentTab, m.currentTab+1)
				if m.currentTab >= len(m.tabs) {
					m.currentTab = len(m.tabs) - 1
				}
				return m, nil
			case "T":
				return m.handleRestoreTab()
			case "d":
				m.confirm(&deleteCommand{m.getTab().dir, m.getPaths()})
				return m, nil
			// case "d":
			// 	return m, newErr(errors.New("EPIC FAIL"))
			case "r":
				return m.handleRename()
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
				m.mode = jumpMode
				return m, nil
			case "v":
				settings := m.getTab().getPageSettings()
				m.visual = settings.cursor
				m.mode = visualMode
				return m, nil
			case "f":
				m.mode = filterMode
				m.input.Placeholder = "e.g., term1;term2,term3"
				m.input.Reset()
				m.input.Focus()
				return m, textinput.Blink
			case ",":
				m.mode = sortMode
				return m, nil
			case "a":
				m.mode = createMode
				m.input.Placeholder = "e.g., filename.txt or dirname/"
				m.input.Reset()
				m.input.Focus()
				return m, textinput.Blink
			case "`":
				m.mode = messagesMode
				m.logStart = 0
				return m, nil
			case "f5":
				return m, tea.Batch(
					m.addMessage(msgInfo, fmt.Sprintf("tab %d updated", m.currentTab+1)),
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
			case "U":
				if !m.cm.canRedo() {
					return m, m.addMessage(msgWarning, "nothing to redo")
				}
				m.jobs++
				return m, tea.Batch(
					m.addMessage(msgInfo, "redo"),
					m.spinner.Tick,
					m.redo())
			case "p":
				return m.handlePaste(false)
			case "P":
				return m.handlePaste(true)
			}

		case jumpMode:
			switch msg.String() {
			case "esc", "tab":
				m.mode = normalMode
				return m, nil
			default:
				if len(msg.Runes) > 0 { // just in case, I dunno
					r := unicode.ToUpper(msg.Runes[0])
					page := m.getPage()
					var matches []int
					for i := range page.items {
						runes := []rune(page.items[i].getName())
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

		case visualMode:
			switch msg.String() {
			case "esc":
				m.mode = normalMode
				return m, nil
			case "v":
				m.mode = normalMode
				return m, nil
			case "y":
				msg := m.copyCut(false)
				m.mode = normalMode
				return m, tea.Batch(m.addMessage(msgInfo, msg), m.update(m.getTab().dir))
			case "x":
				msg := m.copyCut(true)
				m.mode = normalMode
				return m, tea.Batch(m.addMessage(msgInfo, msg), m.update(m.getTab().dir))
			case "r":
				return m.handleRename()
			case "d":
				m.confirm(&deleteCommand{dir: m.getTab().dir, paths: m.getPaths()})
				return m, nil
			}

		case filterMode:
			switch msg.String() {
			case "esc":
				m.mode = normalMode
				page := m.getPage()
				page.tempItems = nil
				return m, nil
			case "enter":
				m.mode = normalMode
				m.filterItems()
				return m, nil
			}

		case sortMode:
			switch msg.String() {
			case "esc", ",":
				m.mode = normalMode
				return m, nil
			case "m":
				m.mode = normalMode
				m.sort(modifiedTimeSort, false)
				return m, m.addMessage(msgInfo, "sorted by modified time")
			case "M":
				m.mode = normalMode
				m.sort(modifiedTimeSort, true)
				return m, m.addMessage(msgInfo, "sorted by modified time (reverse)")
			case "a":
				m.mode = normalMode
				m.sort(alphabeticSort, false)
				return m, m.addMessage(msgInfo, "sorted alphabetically")
			case "A":
				m.mode = normalMode
				m.sort(alphabeticSort, true)
				return m, m.addMessage(msgInfo, "sorted alphabetically (reverse)")
			case "e":
				m.mode = normalMode
				m.sort(extensionSort, false)
				return m, m.addMessage(msgInfo, "sorted by extension")
			case "E":
				m.mode = normalMode
				m.sort(extensionSort, true)
				return m, m.addMessage(msgInfo, "sorted by extension (reverse)")
			case "n":
				m.mode = normalMode
				m.sort(normalSort, false)
				return m, m.addMessage(msgInfo, "sorted normally")
			case "N":
				m.mode = normalMode
				m.sort(normalSort, true)
				return m, m.addMessage(msgInfo, "sorted normally (reverse)")
			case "s":
				m.mode = normalMode
				m.sort(sizeSort, false)
				return m, m.addMessage(msgInfo, "sorted by size")
			case "S":
				m.mode = normalMode
				m.sort(sizeSort, true)
				return m, m.addMessage(msgInfo, "sorted by size (reverse)")
			case "r":
				m.mode = normalMode
				m.sort(randomSort, false)
				return m, m.addMessage(msgInfo, "sorted randomly")
			}

		case renameMode:
			switch msg.String() {
			case "esc":
				m.mode = normalMode
				m.renamePaths = nil
				return m, nil
			case "enter":
				m.mode = normalMode
				if len(m.renamePaths) == 1 {
					value := m.input.Value()
					dir := filepath.Dir(m.renamePaths[0])
					path := filepath.Join(dir, value)
					finalPath := uniquePath(nil, path)
					pairs := []pathPair{{m.renamePaths[0], finalPath}}
					cmd := &fileActionCommand{
						action: renameFileAction,
						dir:    m.getTab().dir,
						pairs:  pairs,
					}
					if finalPath != path {
						return m, tea.Batch(m.addCommand(cmd),
							m.addMessage(msgWarning, fmt.Sprintf("%s already exists", path)))

					} else {
						m.jobs++
						return m, m.addCommand(cmd)
					}
				} else {
					return m, m.addMessage(msgError, "not implemented")
				}
			}

		case createMode:
			switch msg.String() {
			case "esc":
				m.mode = normalMode
				return m, nil
			case "enter":
				m.mode = normalMode
				name := m.input.Value()
				dir := m.getTab().dir
				cmd := newCreateCommand(name, dir)
				m.jobs++
				return m, m.addCommand(cmd)
			}

		case pathMode:
			switch msg.String() {
			case "esc":
				m.mode = normalMode
				return m, nil
			case "enter":
				return m.handleNewPath(false)
			case "ctrl+n":
				return m.handleNewPath(true)
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

		case tabsMode:
			switch msg.String() {
			case "esc":
				m.mode = normalMode
				return m, nil
			case "down", "j":
				m.tabsCursor++
				m.tabsCursor = min(m.tabsCursor, len(m.tabs)-1)
				m.updateTabsStart()
				return m, nil
			case "up", "k":
				m.tabsCursor--
				m.tabsCursor = max(m.tabsCursor, 0)
				m.updateTabsStart()
				return m, nil
			case "enter":
				m.mode = normalMode
				m.currentTab = m.tabsCursor
				return m, nil
			case "d":
				if len(m.tabs) == 1 {
					return m, m.addMessage(msgWarning, "can't close the last tab")
				}
				m.closedTabs = append(m.closedTabs, m.tabs[m.tabsCursor].dir)
				m.tabs = slices.Delete(m.tabs, m.tabsCursor, m.tabsCursor+1)
				if m.tabsCursor == m.currentTab {
					m.tabsCursor = min(m.tabsCursor, len(m.tabs)-1)
					m.currentTab = m.tabsCursor
				} else if m.tabsCursor < m.currentTab {
					m.currentTab = m.currentTab - 1
				} else {
					m.tabsCursor = m.tabsCursor - 1
				}
				return m, nil
			case "u":
				return m.handleRestoreTab()
			case "J":
				if len(m.tabs) == 1 {
					return m, nil
				}
				nextIndex := m.tabsCursor + 1
				if nextIndex > len(m.tabs)-1 {
					return m, nil
				}
				temp1 := *m.tabs[m.tabsCursor]
				temp2 := *m.tabs[nextIndex]
				m.tabs[m.tabsCursor] = &temp2
				m.tabs[nextIndex] = &temp1
				m.tabsCursor = m.tabsCursor + 1
				m.updateTabsStart()
				return m, nil
			case "K":
				if len(m.tabs) == 1 {
					return m, nil
				}
				nextIndex := m.tabsCursor - 1
				if nextIndex < 0 {
					return m, nil
				}
				temp1 := *m.tabs[m.tabsCursor]
				temp2 := *m.tabs[nextIndex]
				m.tabs[m.tabsCursor] = &temp2
				m.tabs[nextIndex] = &temp1
				m.tabsCursor = m.tabsCursor - 1
				m.updateTabsStart()
				return m, nil
			case "a":
				temp := *m.tabs[m.tabsCursor]
				for i := range m.tabs {
					if i != m.tabsCursor {
						m.closedTabs = append(m.closedTabs, m.tabs[i].dir)
					}
				}
				m.tabs = nil
				m.tabs = append(m.tabs, &temp)
				m.currentTab = 0
				m.tabsCursor = 0
				m.tabsStart = 0
				return m, nil
			case "c":
				dir := m.tabs[m.tabsCursor].dir
				err := clipboardWrite(dir)
				if err != nil {
					return m, m.addMessage(msgError, fmt.Sprintf("failed to set clipboard: %s", err))
				}
				return m, m.addMessage(msgInfo, fmt.Sprintf("%s copied to clipboard", dir))
			case "q":
				return m.handleQuit(true)
			case "Q":
				return m.handleQuit(false)
			}

		case messagesMode:
			switch msg.String() {
			case "esc", "`":
				m.mode = normalMode
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
				m.logStart += m.height / 2
				m.logStart = min(len(m.log)-1, m.logStart)
				return m, nil
			case "pgup":
				m.logStart -= m.height / 2
				m.logStart = max(m.logStart, 0)
				return m, nil
			case "home":
				m.logStart = 0
				return m, nil
			case "end":
				m.logStart = len(m.log) - 1
				return m, nil
			case "Q":
				return m.handleQuit(false)
			case "q":
				return m.handleQuit(true)
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
	case filterMode, helpFilterMode, renameMode, createMode:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		switch msg.(type) {
		case tea.KeyMsg:
			switch m.mode {
			case helpFilterMode:
				m.help = 0
				m.helpFilter = m.input.Value()
			case filterMode:
				m.filterItems()
			}
		}
		cmds = append(cmds, cmd)
	case pathMode:
		var cmd tea.Cmd
		m.pathInput, cmd = m.pathInput.Update(msg)
		fillAutocomplete(&m)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}

func (m *model) filterItems() {
	patterns := strings.FieldsFunc(m.input.Value(), func(r rune) bool {
		return r == ',' || r == ';'
	})
	var tempItems []item
	page := m.getPage()
	for i := range page.items {
		for j := range patterns {
			if strings.Contains(page.items[i].getName(), patterns[j]) {
				tempItems = append(tempItems, page.items[i])
			}
		}
	}
	page.tempItems = tempItems
	settings := m.getTab().getPageSettings()
	settings.cursor = 0
	settings.start = 0
}
