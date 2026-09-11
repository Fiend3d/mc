package main

import (
	"path/filepath"
	"time"

	"github.com/fsnotify/fsnotify"
	"mc/internal/event"
)

// Keep the first deadline in each burst, then schedule a new batch for changes
// arriving during a read. No trailing event is discarded and busy tabs wait.
type refreshQueue map[string]time.Time

func (q refreshQueue) changed(change fsnotify.Event, watched map[string]bool, now time.Time) {
	for dir := range watched {
		if samePath(change.Name, dir) || samePath(filepath.Dir(change.Name), dir) {
			if _, pending := q[dir]; !pending {
				q[dir] = now.Add(100 * time.Millisecond)
			}
		}
	}
}

func (q refreshQueue) drain(m *model, now time.Time) []event.Cmd {
	var cmds []event.Cmd
	for dir, due := range q {
		if now.Before(due) {
			continue
		}
		busy := false
		for _, p := range m.panes {
			for _, t := range p.tabs {
				if samePath(t.dir, dir) && t.pendingReads > 0 {
					busy = true
				}
			}
		}
		if busy {
			continue
		}
		delete(q, dir)
		for _, p := range m.panes {
			for _, t := range p.tabs {
				if samePath(t.dir, dir) {
					cmds = append(cmds, m.readTabMode(t, true))
				}
			}
		}
	}
	return cmds
}

// liveTab reports whether an async result still belongs to the tab it was
// started for: the tab is still held by a pane, and has neither navigated nor
// been handed a new page since the work began.
func (m *model) liveTab(t *tab, dir string, page *page) bool {
	for _, p := range m.panes {
		for _, candidate := range p.tabs {
			if candidate == t {
				return t.dir == dir && t.page == page
			}
		}
	}
	return false
}

// Compare raw metadata independently of sorting, selections and calculated
// directory sizes, so periodic safety reads do not disrupt an active range.
func unchangedListing(old, fresh []item) bool {
	if old == nil || len(old) != len(fresh) {
		return false
	}
	byPath := make(map[string]item, len(old))
	for _, it := range old {
		byPath[it.getFullPath()] = it
	}
	for _, it := range fresh {
		switch b := it.(type) {
		case *filepathItem:
			a, ok := byPath[it.getFullPath()].(*filepathItem)
			if !ok {
				return false
			}
			x, y := *a, *b
			x.selected, y.selected = false, false
			x.git, y.git = gitNone, gitNone
			if x.isDir && y.isDir {
				x.size, y.size, x.sizeStr, y.sizeStr = 0, 0, "", ""
			}
			if x != y {
				return false
			}
		case *driveItem:
			a, ok := byPath[it.getFullPath()].(*driveItem)
			if !ok {
				return false
			}
			x, y := *a, *b
			x.selected, y.selected = false, false
			if x != y {
				return false
			}
		case *sharedItem:
			a, ok := byPath[it.getFullPath()].(*sharedItem)
			if !ok {
				return false
			}
			x, y := *a, *b
			x.selected, y.selected = false, false
			if x != y {
				return false
			}
		default:
			return false
		}
	}
	return true
}
