package main

import (
	"strings"

	"charm.land/bubbles/v2/textinput"
)

type searchItem struct {
	name  string
	path  string
	isDir bool
}

type search struct {
	focus int
	name  textinput.Model
	text  textinput.Model

	cursor int
	start  int
	items  []searchItem
}

func newSearch(m *model) *search {
	name := newTextinput(m.theme)
	name.Placeholder = "filename"
	name.Focus()
	text := newTextinput(m.theme)
	text.Placeholder = "text"
	text.Blur()
	return &search{name: name, text: text}
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
	nameWidget := m.search.name.View()
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

	s.WriteString(base.Width(m.width).Render())
	s.WriteRune('\n')
	s.WriteString(base.Width(m.width).Render())
	return s.String()
}
