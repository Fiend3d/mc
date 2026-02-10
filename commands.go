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

type readDirMsg struct {
	entries []os.DirEntry
	dir     string
}

func newErr(err error) tea.Cmd {
	return func() tea.Msg {
		return errorMsg{err}
	}
}

func (m model) readDir(dir string) tea.Cmd {
	return func() tea.Msg {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return errorMsg{err}
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

		return readDirMsg{entries: filteredEntries, dir: dir}
	}
}

type commandDoneMsg struct {
	message string
}

func (m model) newCommandr(cmd command) tea.Cmd {
	return func() tea.Msg {
		m.cm.execute(cmd)
		return commandDoneMsg{fmt.Sprintf("%s", cmd)}
	}
}
