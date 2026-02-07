package main

import (
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/lipgloss"
)

type mode int

const (
	normal mode = iota
	visual
	jump
	filter
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
	visual int
	start  int
}

func newPage(dir string) *page {
	return &page{dir: dir}
}

func (p *page) getStartEnd() (int, int) {
	start := min(p.cursor, p.visual)
	end := max(p.cursor, p.visual)
	return start, end
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

	filterInput textinput.Model

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

func newTextinput(placeholder string, style lipgloss.Style, grayColor lipgloss.Color) textinput.Model {
	input := textinput.New()
	input.Placeholder = placeholder
	input.CharLimit = 256 // hello, windows!
	input.Width = 0
	input.PlaceholderStyle = style.Foreground(grayColor)
	input.TextStyle = style
	input.PromptStyle = style
	input.CompletionStyle = style
	input.Cursor.Style = style
	input.Cursor.TextStyle = style
	return input
}

func initialModel(dirs []string) model {
	tabs := make([]*tab, len(dirs))
	for i, dir := range dirs {
		pages := make(map[string]*page)
		pages[dir] = &page{dir: dir}
		tabs[i] = &tab{dir: dir, pages: pages}
	}

	theme := newTheme()
	filterInput := newTextinput("Enter text to filter", theme.emptyStyle, theme.grayColor)

	return model{
		tabs:        tabs,
		currentTab:  0,
		mode:        normal,
		theme:       theme,
		filterInput: filterInput,
	}
}
