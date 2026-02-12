package main

import (
	"fmt"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"path/filepath"
	"strings"
	"time"
)

type mode int

const (
	normal mode = iota
	visual
	jump
	messages
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

type msgType int

type message struct {
	time        time.Time
	messageType msgType
	message     string
}

func (m *message) render(theme *theme, renderTime bool) string {
	var s strings.Builder
	if renderTime {
		timeStyle := theme.emptyStyle.Foreground(theme.grayColor)
		s.WriteString(timeStyle.Render(m.time.Format("02.01.2006 15:04")))
		s.WriteString(timeStyle.Render(" "))
	}
	style := &theme.emptyStyle
	switch m.messageType {
	case msgTxt:
		s.WriteString(style.Render(m.message))
	case msgInfo:
		s.WriteString(style.Foreground(theme.accentColor3).Render("[info] "))
		s.WriteString(style.Render(m.message))
	case msgWarning:
		s.WriteString(style.Foreground(theme.accentColor4).Render("[warning] "))
		s.WriteString(style.Render(m.message))
	case msgError:
		s.WriteString(style.Foreground(theme.accentColor1).Render("[error] "))
		s.WriteString(style.Render(m.message))
	case msgDone:
		s.WriteString(style.Foreground(theme.greenColor).Render("[done] "))
		s.WriteString(style.Render(m.message))
	case msgFail:
		s.WriteString(style.Foreground(theme.redColor).Render("[fail] "))
		s.WriteString(style.Render(m.message))

	}
	return s.String()
}

type action int

const (
	copyAction action = iota
	cutAction
)

func (m *model) doAction(action action, txt string) tea.Cmd {
	page := m.getPage()
	var paths []string
	switch m.mode {
	case visual:
		start, end := page.getStartEnd()
		for i := start; i <= end; i++ {
			paths = append(paths, page.items[i].fullPath)
		}
		m.mode = normal
	default:
		for i := range page.items {
			if page.items[i].selected {
				paths = append(paths, page.items[i].fullPath)
			}
		}
		if len(paths) == 0 {
			paths = append(paths, page.items[page.cursor].fullPath)
		}
	}
	switch action {
	case copyAction:
		setClipboardFiles(paths, OpCopy)
	case cutAction:
		setClipboardFiles(paths, OpCut)
	}
	return m.addMessage(msgInfo, fmt.Sprintf("%d paths %s", len(paths), txt))
}

type model struct {
	err        error
	tabs       []*tab
	currentTab int
	mode       mode
	submode    submode
	width      int
	height     int

	jobs    int
	spinner spinner.Model

	cm *commandManager

	pathInput    textinput.Model
	pathInputDir string // to optimize autocomplete
	filterInput  textinput.Model

	log      []message
	logStart int
	ticks    int

	theme theme

	result string
}

const (
	msgTxt msgType = iota
	msgInfo
	msgWarning
	msgError
	msgDone
	msgFail
)

type tickMsg struct{}

func tick() tea.Cmd {
	return tea.Tick(time.Second, func(time.Time) tea.Msg {
		return tickMsg{}
	})
}

func (m *model) addMessage(msgType msgType, msg string) tea.Cmd {
	message := message{time: time.Now(), messageType: msgType, message: msg}
	m.log = append(m.log, message)
	m.ticks = 6
	return tick()
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
	input.CompletionStyle = style.Foreground(grayColor)
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
	s := spinner.New()
	s.Spinner = spinner.Dot
	s.Style = theme.baseStyle.Foreground(theme.accentColor1)

	return model{
		tabs:        tabs,
		currentTab:  0,
		mode:        normal,
		theme:       theme,
		filterInput: filterInput,
		pathInput:   pathInput,
		spinner:     s,
		cm:          newCommandManager(),
	}
}
