package main

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"mc/internal/event"
	"mc/shutil"

	"github.com/dustin/go-humanize"
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

// changeDir points the active pane at dir. It is the shared refill path: an
// empty pane grows a tab here, so bookmarks, gc and path mode all work on one.
func (m *model) changeDir(dir string) event.Cmd {
	m.mode = normalMode
	if !m.hasTabs() {
		m.tabs = append(m.tabs, newTab(dir, &page{}))
		m.currentTab = len(m.tabs) - 1
		return m.readTab(m.getTab())
	}
	tab := m.getTab()
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

// calcSizeCommand is a read-only task: it never mutates the filesystem and so
// never reaches the undo history. It satisfies command purely so the task
// machinery (queueing, progress, cancellation, rendering) applies unchanged.
type calcSizeCommand struct {
	paths  []string
	target *tab
	page   *page // captured at submission so stale results can be discarded
	dir    string

	// results and total are written by the worker goroutine and read on the UI
	// goroutine only after the blocking taskDoneMsg send has happened.
	results []dirSize
	total   uint64
}

func newCalcSizeCommand(target *tab, paths []string) *calcSizeCommand {
	return &calcSizeCommand{paths: paths, target: target, page: target.page, dir: target.dir}
}

func (c *calcSizeCommand) undoable() bool { return false }
func (c *calcSizeCommand) undo() error    { return errors.New("calculate size cannot be undone") }
func (c *calcSizeCommand) getDir() string { return c.dir }
func (c *calcSizeCommand) sel() *string   { return nil }

func (c *calcSizeCommand) String() string {
	if len(c.paths) == 1 {
		return "calculate size " + filepath.Base(c.paths[0])
	}
	return fmt.Sprintf("calculate size (%d dirs)", len(c.paths))
}

// execute is the non-cancellable fallback; the task dispatch uses executeTask.
func (c *calcSizeCommand) execute() error { return c.executeTask(context.Background(), nil) }

// applyDirSizes writes finished measurements onto the originating page. It runs
// on the UI goroutine once the worker is done, and drops results whose tab has
// navigated elsewhere in the meantime.
func (m *model) applyDirSizes(c *calcSizeCommand) event.Cmd {
	if len(c.results) == 0 {
		return nil
	}
	if c.target != nil && c.target.page == c.page {
		items := c.target.page.getItems()
		sizes := make(map[string]uint64, len(c.results))
		for i := range c.results {
			sizes[c.results[i].path] = c.results[i].size
		}
		for j := range items {
			size, ok := sizes[items[j].getFullPath()]
			if !ok {
				continue
			}
			if item, ok := items[j].(*filepathItem); ok {
				item.size = size
				item.sizeStr = humanize.Bytes(size)
			}
		}
	}
	return m.addMessage(msgDone, fmt.Sprintf("total size: %s", humanize.Bytes(c.total)))
}

func (c *calcSizeCommand) executeTask(ctx context.Context, report shutil.ProgressFunc) error {
	c.results = make([]dirSize, 0, len(c.paths))
	c.total = 0

	// One readdir per selected root gives the progress bar a denominator without
	// a second full walk. Roots that cannot be read contribute no steps.
	steps := make([]int, len(c.paths))
	totalSteps := 0
	for i, path := range c.paths {
		if entries, err := os.ReadDir(path); err == nil {
			steps[i] = len(entries)
			totalSteps += len(entries)
		}
	}

	var baseBytes int64
	baseFiles, dirFiles, baseSteps := 0, 0, 0
	lastPath := ""
	for i, path := range c.paths {
		if err := ctx.Err(); err != nil {
			return err
		}
		dirFiles = 0
		size, err := shutil.CalcDirSize(ctx, path, func(p shutil.Progress) {
			// CalcDirSize counts from zero per directory, so carry the finished
			// ones forward to keep the running totals monotonic for the request.
			dirFiles = p.Files
			lastPath = p.Path
			p.Bytes += baseBytes
			p.Files += baseFiles
			p.Steps += baseSteps
			p.TotalSteps = totalSteps
			if report != nil {
				report(p)
			}
		})
		if err != nil {
			// A partial walk has an unusable total, so the directory is dropped
			// while the ones already finished are kept.
			return err
		}
		c.results = append(c.results, dirSize{path, size})
		c.total += size
		baseBytes += int64(size)
		baseFiles += dirFiles
		baseSteps += steps[i]
	}
	// A final unthrottled report so the completed row shows exact totals: the
	// throttle in spawn may have dropped the last in-flight update.
	if report != nil {
		// Carrying the last path forward keeps the Tasks view detail line
		// populated once the walk is done.
		report(shutil.Progress{
			Path: lastPath, Bytes: baseBytes, Files: baseFiles,
			Steps: totalSteps, TotalSteps: totalSteps, Scanning: true,
		})
	}
	return nil
}

type processDoneMsg struct {
	dir string
}

func runCmd(cmd *exec.Cmd, dir string) event.Cmd {
	return event.ExecProcess(cmd, func(err error) event.Msg {
		return processDoneMsg{dir: dir}
	})
}
