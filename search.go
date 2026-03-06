package main

import (
	"fmt"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
)

type searchItem struct {
	path  string
	isDir bool
}

type search struct {
	focus    int
	filename textinput.Model
	text     textinput.Model

	working bool
	result  chan string
	done    chan bool

	cursor int
	start  int
	items  []searchItem
}

func newSearch(m *model) *search {
	filename := newTextinput(m.theme)
	filename.Placeholder = "filename"
	filename.Focus()
	text := newTextinput(m.theme)
	text.Placeholder = "text"
	text.Blur()
	return &search{filename: filename, text: text}
}

func renderSearchFocus(widget int, s *strings.Builder, m *model) {
	if m.search.focus == widget {
		s.WriteString(m.theme.emptyStyle.Foreground(m.theme.accentColor3).Render("┃"))
	} else {
		s.WriteString(m.theme.emptyStyle.Render(" "))
	}
}

func viewSearch(m *model) string {
	var s strings.Builder
	base := &m.theme.baseStyle
	empty := &m.theme.emptyStyle

	dir := colorizeDir(m.getTab().dir,
		empty.Bold(true).Foreground(m.theme.whiteColor),
		empty.Bold(true).Foreground(m.theme.accentColor5),
		m.width)
	dir = truncate(dir, m.width)
	s.WriteString(empty.Width(m.width).Render(dir))
	s.WriteRune('\n')
	renderSearchFocus(0, &s, m)
	nameWidget := m.search.filename.View()
	nameWidget = truncate(nameWidget, m.width-1)
	s.WriteString(empty.Width(m.width - 1).Render(nameWidget))
	s.WriteRune('\n')
	renderSearchFocus(1, &s, m)
	textWidget := m.search.text.View()
	textWidget = truncate(textWidget, m.width-1)
	s.WriteString(empty.Width(m.width - 1).Render(textWidget))
	s.WriteRune('\n')

	countItems := 0
	for i := countItems; i < m.height-5; i++ {
		renderSearchFocus(2, &s, m)
		s.WriteString(empty.Width(m.width - 1).Render(" "))
		s.WriteRune('\n')
	}

	modeColor := m.theme.whiteColor
	modeText := " SEARCH "
	if m.search.working {
		modeColor = m.theme.accentColor2
	}
	modeStyle := base.
		Background(modeColor).
		Foreground(m.theme.blackColor).
		Bold(true)

	s.WriteString(modeStyle.Render(modeText))

	if m.search.working {
		s.WriteString(base.Render(m.spinner.View()))
	} else {
		s.WriteString(base.Render("  "))
	}

	s.WriteString(base.Width(m.width - len(modeText) - 2).Render())
	s.WriteRune('\n')
	s.WriteString(empty.Width(m.width).Render(fmt.Sprintf("%d items", len(m.search.items))))
	return s.String()
}

const searchBufferSize = 1000

func (s *search) launch(dir string) {
	if s.done != nil {
		close(s.done)
	}
	s.working = true
	s.cursor = 0
	s.start = 0
	s.items = nil
	s.result = make(chan string, searchBufferSize)
	s.done = make(chan bool)
	go doSearch(dir, s.done, s.result)
}

func (s *search) stop() {
	close(s.done)
	s.working = false
}

func doSearch(dir string, done chan bool, result chan string) {
	for {
		time.Sleep(time.Microsecond * 10)
		select {
		case done, ok := <-done:
			if !ok || done {
				return
			}
		case result <- dir + " HUHEUHEUHUE":
		}
	}
}

type searchTickMsg struct{}

func searchTick() tea.Cmd {
	return tea.Tick(time.Microsecond*100, func(time.Time) tea.Msg {
		return searchTickMsg{}
	})
}
