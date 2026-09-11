package main

import (
	"context"
	"fmt"
	"slices"
	"strings"

	"mc/internal/event"
)

type tab struct {
	readGeneration uint64
	gitGeneration  uint64
	pendingReads   int
	git            *gitInfo
	gitCancel      context.CancelFunc
	lastReadError  string
	sortMethod     sortMethod
	sortReverse    bool
	sorted         bool
	dir            string
	page           *page
	pageSettings   map[string]*pageSettings
	filterText     []string

	history        []string
	historyCurrent int
}

func (t *tab) set(dir string) bool {
	if dir == t.dir {
		return false
	}

	if t.historyCurrent < len(t.history)-1 {
		t.history = t.history[:t.historyCurrent+1]
	}

	t.history = append(t.history, dir)
	t.historyCurrent = len(t.history) - 1

	t.dir = dir
	t.page = &page{}
	t.filterText = nil

	return true
}

func (t *tab) back() string {
	if !t.hasPrev() {
		return t.dir
	}

	t.historyCurrent--
	t.dir = t.history[t.historyCurrent]

	t.page = &page{}
	t.filterText = nil

	return t.dir
}

func (t *tab) next() string {
	if !t.hasNext() {
		return t.dir
	}

	t.historyCurrent++
	t.dir = t.history[t.historyCurrent]

	t.page = &page{}
	t.filterText = nil

	return t.dir
}

func (t *tab) hasPrev() bool {
	return t.historyCurrent > 0
}

func (t *tab) hasNext() bool {
	return t.historyCurrent < len(t.history)-1
}

type page struct {
	selectionRange *rangeSelection
	items          []item
	tempItems      []item
}

type pageSettings struct {
	start  int
	cursor int
	sel    *string // it's for commands to select the item when finished
}

func (s *pageSettings) update(length int) {
	if s.cursor >= length {
		s.cursor = length - 1
	}
	if s.cursor < 0 {
		s.cursor = 0
	}
	if s.start >= length {
		s.start = length - 1
	}
	if s.start < 0 {
		s.start = 0
	}
}

func (t *tab) filter() {
	t.page.selectionRange = nil
	if t.filterText == nil {
		return
	}
	tempItems := make([]item, 0)
loop:
	for i := range t.page.items {
		for j := range t.filterText {
			if !strings.Contains(
				strings.ToUpper(t.page.items[i].getName()),
				strings.ToUpper(t.filterText[j]),
			) {
				continue loop
			}
		}
		tempItems = append(tempItems, t.page.items[i])
	}
	t.page.tempItems = tempItems
	settings := t.getPageSettings()
	settings.cursor = 0
	settings.start = 0
}

func newTab(dir string, page *page) *tab {
	return &tab{
		dir:          dir,
		page:         page,
		pageSettings: make(map[string]*pageSettings),
		history:      []string{dir},
	}
}

func (t *tab) getPageSettings() *pageSettings {
	settings, ok := t.pageSettings[t.dir]
	if !ok {
		settings := &pageSettings{}
		t.pageSettings[t.dir] = settings
		return settings
	}
	return settings
}

func (p *pane) hasTabs() bool {
	return len(p.tabs) > 0
}

// clampCurrent keeps currentTab inside the slice after a tab is removed. An
// empty pane parks it at 0, so a refill lands on a valid index; -1 would turn
// every getTab() into a panic instead of an empty check.
func (p *pane) clampCurrent() {
	if p.currentTab >= len(p.tabs) {
		p.currentTab = max(0, len(p.tabs)-1)
	}
}

// currentDir is the active tab's directory, or "" when the pane has no tabs.
// Callers that only want a path use this instead of getTab().dir.
func (m *model) currentDir() string {
	return m.paneDir(m.activePane)
}

func (m *model) paneDir(index int) string {
	p := m.panes[index]
	if !p.hasTabs() {
		return ""
	}
	return p.tabs[p.currentTab].dir
}

func (m *model) multipleTabs() bool {
	return len(m.tabs) > 1
}

func (m *model) getTabInfo() string {
	return fmt.Sprintf(" [%d/%d] ", m.currentTab+1, len(m.tabs))
}

func paneName(index int) string {
	if index == 0 {
		return "left"
	}
	return "right"
}

// sendTab moves or copies the current tab into the pane at dst. A move takes
// the tab and the focus with it, so the tab stays in front of the user. A copy
// opens the directory in the other pane and leaves the focus alone, which is
// what preparing a transfer needs.
func (m *model) sendTab(dst int, duplicate bool) (bool, event.Cmd) {
	if dst == m.activePane {
		return true, m.addMessage(msgWarning, fmt.Sprintf("already the %s pane", paneName(dst)))
	}
	src, target := m.panes[m.activePane], m.panes[dst]

	if !src.hasTabs() {
		return true, m.addMessage(msgWarning, "no tabs to send")
	}

	if duplicate {
		dir := src.tabs[src.currentTab].dir
		tabCopy := newTab(dir, &page{})
		target.tabs = append(target.tabs, tabCopy)
		target.currentTab = len(target.tabs) - 1
		return true, event.Batch(
			m.addMessage(msgInfo, fmt.Sprintf("tab copied to the %s pane", paneName(dst))),
			m.readTab(tabCopy),
		)
	}

	moved := src.tabs[src.currentTab]
	src.tabs = slices.Delete(src.tabs, src.currentTab, src.currentTab+1)
	src.clampCurrent()
	target.tabs = append(target.tabs, moved)
	target.currentTab = len(target.tabs) - 1

	m.activePane = dst
	m.pane = target
	m.mode = normalMode
	m.click = mouseClick{}
	return true, m.addMessage(msgInfo, fmt.Sprintf("tab moved to the %s pane", paneName(dst)))
}
