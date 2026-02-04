package main

type mode int

const (
	normal mode = iota
	visual
	jump
	shell
)

type tab struct {
	dir   string
	pages map[string]*page
}

func (t *tab) getPage() *page {
	return t.pages[t.dir]
}

type page struct {
	dir    string
	items  []*item
	cursor int
	start  int
}

func (p *page) length() int {
	return len(p.items)
}

func (p *page) updateStart(height int) {
	if p.cursor < p.start {
		p.start = p.cursor
		return
	}
	actualHeight := height - 4
	if p.cursor > p.start+actualHeight {
		p.start = p.cursor - actualHeight
	}
}

func (p *page) moveCursor(move, height int) {
	p.cursor += move
	if p.cursor > p.length()-1 {
		p.cursor = p.length() - 1
	}
	if p.cursor < 0 {
		p.cursor = 0
	}
	p.updateStart(height)
}

type model struct {
	err        error
	tabs       []*tab
	currentTab int
	mode       mode
	width      int
	height     int

	theme theme

	result string
}

func (m *model) getTab() *tab {
	return m.tabs[m.currentTab]
}

func (m *model) getPage() *page { // probably redundant
	tab := m.getTab()
	return tab.pages[tab.dir]
}

func initialModel(dirs []string) model {
	tabs := make([]*tab, len(dirs))
	for i, dir := range dirs {
		pages := make(map[string]*page)
		pages[dir] = &page{dir: dir}
		tabs[i] = &tab{dir: dir, pages: pages}
	}

	return model{
		tabs:       tabs,
		currentTab: 0,
		mode:       normal,
		theme:      newTheme(),
	}
}
