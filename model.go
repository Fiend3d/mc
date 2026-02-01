package main

type mode int

const (
	normal mode = iota
	visual
	shell
)

type tab struct {
	dir string
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
	if p.cursor < 0 {
		p.cursor = 0
	} else if p.cursor > p.length()-1 {
		p.cursor = p.length() - 1
	}
	p.updateStart(height)
}

type model struct {
	err        error
	tabs       []*tab
	currentTab int
	pages      map[string]*page
	mode       mode
	width      int
	height     int

	theme theme
}

func (m *model) getPage() *page {
	dir := m.tabs[m.currentTab].dir
	return m.pages[dir]
}

func initialModel(dirs []string) model {
	pages := make(map[string]*page)
	tabs := make([]*tab, len(dirs))
	for i, dir := range dirs {
		pages[dir] = &page{dir: dir}
		tabs[i] = &tab{dir: dir}
	}

	return model{
		tabs:       tabs,
		currentTab: 0,
		pages:      pages,
		mode:       normal,
		theme:      newTheme(),
	}
}
