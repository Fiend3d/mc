package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"golang.org/x/sys/windows"
)

func isUNC(path string) bool {
	p := filepath.ToSlash(path)
	return strings.HasPrefix(p, "//") &&
		len(p) > 2 &&
		p[2] != '/' // not three slashes (///) or more
}

func isUNCRoot(path string) bool {
	if !isUNC(path) {
		return false
	}
	if len(splitPath(path)) == 1 {
		return true
	}
	return false
}

func splitPath(path string) []string {
	return strings.FieldsFunc(path, func(r rune) bool {
		return r == '\\' || r == '/'
	})
}

func isDisk(path string) bool {
	pattern := `^[A-Za-z]:\\$`
	driveRegex := regexp.MustCompile(pattern)
	if driveRegex.MatchString(path) {
		return true
	}
	return false
}

func filepathDir(path string) string {
	if path == "" {
		return path
	}
	if isUNC(path) {
		parts := splitPath(path)
		if len(parts) > 1 {
			return `\\` + strings.Join(parts[:len(parts)-1], `\`)
		}
	}
	dir := filepath.Dir(path)
	if dir == path && isDisk(path) {
		return ""
	}
	return dir

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
	switch m.mode {
	case pathMode:
		path := m.pathInput.Value()
		if m.getTab().dir == "" {
			m.pathInput.ShowSuggestions = false // TODO: autocomplete drives maybe
			return
		}
		if isUNC(path) { // because network is slow T_T
			m.pathInput.ShowSuggestions = false
			return
		}
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
			return
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
	case shellMode:
		items := m.getPage().getItems()
		suggestions := make([]string, len(items)+1)
		cmd := m.input.Value()
		lastSpaceIndex := strings.LastIndex(cmd, " ")
		if lastSpaceIndex == -1 {
			for i := range items {
				suggestions[i] = items[i].getName()
			}
			suggestions[len(suggestions)-1] = "#sl"
			m.input.ShowSuggestions = true
			m.input.SetSuggestions(suggestions)
		} else {
			prefix := cmd[:lastSpaceIndex+1]
			for i := range items {
				suggestions[i] = prefix + items[i].getName()
			}
			suggestions[len(suggestions)-1] = prefix + "#sl"
			m.input.ShowSuggestions = true
			m.input.SetSuggestions(suggestions)
		}
	}
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

func expandWindowsEnv(path string) (string, error) {
	if strings.ContainsRune(path, '~') {
		home, err := os.UserHomeDir()
		if err == nil {
			path = strings.ReplaceAll(path, "~", home+"\\")
		}
	}
	src, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}

	// First call: get required size (includes null terminator)
	n, err := windows.ExpandEnvironmentStrings(src, nil, 0)
	if err != nil {
		return "", err
	}

	buf := make([]uint16, n)

	// Second call: expand into buffer
	_, err = windows.ExpandEnvironmentStrings(src, &buf[0], n)
	if err != nil {
		return "", err
	}

	// Trim trailing null
	return windows.UTF16ToString(buf[:n-1]), nil
}

func realWindowsPath(p string) (string, error) {
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}

	return filepath.EvalSymlinks(abs)
}
