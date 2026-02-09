package main

import (
	"os"
	"path/filepath"
	"strings"
)

// DirExists checks if a path exists and is a directory
func dirExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

// PathExists checks if a path exists (file or directory)
func pathExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// IsFile checks if a path exists and is a file (not directory)
func isFile(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func numberOfDigits(n int) int {
	if n == 0 {
		return 1
	}
	if n < 0 {
		n = -n
	}

	count := 0
	for n > 0 {
		n /= 10
		count++
	}
	return count
}

func fillAutocomplete(m *model) {
	path := m.pathInput.Value()
	if strings.HasSuffix(path, ":") {
		path = path + "\\"
	}
	dir := filepath.Dir(path)
	if dir == m.pathInputDir {
		return
	} else {
		m.pathInputDir = dir
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		m.pathInput.ShowSuggestions = false
	}

	var suggestions []string
	for i := range entries {
		if entries[i].IsDir() {
			name := entries[i].Name()
			if !checkName(name) {
				continue
			}
			suggestions = append(suggestions, filepath.Join(dir, name))
		}
	}
	m.pathInput.ShowSuggestions = true

	m.pathInput.SetSuggestions(suggestions)
}

// true - ok
func checkName(name string) bool {
	lowerName := strings.ToLower(name)

	// Skip Windows/system files and folders
	switch lowerName {
	// System files
	case "thumbs.db":
		return false
	case "desktop.ini":
		return false
	case "dumpstack.log.tmp":
		return false

	// System folders (legacy and modern)
	case "$recycle.bin":
		return false
	case "system volume information":
		return false
	case "documents and settings": // XP legacy junction
		return false
	case "recovery": // Windows Recovery folder
		return false
	case "config.msi": // Windows Installer temp
		return false

	// Windows system files
	case "pagefile.sys":
		return false
	case "hiberfil.sys":
		return false
	case "swapfile.sys":
		return false
	case "bootmgr":
		return false
	case "bootnxt":
		return false
	}

	return true
}
