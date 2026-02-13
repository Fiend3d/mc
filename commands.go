package main

import (
	"fmt"
	"os"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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
	tab     int
	entries []os.DirEntry
	dir     string
}

type updateDirMsg struct {
	tab     int
	entries []os.DirEntry
	dir     string
	cursor  int
}

func (m *model) update(dir string) tea.Cmd {
	var cmds []tea.Cmd

	for i := range m.tabs {
		tab := m.tabs[i]
		page, ok := tab.pages[dir]
		if !ok {
			continue
		}

		// Create command for each page
		cmd := func(dir string, tab int, cursor int) tea.Cmd {
			return func() tea.Msg {
				entries, err := readEntries(dir)
				if err != nil {
					return newErr(err)
				}
				if cursor >= len(entries) {
					cursor = len(entries) - 1
				}
				return updateDirMsg{
					tab:     tab,
					dir:     dir,
					entries: entries,
					cursor:  cursor,
				}
			}
		}(page.dir, i, page.cursor)

		cmds = append(cmds, cmd)
	}

	if len(cmds) == 0 {
		return nil
	}
	if len(cmds) == 1 {
		return cmds[0]
	}
	return tea.Batch(cmds...)
}

func readEntries(dir string) ([]os.DirEntry, error) {
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

	return filteredEntries, nil
}

func (m model) readDir(tab int, dir string) tea.Cmd {
	return func() tea.Msg {
		entries, err := readEntries(dir)
		if err != nil {
			return errorMsg{err}
		}

		return readDirMsg{tab: tab, entries: entries, dir: dir}
	}
}

type commandDoneMsg struct {
	message string
	dir     string
	err     error
}

func (m model) execute(cmd command, dir string) tea.Cmd {
	return func() tea.Msg {
		err := m.cm.execute(cmd)
		return commandDoneMsg{fmt.Sprintf("%s", cmd), dir, err}
	}
}

func (m model) undo() tea.Cmd {
	return func() tea.Msg {
		cmd, err := m.cm.undo()
		return commandDoneMsg{fmt.Sprintf("undo: %s", cmd), cmd.getDir(), err}
	}
}
