package main

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

func (m *model) handleQuit(result bool) (tea.Model, tea.Cmd) {
	if m.jobs > 0 {
		return m, m.addMessage(msgError, "unfinished jobs")
	}
	if result {
		m.result = m.getTab().dir
	}
	return m, tea.Quit
}

func (m *model) handleConfirm(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "esc", "n":
			m.mode = normalMode
			return m, nil
		case "left", "right", "h", "l":
			m.yes = !m.yes
			return m, nil
		case "enter":
			if m.yes {
				m.mode = normalMode
				m.jobs++
				return m, m.addCommand(m.cmd)
			} else {
				m.mode = normalMode
				return m, nil
			}
		case "y":
			m.mode = normalMode
			m.jobs++
			return m, m.addCommand(m.cmd)
		}
	}

	return m, nil
}

func (m *model) handlePaste(override bool) (tea.Model, tea.Cmd) {
	paths, op, err := getClipboardFiles()
	if err != nil {
		return m, m.addMessage(msgWarning, "nothing to paste")
	}
	var cmd *fileActionCommand
	switch op {
	case OpCopy:
		cmd = newFileActionCommand(copyFileAction, paths, m.getTab().dir, override)
	case OpCut:
		cmd = newFileActionCommand(cutFileAction, paths, m.getTab().dir, override)
	}
	if cmd.collision {
		m.confirm(cmd)
		return m, nil
	}
	m.jobs++
	return m, m.addCommand(cmd)
}

func (m *model) handleRename() (tea.Model, tea.Cmd) {
	paths := m.getPaths()
	if len(paths) != 1 {
		return m, m.addMessage(msgError, "only one path is supported right now")
	}
	m.mode = renameMode
	m.renamePaths = paths
	m.input.Placeholder = ""
	m.input.Reset()
	m.input.Focus()
	m.input.SetValue(filepath.Base(paths[0]))
	return m, textinput.Blink
}

func (m *model) handleWheel(steps int) (tea.Model, tea.Cmd) {
	switch m.mode {
	case normalMode:
		tab := m.getTab()
		if m.height-3 <= tab.page.length() {
			settings := tab.getPageSettings()
			settings.start += steps
			actualHeight := m.height - 3
			settings.start = max(0,
				min(settings.start, tab.page.length()-actualHeight))
		}
	}
	return m, nil
}

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
		err := m.fillPage(msg.tab, msg.items)
		if err != nil {
			return m, m.addMessage(msgError, err.Error())
		}
		settings := m.tabs[msg.tab].getPageSettings()
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
			switch msg.Button {
			case tea.MouseButtonLeft:
				m.click = newClick(msg.X, msg.Y, &m.click)
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
					page.items[i].selected = true
				}
				return m, nil
			case "ctrl+r":
				page := m.getPage()
				for i := range page.items {
					page.items[i].selected = !page.items[i].selected
				}
				return m, nil
			case "ctrl+d":
				page := m.getPage()
				for i := range page.items {
					page.items[i].selected = false
				}
				return m, nil

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
				if m.mode == visualMode {
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
				return m, nil
			case "[":
				m.currentTab = m.currentTab - 1
				if m.currentTab < 0 {
					m.currentTab = len(m.tabs) - 1
				}
				return m, nil
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
				if len(m.closedTabs) == 0 {
					return m, m.addMessage(msgWarning, "nothing to restore")
				}
				dir := m.closedTabs[len(m.closedTabs)-1]
				m.closedTabs = m.closedTabs[:len(m.closedTabs)-1]
				m.tabs = append(m.tabs, newTab(dir, &page{}))
				m.currentTab = len(m.tabs) - 1
				return m, m.readDir(m.currentTab, dir)
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
				return m, nil
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
			}

		case pathMode:
			switch msg.String() {
			case "esc":
				m.mode = normalMode
				return m, nil
			case "enter":
				dir := m.pathInput.Value()
				if strings.TrimSpace(dir) == "" {
					return m, m.addMessage(msgError, "empty path")
				}
				if isUNCroot(dir) {
					return m.changeDir(dir)
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
				if !isUNC(dir) {
					dir, err = realWindowsPath(dir)
					if err != nil {
						return m, m.addMessage(msgError, fmt.Sprintf("failed to get the real Windows path:%s", err))
					}
				}
				return m.changeDir(dir)
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
				return m, nil
			case "up", "k":
				m.tabsCursor--
				m.tabsCursor = max(m.tabsCursor, 0)
				return m, nil
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
	case filterMode, renameMode, createMode:
		var cmd tea.Cmd
		m.input, cmd = m.input.Update(msg)
		cmds = append(cmds, cmd)
	case pathMode:
		var cmd tea.Cmd
		m.pathInput, cmd = m.pathInput.Update(msg)
		fillAutocomplete(&m)
		cmds = append(cmds, cmd)
	}

	return m, tea.Batch(cmds...)
}
