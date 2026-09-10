package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"unicode"

	"mc/shutil"
	"mc/widgets/textinput"

	"mc/internal/event"
)

// emptyPaneKey lists the keys that still mean something in a pane with no
// tabs: switching or refilling the pane, and the global overlays. Everything
// else needs a current tab, so the gate in Update swallows it.
func emptyPaneKey(key string) bool {
	switch key {
	case "tab", "ctrl+left", "ctrl+right", "shift+left", "shift+right":
		return true
	case "T", "b", "g":
		return true // refill routes: restore, bookmarks, Go mode
	case "q", "Q", "w", "f1", "`", "ctrl+h":
		return true
	case "u", "U":
		return true
	}
	return false
}

// emptyPaneBlocked reports keys that reach a mode handler which would index the
// missing tab. The allowlist is keyed on the mode because Go and Tabs mode
// reuse letters that Normal mode lets through.
func (m *model) emptyPaneBlocked(key string) bool {
	if m.hasTabs() {
		return false
	}
	if m.taskView || m.quitting {
		return false // the overlays own the keyboard and never touch a tab
	}
	switch m.mode {
	case normalMode, jumpMode:
		return !emptyPaneKey(key)
	case goMode:
		// gs calculates directory sizes and needs a page of items.
		return key == "s"
	case tabsMode:
		switch key {
		case "esc", "h", "u", "q", "Q":
			return false
		}
		return true
	}
	return false
}

// emptyPaneClick drops pointer events aimed at a pane with no rows to hit. The
// overlay modes keep their clicks: bookmarks and the tab browser can refill it.
func (m *model) emptyPaneClick() bool {
	if m.hasTabs() || m.taskView || m.quitting {
		return false
	}
	return m.mode == normalMode || m.mode == jumpMode
}

