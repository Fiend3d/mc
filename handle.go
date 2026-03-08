package main

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
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
	items := m.getPage().getItems()
	if len(items) > 0 {
		item := items[0]
		switch item.(type) {
		case *driveItem:
			// TODO: implement changing labels
			return m, m.addMessage(msgError, "not supported yet")
		case *sharedItem:
			return m, m.addMessage(msgError, "can't rename these")
		case *filesystemItem:
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
		default:
			return m, m.addMessage(msgError, "uknown type of item")
		}
	}
	return m, m.addMessage(msgError, "nothing to rename")
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
	dir = strings.TrimSpace(dir)
	if isUNCRoot(dir) || dir == "" {
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

	disk := isDisk(dir)
	if disk {
		dir = strings.ToUpper(dir)
	}

	if !isUNC(dir) && !disk { // doesn't work with network disks
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
		return m, nil
	case tabsMode:
		if m.height-2 <= len(m.tabs) {
			m.tabsStart += steps
			actualHeight := m.height - 2
			m.tabsStart = max(0,
				min(m.tabsStart, len(m.tabs)-actualHeight))
		}
		return m, nil
	case bookmarksMode:
		if m.height-2 <= len(m.bm.dirs) {
			m.bm.start += steps
			actualHeight := m.height - 2
			m.bm.start = max(0,
				min(m.bm.start, len(m.bm.dirs)-actualHeight))
		}
		return m, nil
	case messagesMode:
		if m.height <= len(m.log) {
			m.logStart += steps
			m.logStart = max(0,
				min(m.logStart, len(m.log)-m.height))
		}
		return m, nil
	case helpMode:
		m.help += steps
		m.help = max(0, m.help)
		return m, nil
	case searchMode:
		m.search.start += steps
		actualHeight := m.height - 5
		m.search.start = max(0,
			min(m.search.start, m.search.length()-actualHeight))
	}
	return m, nil
}

type clipboardCopy int

const (
	clipboardCopyFilepath clipboardCopy = iota
	clipboardCopyDirectory
	clipboardCopyFilename
	clipboardCopyFilenameNoExt
	clipboardCopyFilepathArgs
	clipboardCopyFilenameArgs
	clipboardCopyFilepathArray
	clipboardCopyFilenameArray
)

