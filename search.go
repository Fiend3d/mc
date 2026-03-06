package main

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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
	result  chan searchItem
	cancel  chan struct{}
	done    chan struct{}

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
	for i := range m.search.items {
		if i+1 > m.height-5 || i+m.search.start >= len(m.search.items) {
			break
		}

		index := i + m.search.start

		style := base
		cursorWidth := 3

		cursor := "   "

		if index == m.search.cursor {
			style = &m.theme.cursorStyle
			cursor = " > "
		}

		renderSearchFocus(2, &s, m)
		s.WriteString(style.Bold(true).Render(cursor))

		item := m.search.items[index]
		text := item.path
		text = truncate(text, m.width-cursorWidth-1)
		if item.isDir {
			s.WriteString(style.Foreground(m.theme.accentColor4).Width(m.width - cursorWidth - 1).Render(text))
		} else {
			if strings.HasSuffix(strings.ToUpper(item.path), ".EXE") {
				s.WriteString(style.Bold(true).Foreground(m.theme.accentColor3).Width(m.width - cursorWidth - 1).Render(text))
			} else {
				s.WriteString(style.Width(m.width - cursorWidth - 1).Render(text))
			}
		}
		s.WriteRune('\n')
		countItems++
	}

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

	rightText := fmt.Sprintf(" [%d/%d] ", m.search.cursor+1, len(m.search.items))

	s.WriteString(base.Width(m.width - len(modeText) - 2 - len(rightText)).Render())
	s.WriteString(base.Render(rightText))
	s.WriteRune('\n')
	if m.ticks > 0 {
		logMsg := m.log[len(m.log)-1].render(m.theme, false)
		if lipgloss.Width(logMsg) > m.width {
			logMsg = truncate(logMsg, m.width)
		}
		s.WriteString(empty.Width(m.width).Render(logMsg))
	} else {
		s.WriteString(empty.Width(m.width).Render())
	}
	return s.String()
}

const searchBufferSize = 1000

func (s *search) launch(dir string) {
	if s.cancel != nil {
		select {
		case _, ok := <-s.cancel:
			if ok {
				close(s.cancel)
			}
		default:
		}
	}
	s.working = true
	s.cursor = 0
	s.start = 0
	s.items = nil
	s.result = make(chan searchItem, searchBufferSize)
	s.cancel = make(chan struct{})
	s.done = make(chan struct{})
	pattern := s.filename.Value()
	text := s.text.Value()
	go doSearch(dir, pattern, text, s.cancel, s.done, s.result)
}

func (s *search) stop() {
	close(s.cancel)
	s.working = false
}

func searchSkip(name string) bool {
	switch name {
	case ".git":
		return true
	case ".github":
		return true
	}
	return false
}

func doSearch(
	dir string,
	pattern string,
	text string,
	cancel <-chan struct{},
	done chan<- struct{},
	result chan<- searchItem,
) {
	defer close(done)

	textSet := text != ""

	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}

		if path == dir {
			return nil
		}

		isDir := d.IsDir()

		name := d.Name()
		if isDir && searchSkip(name) {
			return filepath.SkipDir // it's kinda useless
		}

		if isDir && textSet {
			return nil
		}

		if pattern != "" {
			matched, err := filepath.Match(pattern, name)
			if err != nil || !matched {
				return nil
			}
		}

		select {
		case <-cancel:
			return filepath.SkipAll
		default:
		}

		if textSet && !isDir {
			info, err := d.Info()
			if err != nil {
				return nil
			}
			if info.Size() > 5_242_880 { // 5M ought to be enough for anybody
				return nil
			}
			contains, err := fileContainsText(path, text)
			if err != nil || !contains {
				return nil
			}
		}

		select {
		case result <- searchItem{path: path, isDir: isDir}:
		case <-cancel:
			return filepath.SkipAll
		}

		return nil
	})

	if err != nil {
		select {
		case result <- searchItem{path: "Error: " + err.Error()}:
		default:
		}
	}
}

func fileContainsText(path, text string) (bool, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	return strings.Contains(string(content), text), nil
}

type searchTickMsg struct{}

func searchTick() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(time.Time) tea.Msg {
		return searchTickMsg{}
	})
}
