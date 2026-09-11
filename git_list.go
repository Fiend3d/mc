package main

import (
	"fmt"
	"path/filepath"

	"mc/internal/event"
)

// gitList is the overlay behind a tally: the files that tally counted, held as
// a snapshot. A status read in the background must not shuffle rows under the
// cursor, so the list keeps its own copy until it is opened again.
type gitList struct {
	tally   gitState
	root    string
	branch  string
	entries []gitEntry
	cursor  int
	start   int
}

// openGitList shows the files behind one tally of the active tab's repository.
// An empty tally says so instead of opening a list with nothing in it.
func (m *model) openGitList(tally gitState) (event.Model, event.Cmd) {
	if !m.hasTabs() {
		return m, nil
	}
	info := m.getTab().git
	if info == nil {
		return m, m.addMessage(msgWarning, "not a git repository")
	}
	entries := info.changed(tally)
	if len(entries) == 0 {
		return m, m.addMessage(msgInfo, fmt.Sprintf("no %s files", tallyName(tally)))
	}
	m.gitList = gitList{tally: tally, root: info.root, branch: info.branch, entries: entries}
	m.mode = gitListMode
	return m, nil
}

// current is the entry under the cursor, or nil once the list is empty.
func (l *gitList) current() *gitEntry {
	if l.cursor < 0 || l.cursor >= len(l.entries) {
		return nil
	}
	return &l.entries[l.cursor]
}

// move walks the cursor and keeps the viewport around it.
func (l *gitList) move(delta, height int) {
	l.cursor = min(max(0, l.cursor+delta), max(0, len(l.entries)-1))
	l.keepCursor(height)
}

func (l *gitList) keepCursor(height int) {
	rows := max(1, height-2)
	if l.cursor < l.start {
		l.start = l.cursor
	}
	if l.cursor >= l.start+rows {
		l.start = l.cursor - rows + 1
	}
	l.start = max(0, min(l.start, max(0, len(l.entries)-rows)))
}

// gitListJump leaves the list for the file under the cursor: the pane goes to
// its directory and the cursor lands on it, as a search result does.
func (m *model) gitListJump() (event.Model, event.Cmd) {
	entry := m.gitList.current()
	if entry == nil {
		return m, nil
	}
	m.mode = normalMode
	return m, event.Sequence(m.changeDir(filepath.Dir(entry.path)), selectItem(m.getTab(), entry.path))
}

// handleGitList is the list's keyboard. F2 to F12 fall through to handleTool,
// which reads the highlighted entry out of getPaths.
func (m *model) handleGitList(key string) (event.Model, event.Cmd) {
	switch key {
	case "esc", "q":
		m.mode = normalMode
		return m, nil
	case "j", "down":
		m.gitList.move(1, m.height)
		return m, nil
	case "k", "up":
		m.gitList.move(-1, m.height)
		return m, nil
	case "pgdown":
		m.gitList.move((m.height-2)/2, m.height)
		return m, nil
	case "pgup":
		m.gitList.move(-(m.height-2)/2, m.height)
		return m, nil
	case "home":
		m.gitList.move(-len(m.gitList.entries), m.height)
		return m, nil
	case "end":
		m.gitList.move(len(m.gitList.entries), m.height)
		return m, nil
	case "enter", "l", "right":
		return m.gitListJump()
	case "f2", "f3", "f4", "f6", "f7", "f8", "f9", "f10", "f11", "f12":
		return m.handleTool(key)
	}
	return m, nil
}
