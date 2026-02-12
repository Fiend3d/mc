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

func (m *model) update(tab int) tea.Cmd {
	return func() tea.Msg {
		page := m.tabs[tab].getPage()
		entries, err := readEntries(page.dir)
		if err != nil {
			return newErr(err)
		}

		cursor := page.cursor
		if cursor >= len(entries) {
			cursor = len(entries) - 1
		}

		return updateDirMsg{dir: page.dir, entries: entries, cursor: cursor}
	}
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
	err     error
}

func (m model) newCommand(cmd command) tea.Cmd {
	return func() tea.Msg {
		err := m.cm.execute(cmd)
		return commandDoneMsg{fmt.Sprintf("%s", cmd), err}
	}
}