func (m *model) handleClipboardCopy(action clipboardCopy, forward bool) (tea.Model, tea.Cmd) {
	switchMode := func() {
		if m.mode == copyVisualMode {
			m.mode = visualMode
		} else {
			m.mode = normalMode
		}
	}

	result := func(paths []string) (tea.Model, tea.Cmd) {
		if len(paths) == 1 {
			return m, m.addMessage(msgInfo, fmt.Sprintf(`"%s" copied`, paths[0]))
		} else {
			return m, m.addMessage(msgInfo, fmt.Sprintf("%d paths copied", len(paths)))
		}
	}

	const maxWidth = 40

	switch action {

	case clipboardCopyFilepath:
		paths := m.getPaths()
		switchMode()
		if len(paths) == 0 {
			return m, m.addMessage(msgWarning, "nothing to copy")
		}
		if forward {
			for i := range paths {
				paths[i] = strings.ReplaceAll(paths[i], "\\", "/")
			}
		}
		err := clipboardWrite(strings.Join(paths, "\n"))
		if err != nil {
			return m, m.addMessage(msgError, fmt.Sprintf("failed to set clipboard: %s", err))
		}
		return result(paths)

	case clipboardCopyDirectory:
		dir := m.getTab().dir
		if forward {
			dir = strings.ReplaceAll(dir, "\\", "/")
		}
		switchMode()
		err := clipboardWrite(dir)
		if err != nil {
			return m, m.addMessage(msgError, fmt.Sprintf("failed to set clipboard: %s", err))
		}
		return m, m.addMessage(msgInfo, fmt.Sprintf(`"%s" copied`, dir))

	case clipboardCopyFilename:
		paths := m.getPaths()
		switchMode()
		if len(paths) == 0 {
			return m, m.addMessage(msgWarning, "nothing to copy")
		}
		for i := range paths {
			paths[i] = filepath.Base(paths[i])
		}
		err := clipboardWrite(strings.Join(paths, "\n"))
		if err != nil {
			return m, m.addMessage(msgError, fmt.Sprintf("failed to set clipboard: %s", err))
		}
		return result(paths)

	case clipboardCopyFilenameNoExt:
		paths := m.getPaths()
		switchMode()
		if len(paths) == 0 {
			return m, m.addMessage(msgWarning, "nothing to copy")
		}
		for i := range paths {
			base := filepath.Base(paths[i])
			ext := filepath.Ext(base)
			name := base[:len(base)-len(ext)]
			paths[i] = name
		}
		err := clipboardWrite(strings.Join(paths, "\n"))
		if err != nil {
			return m, m.addMessage(msgError, fmt.Sprintf("failed to set clipboard: %s", err))
		}
		return result(paths)

	case clipboardCopyFilepathArgs:
		paths := m.getPaths()
		switchMode()
		if len(paths) == 0 {
			return m, m.addMessage(msgWarning, "nothing to copy")
		}
		for i := range paths {
			path := paths[i]
			if forward {
				path = strings.ReplaceAll(path, "\\", "/")
			}
			if strings.Contains(path, " ") {
				path = fmt.Sprintf(`"%s"`, path)
			}
			paths[i] = path
		}
		output := strings.Join(paths, " ")
		err := clipboardWrite(output)
		if err != nil {
			return m, m.addMessage(msgError, fmt.Sprintf("failed to set clipboard: %s", err))
		}
		return m, m.addMessage(msgInfo, fmt.Sprintf(`"%s" copied`, truncate(output, maxWidth)))

	case clipboardCopyFilenameArgs:
		paths := m.getPaths()
		switchMode()
		if len(paths) == 0 {
			return m, m.addMessage(msgWarning, "nothing to copy")
		}
		for i := range paths {
			name := filepath.Base(paths[i])
			if strings.Contains(name, " ") {
				name = fmt.Sprintf(`"%s"`, name)
			}
			paths[i] = name
		}
		output := strings.Join(paths, " ")
		err := clipboardWrite(output)
		if err != nil {
			return m, m.addMessage(msgError, fmt.Sprintf("failed to set clipboard: %s", err))
		}
		return m, m.addMessage(msgInfo, fmt.Sprintf(`"%s" copied`, truncate(output, maxWidth)))

	case clipboardCopyFilepathArray:
		paths := m.getPaths()
		switchMode()
		if len(paths) == 0 {
			return m, m.addMessage(msgWarning, "nothing to copy")
		}
		for i := range paths {
			path := paths[i]
			if forward {
				path = strings.ReplaceAll(path, "\\", "/")
			}
			path = strconv.Quote(path)
			paths[i] = path
		}
		output := strings.Join(paths, ", ")
		err := clipboardWrite(output)
		if err != nil {
			return m, m.addMessage(msgError, fmt.Sprintf("failed to set clipboard: %s", err))
		}
		return m, m.addMessage(msgInfo, fmt.Sprintf("[%s] copied", truncate(output, maxWidth)))

	case clipboardCopyFilenameArray:
		paths := m.getPaths()
		switchMode()
		if len(paths) == 0 {
			return m, m.addMessage(msgWarning, "nothing to copy")
		}
		for i := range paths {
			name := strconv.Quote(filepath.Base(paths[i]))
			paths[i] = name
		}
		output := strings.Join(paths, ", ")
		err := clipboardWrite(output)
		if err != nil {
			return m, m.addMessage(msgError, fmt.Sprintf("failed to set clipboard: %s", err))
		}
		return m, m.addMessage(msgInfo, fmt.Sprintf("[%s] copied", truncate(output, maxWidth)))
	}

	return m, m.addMessage(msgError, "lol?")
}

func (m *model) handleTool(key string) (tea.Model, tea.Cmd) {
	var t *ToolConfig
	switch key {
	case "f3":
		t = m.cfg.F3
	case "f4":
		t = m.cfg.F4
	case "f6":
		t = m.cfg.F6
	case "f7":
		t = m.cfg.F7
	case "f8":
		t = m.cfg.F8
	case "f9":
		t = m.cfg.F9
	case "f10":
		t = m.cfg.F10
	case "f11":
		t = m.cfg.F11
	case "f12":
		t = m.cfg.F12
	}

	if t == nil {
		return m, m.addMessage(msgWarning, "undefined tool")
	}
	var cmd *exec.Cmd
	switch t.Type {
	case "path":
		paths := m.getPaths()
		if len(paths) == 0 {
			return m, m.addMessage(msgWarning, "nothing is selected")
		}
		args := append(t.Args, paths...)
		cmd = exec.Command(t.Command, args...)
	case "dir":
		args := append(t.Args, m.getTab().dir)
		cmd = exec.Command(t.Command, args...)
	case "none": // why?
		cmd = exec.Command(t.Command, t.Args...)
	default:
		return m, m.addMessage(msgError, "unknown type of tool (not path/dir/none)")
	}
	cmd.Dir = m.getTab().dir
	return m, tea.ExecProcess(cmd, nil)
}
