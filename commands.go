package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "charm.land/bubbletea/v2"
)

type errorMsg struct {
	err error
}

func newErr(err error) tea.Cmd {
	return func() tea.Msg {
		return errorMsg{err}
	}
}

type readDirMsg struct {
	tab   int
	items []item
	dir   string
}

func (m *model) update(dir string) tea.Cmd {
	var cmds []tea.Cmd

	for i := range m.tabs {
		tab := m.tabs[i]
		if tab.dir != dir {
			continue
		}

		// Create command for each page
		cmd := func(dir string, tab int) tea.Cmd {
			return func() tea.Msg {
				items, err := readItems(dir)
				if err != nil {
					return newErr(err)
				}
				return readDirMsg{
					tab:   tab,
					dir:   dir,
					items: items,
				}
			}
		}(tab.dir, i)

		cmds = append(cmds, cmd)
	}

	if len(cmds) == 0 {
		return nil
	}
	if len(cmds) == 1 { // is it faster, lol?
		return cmds[0]
	}
	return tea.Batch(cmds...)
}

func readItems(dir string) ([]item, error) {
	if dir == "" {
		drives, err := getDrives()
		if err != nil {
			return nil, err
		}
		result := make([]item, len(drives))
		for i := range drives {
			result[i] = newDriveItem(drives[i])
		}
		return result, nil
	}

	clipboardFiles, op, _ := getClipboardFiles() // not sure about handling this error

	if isUNCRoot(dir) {
		paths, err := netView(dir)
		if err != nil {
			return nil, err
		}
		result := make([]item, len(paths))
		for i := range paths {
			item := newSharedItem(clipboardFiles, op, paths[i], filepath.Join(dir, paths[i]))
			result[i] = item
		}
		return result, nil
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}

	filteredEntries := make([]os.DirEntry, 0, len(entries))
	for _, entry := range entries {
		if !checkName(entry.Name()) {
			continue
		}

		filteredEntries = append(filteredEntries, entry)
	}

	// Sort: directories first, then by name (case-insensitive)
	sort.Slice(filteredEntries, func(i, j int) bool {
		iIsDir := filteredEntries[i].IsDir()
		jIsDir := filteredEntries[j].IsDir()

		if iIsDir && !jIsDir {
			return true
		}
		if !iIsDir && jIsDir {
			return false
		}

		return strings.ToLower(filteredEntries[i].Name()) < strings.ToLower(filteredEntries[j].Name())
	})

	items := make([]item, 0, len(filteredEntries))
	for i := range filteredEntries {
		item, err := newFilepathItem(clipboardFiles, op, filteredEntries[i], dir)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}

	return items, nil
}

func (m *model) changeDir(dir string) tea.Cmd {
	tab := m.getTab()
	m.mode = normalMode
	if !tab.set(dir) {
		return nil
	}
	return m.readDir(m.currentTab, dir)
}

func (m *model) readDir(tab int, dir string) tea.Cmd {
	return func() tea.Msg {
		items, err := readItems(dir)
		if err != nil {
			return errorMsg{err}
		}

		return readDirMsg{tab: tab, items: items, dir: dir}
	}
}

type commandDoneMsg struct {
	message string
	dir     string
	sel     *string
	err     error
}

func (m model) execute(cmd command) tea.Cmd {
	return func() tea.Msg {
		err := m.cm.execute(cmd)
		return commandDoneMsg{fmt.Sprintf("%s", cmd), cmd.getDir(), cmd.sel(), err}
	}
}

func (m model) undo() tea.Cmd {
	return func() tea.Msg {
		cmd, err := m.cm.undo()
		return commandDoneMsg{fmt.Sprintf("undo: %s", cmd), cmd.getDir(), cmd.sel(), err}
	}
}

func (m model) redo() tea.Cmd {
	return func() tea.Msg {
		cmd, err := m.cm.redo()
		return commandDoneMsg{fmt.Sprintf("redo: %s", cmd), cmd.getDir(), cmd.sel(), err}
	}
}

type dirSize struct {
	path string
	size uint64
}

type calcDirSizeMsg struct {
	dir      string
	dirSizes []dirSize
	total    uint64
}

func calculateSize(dir string, paths []string) tea.Cmd {
	return func() tea.Msg {
		result := calcDirSizeMsg{dir: dir, dirSizes: make([]dirSize, 0, len(paths))}
		for i := range paths {
			size, err := calcDirSize(paths[i])
			if err == nil {
				result.dirSizes = append(result.dirSizes, dirSize{paths[i], size})
				result.total += size
			}
		}
		return result
	}
}