func (m *model) Update(msg event.Msg) (event.Model, event.Cmd) {
	switch input := msg.(type) {
	case event.KeyMsg:
		m.clearHover()
		if !rangeKey(input.String()) {
			m.finishRangeSelection()
		}
		if m.emptyPaneBlocked(input.String()) {
			return m, nil
		}
	case event.MouseClickMsg:
		m.clearHover()
		if !input.Shift && !input.Ctrl {
			m.finishRangeSelection()
		}
		if m.emptyPaneClick() {
			return m, nil
		}
	case event.MouseWheelMsg:
		m.finishRangeSelection()
		if m.emptyPaneClick() {
			return m, nil
		}
	case event.BlurMsg, event.WindowSizeMsg:
		m.clearHover()
	}
	if handled, cmd := m.updateV2(msg); handled {
		return m, cmd
	}
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

	case processDoneMsg:
		return m, m.update(msg.dir)

	case searchTickMsg:
		if !m.search.working {
			return m, nil
		}

		items := make([]searchItem, 0, searchBufferSize)

	outer:
		for {
			select {
			case _, ok := <-m.search.cancel:
				if !ok {
					return m, nil
				}

			case result := <-m.search.result:
				items = append(items, result)

			default:
				break outer
			}
		}

		m.search.items = append(m.search.items, items...)

		done := false
		select {
		case _, ok := <-m.search.done:
			done = !ok
		default:
		}

		if !done {
			return m, searchTick()
		} else {
			m.search.stop()
			return m, m.addMessage(msgInfo, "done searching")
		}

	case massRenameMsg:
		lines := slices.Collect(readLines(msg.tempFile))
		os.Remove(msg.tempFile)
		// An emptied pane reads like navigating away: the rename is dropped.
		if m.currentDir() == msg.dir {
			if slices.Equal(msg.lines, lines) {
				return m, m.addMessage(msgError, "nothing changed")
			}
			if len(lines) != len(msg.lines) {
				return m, m.addMessage(msgError, "number of lines is wrong")
			}
			for i := range lines {
				if strings.TrimSpace(lines[i]) == "" {
					return m, m.addMessage(msgError,
						fmt.Sprintf("line %d is empty", i+1))
				}
			}
			pairs := buildRenamePairs(msg.paths, lines)
			cmd := &fileActionCommand{
				action: renameFileAction,
				dir:    msg.dir,
				pairs:  pairs,
			}
			m.addJob()
			return m, m.addCommand(cmd)
		}

	case selectItemMsg:
		tab := msg.target
		if tab.page != msg.page {
			return m, nil
		}
		tab.page.selectionRange = nil
		settings := tab.getPageSettings()
		for i, it := range tab.page.getItems() {
			if it.getFullPath() == msg.path {
				settings.cursor = i
				break
			}
		}
		revealCursor(settings, max(1, m.screenHeight-5))
		return m, nil

	case readDirMsg:
		tab := msg.target
		tab.pendingReads = max(0, tab.pendingReads-1)
		found := false
		for _, p := range m.panes {
			for _, t := range p.tabs {
				if t == tab {
					found = true
				}
			}
		}
		if !found || tab.dir != msg.dir || tab.page != msg.page || tab.readGeneration != msg.generation {
			return m, nil
		}
		if msg.err != nil {
			if msg.automatic && tab.lastReadError == msg.err.Error() {
				return m, nil
			}
			tab.lastReadError = msg.err.Error()
			return m, m.addMessage(msgError, msg.err.Error())
		}
		tab.lastReadError = ""
		if msg.automatic && unchangedListing(tab.page.items, msg.items) {
			return m, nil
		}
		m.clearHover()
		settings := tab.getPageSettings()
		cursor, start := settings.cursor, settings.start
		cursorPath, topPath := "", ""
		previous := tab.page.getItems()
		if cursor >= 0 && cursor < len(previous) {
			cursorPath = previous[cursor].getFullPath()
		}
		if start >= 0 && start < len(previous) {
			topPath = previous[start].getFullPath()
		}
		selected := map[string]bool{}
		calculatedSizes := map[string]struct {
			size uint64
			text string
		}{}
		for _, it := range tab.page.items {
			if it.isSelected() {
				selected[it.getFullPath()] = true
			}
			if old, ok := it.(*filepathItem); ok && old.isDir && old.sizeStr != "" {
				calculatedSizes[old.getFullPath()] = struct {
					size uint64
					text string
				}{size: old.size, text: old.sizeStr}
			}
		}
		for _, it := range msg.items {
			it.setSelected(selected[it.getFullPath()])
			if current, ok := it.(*filepathItem); ok && current.isDir {
				if saved, ok := calculatedSizes[current.getFullPath()]; ok {
					current.size, current.sizeStr = saved.size, saved.text
				}
			}
		}
		tab.page.items = msg.items
		tab.filter()
		if tab.sorted {
			tab.sortItems(tab.sortMethod, tab.sortReverse)
		}
		settings.cursor, settings.start = cursor, start
		for i, it := range tab.page.getItems() {
			if samePath(it.getFullPath(), cursorPath) {
				settings.cursor = i
			}
			if samePath(it.getFullPath(), topPath) {
				settings.start = i
			}
		}
		settings.update(tab.page.length())
		if settings.sel != nil {
			for i, it := range tab.page.getItems() {
				if it.getFullPath() == *settings.sel {
					settings.cursor = i
					break
				}
			}
			settings.sel = nil
			revealCursor(settings, max(1, m.screenHeight-5))
		}
		return m, nil

	case event.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case event.MouseHoverMsg:
		if msg.Search {
			m.hoverSearchIndex = msg.Index
			m.hoverPane, m.hoverIndex = -1, -1
			m.hoverTabPane, m.hoverTabIndex, m.hoverPathPane, m.hoverPathX = -1, -1, -1, -1
		} else if msg.Tab {
			m.hoverTabPane, m.hoverTabIndex = msg.Pane, msg.Index
			m.hoverPane, m.hoverIndex, m.hoverSearchIndex = -1, -1, -1
			m.hoverPathPane, m.hoverPathX = -1, -1
		} else if msg.Path {
			m.hoverPathPane, m.hoverPathX = msg.Pane, msg.X
			m.hoverPane, m.hoverIndex, m.hoverSearchIndex = -1, -1, -1
			m.hoverTabPane, m.hoverTabIndex = -1, -1
		} else {
			m.hoverPane, m.hoverIndex = msg.Pane, msg.Index
			m.hoverSearchIndex = -1
			m.hoverTabPane, m.hoverTabIndex, m.hoverPathPane, m.hoverPathX = -1, -1, -1, -1
		}
		return m, nil

	case event.MouseTabMsg:
		m.clearHover()
		m.click = mouseClick{}
		m.finishRangeSelection()
		if msg.Pane >= 0 && msg.Pane < len(m.panes) && msg.Index >= 0 && msg.Index < len(m.panes[msg.Pane].tabs) {
			m.activePane = msg.Pane
			m.pane = m.panes[msg.Pane]
			m.currentTab = msg.Index
			m.mode = normalMode
		}
		return m, nil

	case event.MouseDragMsg:
		if m.helpDragging {
			m.help = m.helpOffsetForRow(msg.Mouse().Y)
		}
		return m, nil

	case event.MouseUpMsg:
		m.helpDragging = false
		return m, nil

	case event.MouseWheelMsg:
		m.hoverPane, m.hoverIndex, m.hoverSearchIndex = -1, -1, -1
		m.hoverTabPane, m.hoverTabIndex, m.hoverPathPane, m.hoverPathX = -1, -1, -1, -1
		data := msg.Mouse()
		switch data.Button {
		case event.MouseWheelUp:
			return m.handleWheel(-3)
		case event.MouseWheelDown:
			return m.handleWheel(3)
		}

	case event.MouseClickMsg:
		data := msg.Mouse()
		switch data.Button {
		case event.MouseLeft:
			m.click = newClick(data.X, data.Y, &m.click)
			switch m.mode {
			case helpMode, helpFilterMode:
				if m.helpScrollbarHit(m.click.x, m.click.y) {
					m.helpDragging = true
					m.help = m.helpOffsetForRow(m.click.y)
				}
				return m, nil
			case normalMode, jumpMode:
				if m.click.y == 0 {
					m.finishRangeSelection()
					dir := m.getTab().dir
					if target := breadcrumbAtX(dir, m.click.x, m.width); target != "" {
						return m, m.changeDir(target)
					}
				} else if m.click.y < m.height-2 {
					tab := m.getTab()
					settings := tab.getPageSettings()
					if m.click.y >= 1 && m.click.y-1 < len(tab.page.getItems())-settings.start {
						if data.Shift || data.Ctrl {
							m.selectRangeTo(m.click.y - 1 + settings.start)
							m.click = mouseClick{}
							return m, nil
						}
						settings.cursor = m.click.y - 1 + settings.start
						if m.click.doubleClick {
							return m.right(false)
						}
					}
				}
			case tabsMode:
				if m.click.y > 0 &&
					m.click.y < m.height-1 &&
					m.click.y-1 < len(m.tabs)-m.tabsStart {
					m.tabsCursor = m.click.y - 1 + m.tabsStart
					if m.click.doubleClick {
						m.mode = normalMode
						m.currentTab = m.tabsCursor
					}
				}
			case bookmarksMode:
				if m.click.y > 0 &&
					m.click.y < m.height-1 &&
					m.click.y-1 < len(m.bm.dirs)-m.bm.start {
					m.bm.cursor = m.click.y - 1 + m.bm.start
					if m.click.doubleClick {
						m.mode = normalMode
						dir := m.bm.dirs[m.bm.cursor]
						if m.bm.changed() {
							err := saveBookmarks(m.bm.dirs)
							if err != nil {
								return m, m.addMessage(msgError, err.Error())
							}
						}
						m.bm = nil
						return m, m.changeDir(dir)
					}
				}
			case searchMode:
				if m.click.y == 1 {
					if m.search.focus != 0 {
						m.search.setFocus(0)
						return m, textinput.Blink
					}
				} else if m.click.y == 2 {
					if m.search.focus != 1 {
						m.search.setFocus(1)
						return m, textinput.Blink
					}
				} else if m.click.y >= 3 &&
					m.click.y < m.height-2 &&
					m.click.y-3 < m.search.length()-m.search.start {
					m.search.setFocus(2)
					m.search.cursor = m.click.y - 3 + m.search.start
					if m.click.doubleClick {
						return m.searchRight()
					}
					return m, nil
				}
			}
			return m, nil
		}

	case event.KeyMsg:
		switch m.mode {
		case normalMode, jumpMode:
			switch msg.String() {
			case "shift+up", "shift+down", "shift+home", "shift+end":
				end := m.getTab().getPageSettings().cursor
				switch msg.String() {
				case "shift+up":
					end--
				case "shift+down":
					end++
				case "shift+home":
					end = 0
				case "shift+end":
					end = m.getPage().length() - 1
				}
				m.selectRangeTo(end)
				return m, nil
			case "ctrl+a":
				items := m.getPage().getItems()
				for i := range items {
					items[i].setSelected(true)
				}
				return m, nil
			case "ctrl+r":
				items := m.getPage().getItems()
				for i := range items {
					items[i].setSelected(!items[i].isSelected())
				}
				return m, nil
			case "ctrl+d":
				items := m.getPage().getItems()
				for i := range items {
					items[i].setSelected(false)
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
				if tab.page.length() > 0 {
					settings.cursor = tab.page.length() - 1
					m.updateStart()
				}
				return m, nil
			case "space", "insert":
				tab := m.getTab()
				settings := tab.getPageSettings()
				items := tab.page.getItems()
				if len(items) > 0 {
					items[settings.cursor].setSelected(!items[settings.cursor].isSelected())
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
				return m.right(false)
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
				dir := m.currentDir()
				if dir != "" && !isUNCRoot(dir) { // these aren't valid directories
					os.Chdir(dir)
				}
				m.pathInput.Reset()
				m.pathInput.SetValue(dir)
				m.pathInput.Focus()
				m.pathInputDir = "nope"
				return m, textinput.Blink
			case "t":
				m.mode = tabsMode
				m.tabsCursor = m.currentTab
				m.updateTabsStart()
				return m, nil
			case "T":
				m.mode = themeMode
				m.themeCursor = findPresetIndex(m.cfg.Theme)
				m.themeOld = m.theme
				return m, nil
			case "c":
				m.mode = normalMode
				configDir := getConfigDir()
				if !shutil.DirExists(configDir) {
					err := os.MkdirAll(configDir, 0755)
					if err != nil {
						return m, m.addMessage(msgError, err.Error())
					}
				}
				return m, m.changeDir(configDir)
			case "C":
				m.mode = normalMode
				configPath := getConfigPath()
				configExists := shutil.PathExists(configPath)
				err := saveConfig(m.cfg)
				if err != nil {
					return m, m.addMessage(msgError, err.Error())
				}
				var info string
				if configExists {
					info = fmt.Sprintf("config overridden: %s", configPath)
				} else {
					info = fmt.Sprintf("config saved: %s", configPath)
				}

				dir := m.currentDir()
				cmd := m.addMessage(msgInfo, info)
				if dir == getConfigDir() {
					return m, event.Batch(cmd, m.update(dir))
				} else {
					return m, cmd
				}
			case "s":
				m.mode = normalMode
				items := m.getPage().getItems()
				if len(items) == 0 {
					return m, m.addMessage(msgError, "nothing selected")
				}
				_, ok := items[0].(*filepathItem)
				if !ok {
					return m, m.addMessage(msgError, "only filepaths are supported")
				}
				selectedPaths := m.getPaths()
				paths := make([]string, 0, len(selectedPaths))
				for i := range items {
					if items[i].isDirectory() && slices.Contains(selectedPaths, items[i].getFullPath()) {
						paths = append(paths, items[i].getFullPath())
					}
				}
				if len(paths) == 0 {
					return m, m.addMessage(msgError, "please select at least one directory")
				}
				m.addJob()
				return m, event.Batch(m.enqueueReadOnly(newCalcSizeCommand(m.getTab(), paths), "execute"), m.spinner.Tick)
			}

		case helpMode:
			switch msg.String() {
			case "esc":
				m.mode = normalMode
				m.helpDragging = false
				return m, nil
			case "j", "down":
				m.help = m.scrollHelp(1)
				return m, nil
			case "k", "up":
				m.help = m.scrollHelp(-1)
				return m, nil
			case "pgdown":
				m.help = m.scrollHelp((m.height - 1) / 2)
				return m, nil
			case "pgup":
				m.help -= (m.height - 1) / 2
				m.help = max(0, m.help)
				return m, nil
			case "home":
				m.help = 0
				return m, nil
			case "f":
				m.mode = helpFilterMode
				m.resetInput("e.g., undo")
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

		case confirmDialogMode:
			return m.handleConfirm(msg)

		case normalMode:
			switch msg.String() {
			case "f1":
				m.mode = helpMode
				m.help = 0
				m.helpFilter = ""
				m.helpDragging = false
				return m, nil
			case "Q":
				return m.handleQuit(false)
			case "q":
				return m.handleQuit(true)
			case "esc":
				tab := m.getTab()
				if tab.page.isTemp() {
					if len(tab.page.tempItems) > 0 {
						settings := tab.getPageSettings()
						selectedItem := tab.page.tempItems[settings.cursor]
						for i := range tab.page.items {
							if tab.page.items[i].getFullPath() == selectedItem.getFullPath() {
								settings.cursor = i
								break
							}
						}
						settings.update(len(tab.page.items))
					}
					tab.page.tempItems = nil
					tab.filterText = nil
					m.updateStart()
				}
				return m, nil
			case "ctrl+h":
				m.mode = hiddenMode
				return m, nil
			case "g":
				m.mode = goMode
				return m, nil
			case "t":
				dir := m.getTab().dir
				tabCopy := newTab(dir, &page{})
				m.tabs = append(m.tabs, tabCopy)
				m.currentTab = len(m.tabs) - 1
				return m, event.Batch(
					m.addMessage(msgInfo, "tab copied"),
					m.readDir(m.currentTab, dir),
				)
			case "ctrl+t", "ctrl+n":
				return m.right(true)
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
				m.closedTabs = append(m.closedTabs, m.getTab().dir)
				m.tabs = slices.Delete(m.tabs, m.currentTab, m.currentTab+1)
				m.clampCurrent()
				return m, nil
			case "T":
				return m.handleRestoreTab()
			case "d":
				paths := m.getPaths()
				if len(paths) == 0 {
					return m, m.addMessage(msgWarning, "nothing selected")
				}
				m.confirm(&deleteCommand{m.getTab().dir, paths})
				return m, nil
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
				return m.right(false)
			case "ctrl+b":
				tab := m.getTab()
				if !tab.hasPrev() {
					return m, m.addMessage(msgWarning, "no history, can't go back")
				}
				dir := tab.back()
				return m, m.readDir(m.currentTab, dir)
			case "ctrl+f":
				tab := m.getTab()
				if !tab.hasNext() {
					return m, m.addMessage(msgWarning, "can't go forward")
				}
				dir := tab.next()
				return m, m.readDir(m.currentTab, dir)
			case "shift+tab":
				m.mode = jumpMode
				return m, nil
			case "f":
				m.mode = filterMode
				m.resetInput("e.g., term1;term2,term3")
				tab := m.getTab()
				if tab.page.isTemp() {
					m.input.SetValue(strings.Join(tab.filterText, ";"))
				}
				return m, textinput.Blink
			case ",":
				m.mode = sortMode
				return m, nil
			case "a":
				m.mode = createMode
				m.resetInput("e.g., filename.txt or dirname/")
				return m, textinput.Blink
			case "c":
				m.mode = copyMode
				return m, nil
			case "`":
				m.mode = messagesMode
				m.logStart = 0
				return m, nil
			case "f5":
				return m, event.Batch(
					m.addMessage(msgInfo, fmt.Sprintf("tab %d updated", m.currentTab+1)),
					m.update(m.getTab().dir))
			case "y":
				msg := m.copyCut(false)
				return m, event.Batch(m.addMessage(msgInfo, msg), m.update(m.getTab().dir))
			case "x":
				msg := m.copyCut(true)
				return m, event.Batch(m.addMessage(msgInfo, msg), m.update(m.getTab().dir))
			case "u":
				if !m.cm.canUndo() {
					return m, m.addMessage(msgWarning, "nothing to undo")
				}
				cmd, err := m.cm.peekUndo()
				if err != nil {
					return m, m.addMessage(msgError, err.Error())
				}
				m.addJob()
				return m, event.Batch(
					m.addMessage(msgInfo, fmt.Sprintf("undo: %s", cmd)),
					m.spinner.Tick,
					m.runUndo(cmd))
			case "U":
				if !m.cm.canRedo() {
					return m, m.addMessage(msgWarning, "nothing to redo")
				}
				cmd, err := m.cm.peekRedo()
				if err != nil {
					return m, m.addMessage(msgError, err.Error())
				}
				m.addJob()
				return m, event.Batch(
					m.addMessage(msgInfo, fmt.Sprintf("redo: %s", cmd)),
					m.spinner.Tick,
					m.runRedo(cmd))
			case "p":
				return m.handlePaste(false)
			case "P":
				return m.handlePaste(true)
			case "f2", "f3", "f4", "f6", "f7", "f8", "f9", "f10", "f11", "f12":
				return m.handleTool(msg.String())
			case "b":
				bookmarks, err := loadBookmarks()
				if err != nil {
					m.mode = normalMode
					return m, m.addMessage(msgError, err.Error())
				}
				m.bm = newBookmarks(bookmarks)
				m.mode = bookmarksMode
				return m, nil
			case "B":
				bookmarks, err := loadBookmarks()
				if err != nil {
					return m, m.addMessage(msgError, err.Error())
				}
				dir := m.getTab().dir
				if slices.Contains(bookmarks, dir) {
					index := slices.Index(bookmarks, dir)
					bookmarks = slices.Delete(bookmarks, index, index+1)
				}
				result := make([]string, 0, len(bookmarks)+1)
				result = append(result, dir)
				result = append(result, bookmarks...)
				err = saveBookmarks(result)
				if err != nil {
					return m, m.addMessage(msgError, err.Error())
				}
				cmd := m.addMessage(msgInfo, fmt.Sprintf(`"%s" bookmarked`, dir))
				if dir == getConfigDir() {
					return m, event.Batch(cmd, m.update(dir))
				} else {
					return m, cmd
				}

			case "s":
				if m.search == nil {
					m.search = newSearch(m)
				}
				m.mode = searchMode
				return m, m.search.blink()

			case ";":
				return m.handleRepeatShell()
			case ":":
				shellHistory, err := loadShellHistory()
				if err != nil {
					return m, m.addMessage(msgError, fmt.Sprintf("failed to load shell history: %s", err))
				}
				m.mode = shellMode
				m.shellHistory = shellHistory
				m.shellHistoryCurrent = -1
				m.resetInput(fmt.Sprintf("%s (#sl - pipe selected)", SHELL))
				fillAutocomplete(m)
				return m, textinput.Blink
			}

		case hiddenMode:
			switch msg.String() {
			case "esc", "ctrl+h":
				m.mode = normalMode
				return m, nil
			}

		case searchMode:
			switch msg.String() {
			case "esc":
				if m.search.working {
					m.search.stop()
					return m, nil
				}
				m.mode = normalMode
				return m, nil
			case "tab":
				m.search.setFocus((m.search.focus + 1) % 3)
				return m, nil
			case "f1":
				m.search.gitignore = !m.search.gitignore
				return m, nil
			case "f2":
				m.search.caseIgnore = !m.search.caseIgnore
				return m, nil
			case "f3":
				if m.search.isItem(m.search.cursor) {
					return m.handleTool(msg.String())
				} else {
					i, j := m.search.mapIndex(m.search.cursor)
					if i < 0 || i >= len(m.search.items) {
						return m, nil
					}
					item := m.search.items[i]
					if j < 0 || j >= len(item.lines) {
						return m, nil
					}
					line := item.lines[j]
					text := line.line[line.start:line.end]
					var cmd *exec.Cmd
					switch m.cfg.F3.Command {
					case "bat":
						cmd = exec.Command(
							"bat",
							"--color=always",
							"-p",
							"--pager",
							fmt.Sprintf(`less -c -R -S +%dg/"%s"`, line.lineNumber, text),
							item.path,
						)
					case "koneko":
						cmd = exec.Command(
							"koneko",
							fmt.Sprintf("-theme=%s", m.cfg.Theme),
							fmt.Sprintf(
								"-select=%d:%d-%d:%d",
								line.lineNumber,
								line.start+1,
								line.lineNumber,
								line.end+1),
							item.path,
						)
					default:
						cmd = exec.Command(m.cfg.F3.Command, item.path)
					}
					cmd.Dir = m.getTab().dir
					return m, event.ExecProcess(cmd, nil)
				}
			case "f4", "f6", "f7", "f8", "f9", "f10", "f11", "f12":
				if m.search.isItem(m.search.cursor) {
					return m.handleTool(msg.String())
				}
			case "f5":
				return m.launchSearch()
			case "enter":
				switch m.search.focus {
				case 0, 1:
					return m.launchSearch()
				case 2:
					return m.searchRight()
				}
			case "h":
				if m.search.focus == 2 {
					if m.search.showLines {
						item, _ := m.search.mapIndex(m.search.cursor)
						m.search.cursor = item
						m.search.showLines = false
						m.search.start = 0
						m.search.updateStart(m.height)
						return m, nil
					} else {
						current := 0
						for i := range m.search.items {
							if i == m.search.cursor {
								m.search.cursor = current
								m.search.showLines = true
								m.search.start = 0
								m.search.updateStart(m.height)
								return m, nil
							}
							current++
							current += len(m.search.items[i].lines)
						}
					}
				}
			case "l", "right":
				if m.search.focus == 2 {
					return m.searchRight()
				}
			case "j", "down":
				if m.search.focus == 2 {
					m.search.moveCursor(1, m.height)
					return m, nil
				}
			case "k", "up":
				if m.search.focus == 2 {
					m.search.moveCursor(-1, m.height)
					return m, nil
				}
			case "end":
				if m.search.focus == 2 {
					m.search.cursor = m.search.length() - 1
					m.search.updateStart(m.height)
					return m, nil
				}
			case "home":
				if m.search.focus == 2 {
					m.search.cursor = 0
					m.search.updateStart(m.height)
					return m, nil
				}
			case "pgdown":
				if m.search.focus == 2 {
					m.search.moveCursor((m.height-5)/2, m.height)
					return m, nil
				}
			case "pgup":
				if m.search.focus == 2 {
					m.search.moveCursor(-((m.height - 5) / 2), m.height)
					return m, nil
				}
			}

		case shellMode:
			switch msg.String() {
			case "esc":
				m.mode = normalMode
				return m, nil
			case "ctrl+b":
				if len(m.shellHistory) == 0 {
					return m, m.addMessage(msgError, "no shell history")
				}
				m.shellHistoryCurrent++
				m.shellHistoryCurrent = min(len(m.shellHistory)-1, m.shellHistoryCurrent)
				m.input.SetValue(m.shellHistory[m.shellHistoryCurrent])
				fillAutocomplete(m)
				return m, nil
			case "ctrl+f":
				if len(m.shellHistory) == 0 {
					return m, m.addMessage(msgError, "no shell history")
				}
				m.shellHistoryCurrent--
				m.shellHistoryCurrent = max(-1, m.shellHistoryCurrent)
				if m.shellHistoryCurrent == -1 {
					m.input.SetValue("")
				} else {
					m.input.SetValue(m.shellHistory[m.shellHistoryCurrent])
				}
				fillAutocomplete(m)
				return m, nil
			case "enter":
				m.mode = normalMode
				cmdText := m.input.Value()
				err := saveShellHistory(m.shellHistory, cmdText)
				if err != nil {
					return m, m.addMessage(msgError, fmt.Sprintf("failed to save shell history: %s", err))
				}
				return m, m.runShellCommand(cmdText)
			}

		case jumpMode:
			switch msg.String() {
			case "esc", "tab":
				m.mode = normalMode
				return m, nil
			case "f2", "f3", "f4", "f6", "f7", "f8", "f9", "f10", "f11", "f12":
				return m.handleTool(msg.String())
			default:
				runes := []rune(msg.String())
				if len(runes) > 0 { // just in case, I dunno
					r := unicode.ToUpper(runes[0])
					items := m.getPage().getItems()
					var matches []int
					for i := range items {
						runes := []rune(items[i].getName())
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

		case filterMode:
			switch msg.String() {
			case "esc":
				m.mode = normalMode
				tab := m.getTab()
				tab.page.tempItems = nil
				tab.filterText = nil
				return m, nil
			case "enter":
				m.mode = normalMode
				m.setFilter()
				m.getTab().filter()
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
					value := strings.TrimSpace(m.input.Value())
					if value == "" {
						m.renamePaths = nil
						return m, m.addMessage(msgError, "the name is empty")
					}
					src := m.renamePaths[0]
					path := filepath.Join(filepath.Dir(src), value)
					pairs := buildRenamePairs([]string{src}, []string{value})
					finalPath := pairs[0].dst
					cmd := &fileActionCommand{
						action: renameFileAction,
						dir:    m.getTab().dir,
						pairs:  pairs,
					}
					if finalPath != path {
						m.addJob()
						return m, event.Batch(m.addCommand(cmd),
							m.addMessage(msgWarning, fmt.Sprintf("%s already exists", path)))

					} else {
						m.addJob()
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
				name := strings.TrimSpace(m.input.Value())
				if name == "" || name == "\\" || name == "/" {
					return m, m.addMessage(msgError, "the name is empty")
				}
				dir := m.getTab().dir
				cmd := newCreateCommand(name, dir)
				m.addJob()
				return m, m.addCommand(cmd)
			}

		case copyMode:
			switch msg.String() {
			case "esc":
				m.mode = normalMode
				return m, nil
			case "c":
				return m.handleClipboardCopy(clipboardCopyFilepath, false)
			case "C":
				return m.handleClipboardCopy(clipboardCopyFilepath, true)
			case "d":
				return m.handleClipboardCopy(clipboardCopyDirectory, false)
			case "D":
				return m.handleClipboardCopy(clipboardCopyDirectory, true)
			case "f":
				return m.handleClipboardCopy(clipboardCopyFilename, false)
			case "n":
				return m.handleClipboardCopy(clipboardCopyFilenameNoExt, false)
			case "a":
				return m.handleClipboardCopy(clipboardCopyFilepathArgs, false)
			case "A":
				return m.handleClipboardCopy(clipboardCopyFilepathArgs, true)
			case "s":
				return m.handleClipboardCopy(clipboardCopyFilenameArgs, false)
			case "q":
				return m.handleClipboardCopy(clipboardCopyFilepathArray, false)
			case "Q":
				return m.handleClipboardCopy(clipboardCopyFilepathArray, true)
			case "w":
				return m.handleClipboardCopy(clipboardCopyFilenameArray, false)
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

		case bookmarksMode:
			switch msg.String() {
			case "esc", "h":
				m.mode = normalMode
				if m.bm.changed() {
					err := saveBookmarks(m.bm.dirs)
					if err != nil {
						return m, m.addMessage(msgError, err.Error())
					}
				}
				m.bm = nil
				return m, nil
			case "down", "j":
				m.bm.moveCursor(1, m.height)
				return m, nil
			case "up", "k":
				m.bm.moveCursor(-1, m.height)
				return m, nil
			case "home":
				m.bm.cursor = 0
				m.bm.start = 0
				return m, nil
			case "end":
				m.bm.cursor = max(0, len(m.bm.dirs)-1)
				m.bm.updateStart(m.height)
				return m, nil
			case "pgdown":
				m.bm.moveCursor((m.height-3)/2, m.height)
				return m, nil
			case "pgup":
				m.bm.moveCursor(-(m.height-3)/2, m.height)
				return m, nil
			case "enter", "l":
				m.mode = normalMode
				if m.bm.changed() {
					err := saveBookmarks(m.bm.dirs)
					if err != nil {
						return m, m.addMessage(msgError, err.Error())
					}
				}
				if len(m.bm.dirs) == 0 {
					m.bm = nil
					return m, nil
				}
				dir := m.bm.dirs[m.bm.cursor]
				m.bm = nil
				return m, m.changeDir(dir)
			case "d":
				if len(m.bm.dirs) == 0 {
					return m, nil
				}
				m.bm.dirs = slices.Delete(m.bm.dirs, m.bm.cursor, m.bm.cursor+1)
				m.bm.cursor = min(len(m.bm.dirs)-1, m.bm.cursor)
				m.bm.updateStart(m.height)
				return m, nil
			case "J":
				if m.bm.cursor == len(m.bm.dirs)-1 {
					return m, nil
				}
				m.bm.dirs[m.bm.cursor], m.bm.dirs[m.bm.cursor+1] =
					m.bm.dirs[m.bm.cursor+1], m.bm.dirs[m.bm.cursor]
				m.bm.cursor++
				return m, nil
			case "K":
				if m.bm.cursor == 0 {
					return m, nil
				}
				m.bm.dirs[m.bm.cursor], m.bm.dirs[m.bm.cursor-1] =
					m.bm.dirs[m.bm.cursor-1], m.bm.dirs[m.bm.cursor]
				m.bm.cursor--
				return m, nil
			}

		case tabsMode:
			switch msg.String() {
			case "esc", "h":
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
			case "enter", "l":
				m.mode = normalMode
				m.currentTab = m.tabsCursor
				return m, nil
			case "d":
				m.closedTabs = append(m.closedTabs, m.tabs[m.tabsCursor].dir)
				m.tabs = slices.Delete(m.tabs, m.tabsCursor, m.tabsCursor+1)
				if len(m.tabs) == 0 {
					m.tabsCursor, m.currentTab, m.tabsStart = 0, 0, 0
					return m, nil
				}
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

		case themeMode:
			switch msg.String() {
			case "esc":
				m.mode = normalMode
				m.theme = m.themeOld
				m.themeOld = nil
				return m, nil
			case "j", "down":
				m.themeCursor++
				m.themeCursor = min(len(themeList)-1, m.themeCursor)
				m.theme = newTheme(themeList[m.themeCursor].name)
				return m, nil
			case "k", "up":
				m.themeCursor--
				m.themeCursor = max(0, m.themeCursor)
				m.theme = newTheme(themeList[m.themeCursor].name)
				return m, nil
			case "l", "enter":
				m.mode = normalMode
				m.themeOld = nil
				name := themeList[m.themeCursor].name
				m.cfg.Theme = name
				m.updateTheme()
				return m, m.addMessage(msgInfo, fmt.Sprintf("%s theme set; don't forget to save the current config", name))
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

	var cmds []event.Cmd

	searching := false
	if m.search != nil {
		searching = m.search.working
	}

	if m.hasJobs() || searching {
		var cmd event.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		cmds = append(cmds, cmd)
	}

	switch m.mode {
	case filterMode, helpFilterMode, renameMode, createMode, shellMode:
		var cmd event.Cmd
		m.input, cmd = m.input.Update(msg)
		switch msg.(type) {
		case event.KeyMsg:
			switch m.mode {
			case helpFilterMode:
				m.help = 0
				m.helpFilter = m.input.Value()
			case filterMode:
				m.setFilter()
				m.getTab().filter()
			case shellMode:
				fillAutocomplete(m)
			}
		}
		cmds = append(cmds, cmd)
	case pathMode:
		var cmd event.Cmd
		m.pathInput, cmd = m.pathInput.Update(msg)
		fillAutocomplete(m)
		cmds = append(cmds, cmd)
	case searchMode:
		switch m.search.focus {
		case 0:
			var cmd event.Cmd
			m.search.filename, cmd = m.search.filename.Update(msg)
			cmds = append(cmds, cmd)
		case 1:
			var cmd event.Cmd
			m.search.text, cmd = m.search.text.Update(msg)
			cmds = append(cmds, cmd)
		}
	}

	return m, event.Batch(cmds...)
}
