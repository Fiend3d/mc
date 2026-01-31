package main

import (
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
			name := entry.Name()
			lowerName := strings.ToLower(name)

			// Skip Windows/system files and folders
			switch lowerName {
			// System files
			case "thumbs.db":
				continue
			case "desktop.ini":
				continue
			case "dumpstack.log.tmp":
				continue

			// System folders (legacy and modern)
			case "$recycle.bin":
				continue
			case "system volume information":
				continue
			case "documents and settings": // XP legacy junction
				continue
			case "recovery": // Windows Recovery folder
				continue
			case "config.msi": // Windows Installer temp
				continue

			// Windows system files
			case "pagefile.sys":
				continue
			case "hiberfil.sys":
				continue
			case "swapfile.sys":
				continue
			case "bootmgr":
				continue
			case "bootnxt":
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
