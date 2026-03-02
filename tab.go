package main

import "strings"

type tab struct {
	dir          string
	page         *page
	pageSettings map[string]*pageSettings
	filterText   []string

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
	items     []item
	tempItems []item
}

type pageSettings struct {
	start  int
	cursor int
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
