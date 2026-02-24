package main

import (
	"fmt"
	"path/filepath"
	"strings"

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
		return m, m.addMessage(msgError, "only one path is supported at the moment")
	}
	m.mode = renameMode
	m.renamePaths = paths
	m.input.Placeholder = ""
	m.input.Reset()
	m.input.Focus()
	m.input.SetValue(filepath.Base(paths[0]))
	return m, textinput.Blink
}

func (m *model) handleRestoreTab() (tea.Model, tea.Cmd) {
	if len(m.closedTabs) == 0 {
		return m, m.addMessage(msgWarning, "nothing to restore")
	}
	dir := m.closedTabs[len(m.closedTabs)-1]
	m.closedTabs = m.closedTabs[:len(m.closedTabs)-1]
	m.tabs = append(m.tabs, newTab(dir, &page{}))
	m.currentTab = len(m.tabs) - 1
	return m, m.readDir(m.currentTab, dir)
}

func (m *model) handleNewPath(addTab bool) (tea.Model, tea.Cmd) {
	dir := m.pathInput.Value()
	if strings.TrimSpace(dir) == "" {
		return m, m.addMessage(msgError, "empty path")
	}
	if isUNCroot(dir) {
		if addTab {
			m.tabs = append(m.tabs, newTab(m.getTab().dir, &page{}))
			m.currentTab = len(m.tabs) - 1
		}
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
	if addTab {
		m.tabs = append(m.tabs, newTab(m.getTab().dir, &page{}))
		m.currentTab = len(m.tabs) - 1
	}
	return m.changeDir(dir)
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
	case tabsMode:
		if m.height-2 <= len(m.tabs) {
			m.tabsStart += steps
			actualHeight := m.height - 2
			m.tabsStart = max(0,
				min(m.tabsStart, len(m.tabs)-actualHeight))
		}
	case messagesMode:
		if m.height <= len(m.log) {
			m.logStart += steps
			m.logStart = max(0,
				min(m.logStart, len(m.log)-m.height))
		}
	}
	return m, nil
}
