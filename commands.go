package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"mc/internal/event"
	"mc/shutil"
)

type errorMsg struct {
	err error
}

type readDirMsg struct {
	target     *tab
	generation uint64
	page       *page
	items      []item
	dir        string
	err        error
	automatic  bool
}

func (m *model) update(dir string) event.Cmd {
	var cmds []event.Cmd
	for _, p := range m.panes {
		for _, t := range p.tabs {
			if t.dir == dir {
				cmds = append(cmds, m.readTab(t))
			}
		}
	}
	return event.Batch(cmds...)
}
func (m *model) readTab(t *tab) event.Cmd {
	return m.readTabMode(t, false)
}

func (m *model) readTabMode(t *tab, automatic bool) event.Cmd {
	if automatic && t.pendingReads > 0 {
		return nil
	}
	t.pendingReads++
	t.readGeneration++
	generation := t.readGeneration
	dir, page := t.dir, t.page
	return func() event.Msg {
		items, err := readItems(dir)
		return readDirMsg{target: t, generation: generation, page: page, items: items, dir: dir, err: err, automatic: automatic}
	}
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

	clipboardFiles, op, _ := getClipboardFilesCached() // not sure about handling this error

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

func (m *model) changeDir(dir string) event.Cmd {
	tab := m.getTab()
	m.mode = normalMode
	if !tab.set(dir) {
		return nil
	}
	return m.readDir(m.currentTab, dir)
}

func (m *model) readDir(index int, dir string) event.Cmd { return m.readTab(m.tabs[index]) }

func (m *model) execute(cmd command) event.Cmd { return m.enqueue(cmd, "execute") }
func (m *model) runUndo(cmd command) event.Cmd { return m.enqueue(cmd, "undo") }
func (m *model) runRedo(cmd command) event.Cmd { return m.enqueue(cmd, "redo") }

type dirSize struct {
	path string
	size uint64
}

type calcDirSizeMsg struct {
	target   *tab
	page     *page
	dirSizes []dirSize
	total    uint64
}

func calculateSize(target *tab, paths []string) event.Cmd {
	page := target.page
	return func() event.Msg {
		result := calcDirSizeMsg{target: target, page: page, dirSizes: make([]dirSize, 0, len(paths))}
		for i := range paths {
			size, err := shutil.CalcDirSize(paths[i])
			if err == nil {
				result.dirSizes = append(result.dirSizes, dirSize{paths[i], size})
				result.total += size
			}
		}
		return result
	}
}

type processDoneMsg struct {
	dir string
}

func runCmd(cmd *exec.Cmd, dir string) event.Cmd {
	return event.ExecProcess(cmd, func(err error) event.Msg {
		return processDoneMsg{dir: dir}
	})
}
