package main

import (
	"path/filepath"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type mode int

const (
	normal mode = iota
	visual
	jump
	filter
	path
	bookmark
	bookmarkSelect
)

type submode int

const (
	noSubmode submode = iota
	goMode
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
	submode    submode
	width      int
	height     int

	pathInput   textinput.Model
	filterInput textinput.Model

	log   []string
	ticks int

	theme theme

	result string
}

type msgType int

const (
	msgTxt msgType = iota
	msgInfo
	msgWarning
	msgError
)

type tickMsg struct{}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m *model) addMessage(msgType msgType, msg string) (tea.Model, tea.Cmd) {
	var s strings.Builder
	style := &m.theme.emptyStyle
	switch msgType {
	case msgTxt:
		s.WriteString(style.Render(msg))
	case msgInfo:
		s.WriteString(style.Foreground(m.theme.accentColor3).Render("[info]"))
		s.WriteString(style.Render(msg))
	case msgWarning:
		s.WriteString(style.Foreground(m.theme.accentColor4).Render("[warning] "))
		s.WriteString(style.Render(msg))
	case msgError:
		s.WriteString(style.Foreground(m.theme.accentColor1).Render("[error] "))
		s.WriteString(style.Render(msg))
	}
	m.log = append(m.log, s.String())
	m.ticks = 6
	return m, tick()
}

func (m *model) left() (tea.Model, tea.Cmd) {
	tab := m.getTab()
	parent := filepath.Dir(tab.dir)
	tab.dir = parent
	_, exists := tab.pages[parent] // not gonna update anything
	if exists {
		return m, nil
	}
	tab.pages[parent] = &page{dir: parent}
	return m, m.readDir(parent)
}

func (m *model) right() (tea.Model, tea.Cmd) {
	tab := m.getTab()
	currentPage := tab.getPage()
	if currentPage.cursor > len(currentPage.items)-1 {
		return m, nil
	}
	selectedItem := currentPage.items[currentPage.cursor]
	if !selectedItem.isDir {
		return m, nil
	}
	dir := filepath.Join(tab.dir, selectedItem.name)
	tab.dir = dir
	_, exists := tab.pages[dir] // not gonna update
	if exists {
		return m, nil
	}
	tab.pages[dir] = newPage(dir)
	return m, m.readDir(dir)
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
	pathInput := newTextinput("", theme.emptyStyle, theme.grayColor)

	return model{
		tabs:        tabs,
		currentTab:  0,
		mode:        normal,
		theme:       theme,
		filterInput: filterInput,
		pathInput:   pathInput,
	}
}
