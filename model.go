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

func (m *model) cursorDown() {
	page := m.getPage()
	if len(page.items)-2 >= page.cursor {
		page.cursor += 1
	}
}

func (m *model) cursorUp() {
	page := m.getPage()
	if page.cursor > 0 {
		page.cursor -= 1
	}
}

func initialModel(dir string) model {
	pages := make(map[string]*page)
	pages[dir] = &page{dir: dir}

	return model{
		tabs: []*tab{
			{dir: dir},
		},
		currentTab: 0,
		pages:      pages,
		mode:       normal,
		theme:      newTheme(),
	}
}
