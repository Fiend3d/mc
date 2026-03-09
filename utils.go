package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"sort"
	"strconv"
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
	if path == "" {
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

func realWindowsPath(path string) (string, error) {
	p, err := windows.UTF16PtrFromString(path)
	if err != nil {
		return "", err
	}

	handle, err := windows.CreateFile(
		p,
		0, // query only
		windows.FILE_SHARE_READ|
			windows.FILE_SHARE_WRITE|
			windows.FILE_SHARE_DELETE,
		nil,
		windows.OPEN_EXISTING,
		windows.FILE_FLAG_BACKUP_SEMANTICS,
		0,
	)
	if err != nil {
		return "", err
	}
	defer windows.CloseHandle(handle)

	// First call: get required size
	n, err := windows.GetFinalPathNameByHandle(handle, nil, 0, 0)
	if err != nil {
		return "", err
	}

	buf := make([]uint16, n)

	// Second call: get the path
	_, err = windows.GetFinalPathNameByHandle(handle, &buf[0], n, 0)
	if err != nil {
		return "", err
	}

	result := windows.UTF16ToString(buf)

	// Strip \\?\ prefix if present
	const prefix = `\\?\`
	if len(result) >= len(prefix) && result[:len(prefix)] == prefix {
		result = result[len(prefix):]
	}

	return result, nil
}

// uniquePath returns the next available numbered path
// Like Maya's naming: if test01, test02 exist, returns test03
func uniquePath(reserved []string, path string) string {
	dir := filepath.Dir(path)
	base := filepath.Base(path)
	ext := filepath.Ext(base)
	name := strings.TrimSuffix(base, ext)

	baseName, number, width, hasNumber := parseName(name)

	if !hasNumber {
		if !pathExists(path) && !slices.Contains(reserved, path) {
			return path
		}
	}

	existing := findExistingNumbers(reserved, dir, baseName, ext)

	used := map[int]struct{}{}
	for _, n := range existing {
		used[n] = struct{}{}
	}

	next := number
	if !hasNumber {
		next = 1
	}

	for {
		if _, ok := used[next]; !ok {
			candidate := filepath.Join(dir,
				fmt.Sprintf("%s%0*d%s", baseName, width, next, ext))

			if !pathExists(candidate) && !slices.Contains(reserved, candidate) {
				return candidate
			}
		}
		next++
	}
}

func parseName(name string) (baseName string, number int, width int, hasNumber bool) {
	re := regexp.MustCompile(`^(.*?)(\d+)$`)
	m := re.FindStringSubmatch(name)

	if len(m) == 3 {
		n, _ := strconv.Atoi(m[2])
		return m[1], n, len(m[2]), true
	}

	return name, 0, 1, false
}

func findExistingNumbers(reserved []string, dir, baseName, ext string) []int {
	var nums []int
	pattern := regexp.MustCompile(`^` + regexp.QuoteMeta(baseName) + `(\d+)` + regexp.QuoteMeta(ext) + `$`)

	entries, err := os.ReadDir(dir)
	if err != nil {
		return nums
	}

	for _, entry := range entries {
		matches := pattern.FindStringSubmatch(entry.Name())
		if len(matches) == 2 {
			num, _ := strconv.Atoi(matches[1])
			if num > 0 {
				nums = append(nums, num)
			}
		}
	}

	for _, path := range reserved {
		name := filepath.Base(path)
		matches := pattern.FindStringSubmatch(name)
		if len(matches) == 2 {
			num, _ := strconv.Atoi(matches[1])
			if num > 0 {
				nums = append(nums, num)
			}
		}
	}

	sort.Ints(nums)
	return nums
}
