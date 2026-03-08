package main

import (
	"bufio"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strconv"
	"strings"
	"time"

	"charm.land/bubbles/v2/textinput"
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

type searchLine struct {
	line       string
	lineNumber int
	start      int
	end        int
}

type searchItem struct {
	path  string
	isDir bool
	lines []searchLine
}

type search struct {
	focus    int
	filename textinput.Model
	text     textinput.Model

	working bool
	result  chan searchItem
	cancel  chan struct{}
	done    chan struct{}

	cursor    int
	start     int
	showLines bool
	items     []searchItem
}

func (m *model) launchSearch() (tea.Model, tea.Cmd) {
	dir := m.getTab().dir
	m.search.launch(dir)
	return m, tea.Batch(
		m.spinner.Tick,
		searchTick(),
		m.addMessage(msgInfo, fmt.Sprintf("searching: %s", dir)),
	)
}

func (s *search) length() int {
	if !s.showLines {
		return len(s.items)
	}

	result := 0
	for i := range s.items {
		result++ // file
		result += len(s.items[i].lines)
	}
	return result
}

func (s *search) isItem(index int) bool {
	if !s.showLines {
		return true
	}

	current := 0
	for i := range s.items {
		if current == index {
			return true
		}
		current++

		for range s.items[i].lines {
			if current == index {
				return false
			}
			current++
		}
	}
	return false
}

func (s *search) mapIndex(index int) (int, int) {
	if !s.showLines {
		return index, -1
	}

	current := 0
	for i := range s.items {
		if current == index {
			return i, -1 // file row
		}
		current++

		for j := range s.items[i].lines {
			if current == index {
				return i, j // line row
			}
			current++
		}
	}

	return -1, -1
}

func (s *search) blink() tea.Cmd {
	switch s.focus {
	case 0, 1:
		return textinput.Blink
	}
	return nil
}

func (s *search) setFocus(focus int) {
	switch focus {
	case 0:
		if s.focus == 1 {
			s.text.Blur()
		}
		s.filename.Focus()
	case 1:
		if s.focus == 0 {
			s.filename.Blur()
		}
		s.text.Focus()
	case 2:
		switch s.focus {
		case 0:
			s.filename.Blur()
		case 1:
			s.text.Blur()
		}
	}
	s.focus = focus
}

func (s *search) moveCursor(move int, height int) {
	s.cursor += move
	length := s.length()
	if s.cursor >= length {
		s.cursor = length - 1
	}
	if s.cursor < 0 {
		s.cursor = 0
	}
	s.updateStart(height)
}

func (s *search) updateStart(height int) {
	if s.cursor < s.start {
		s.start = s.cursor
		return
	}
	actualHeight := height - 6
	if s.cursor > s.start+actualHeight {
		s.start = s.cursor - actualHeight
	}
}

func newSearch(m *model) *search {
	filename := newTextinput(m.theme)
	filename.Placeholder = "filename"
	filename.Focus()
	text := newTextinput(m.theme)
	text.Placeholder = "text"
	text.Blur()
	return &search{filename: filename, text: text, showLines: true}
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
	for i := range m.search.length() {
		if i+1 > m.height-5 || i+m.search.start >= m.search.length() {
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

		if m.search.isItem(index) {
			actualIndex, _ := m.search.mapIndex(index)
			item := m.search.items[actualIndex]
			text := item.path
			suffix := ""
			if !m.search.showLines {
				if len(item.lines) > 0 {
					suffix = fmt.Sprintf(" [%d]", len(item.lines))
				}
			}
			textBlock := ""
			if item.isDir {
				textBlock += style.Foreground(m.theme.accentColor4).Render(text)
			} else {
				if strings.HasSuffix(strings.ToUpper(item.path), ".EXE") {
					textBlock += style.Bold(true).Foreground(m.theme.greenColor).Render(text)
				} else {
					if m.search.showLines && len(item.lines) > 0 {
						textBlock += style.Foreground(m.theme.accentColor2).Render(text)
					} else {
						textBlock += style.Render(text)
					}
				}
			}
			if suffix != "" {
				textBlock += style.Foreground(m.theme.greenColor).Render(suffix)
			}
			textBlock = truncate(textBlock, m.width-cursorWidth-1)
			s.WriteString(style.Width(m.width - cursorWidth - 1).Render(textBlock))

		} else {
			actualIndex, lineIndex := m.search.mapIndex(index)
			item := m.search.items[actualIndex]
			line := item.lines[lineIndex]
			if lineIndex != len(item.lines)-1 {
				s.WriteString(style.Foreground(m.theme.grayColor).Render("├─"))
			} else {
				s.WriteString(style.Foreground(m.theme.grayColor).Render("└─"))
			}
			lineNumber := strconv.Itoa(line.lineNumber)
			s.WriteString(style.Foreground(m.theme.greenColor).Render(lineNumber))
			s.WriteString(style.Render(":"))
			lineLength := 7 + len(lineNumber)
			token1 := line.line[:line.start]
			token2 := line.line[line.start:line.end]
			token3 := line.line[line.end:]
			tokens := style.Render(token1)
			tokens += style.Foreground(m.theme.redColor).Render(token2)
			tokens += style.Render(token3)
			tokens = truncate(tokens, m.width-lineLength)
			s.WriteString(style.Width(m.width - lineLength).Render(tokens))
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

	helpText := base.Foreground(m.theme.grayColor).Render("Keys: ")
	helpText += base.Render("F5")
	helpText += base.Foreground(m.theme.grayColor).Render(" - search ")
	if m.search.focus == 2 {
		helpText += base.Render("h")
		helpText += base.Foreground(m.theme.grayColor).Render(" - hide lines ")
	}

	rightText := fmt.Sprintf(" [%d/%d] ", m.search.cursor+1, m.search.length())
	helpText = truncate(helpText, m.width-len(modeText)-2-len(rightText))
	s.WriteString(base.Width(m.width - len(modeText) - 2 - len(rightText)).Render(helpText))
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

func checkDirSkip(name string) bool {
	switch name {
	case ".git":
		return false
	case ".github":
		return false
	}
	return true
}

type walkItem struct {
	path string
	info fs.FileInfo
}

func readLines(path string) func(func(string) bool) {
	return func(yield func(string) bool) {
		f, err := os.Open(path)
		if err != nil {
			return
		}
		defer f.Close()

		scanner := bufio.NewScanner(f)

		for scanner.Scan() {
			if !yield(scanner.Text()) {
				return
			}
		}
	}
}

func walkDir(root string) func(func(walkItem) bool) {
	return func(yield func(walkItem) bool) {

		type rule struct {
			pattern  string
			negate   bool
			dirOnly  bool
			anchored bool
		}

		var walk func(string, string, []rule) bool

		walk = func(dir, rel string, rules []rule) bool {

			entries, err := os.ReadDir(dir)
			if err != nil {
				return true
			}

			localRules := append([]rule{}, rules...)

			// Load .gitignore
			for _, e := range entries {
				if e.IsDir() || e.Name() != ".gitignore" {
					continue
				}

				for line := range readLines(filepath.Join(dir, ".gitignore")) {

					line = strings.TrimSpace(line)
					if line == "" || strings.HasPrefix(line, "#") {
						continue
					}

					r := rule{}

					if strings.HasPrefix(line, "!") {
						r.negate = true
						line = line[1:]
					}

					if strings.HasSuffix(line, "/") {
						r.dirOnly = true
						line = strings.TrimSuffix(line, "/")
					}

					if strings.HasPrefix(line, "/") {
						r.anchored = true
						line = line[1:]
					}

					r.pattern = filepath.ToSlash(line)

					localRules = append(localRules, r)
				}
			}

			sort.Slice(entries, func(i, j int) bool {
				a := entries[i]
				b := entries[j]

				if a.IsDir() != b.IsDir() {
					return !a.IsDir()
				}

				return strings.ToLower(a.Name()) < strings.ToLower(b.Name())
			})

			for _, e := range entries {

				name := e.Name()
				path := filepath.Join(dir, name)
				relPath := filepath.ToSlash(filepath.Join(rel, name))

				ignored := false

				for _, r := range localRules {

					target := relPath
					if !strings.Contains(r.pattern, "/") {
						target = name
					}

					match := gitMatch(r.pattern, target, r.anchored)

					if match {
						if r.dirOnly && !e.IsDir() {
							continue
						}

						if r.negate {
							ignored = false
						} else {
							ignored = true
						}
					}
				}

				if ignored {
					continue
				}

				info, err := e.Info()
				if err != nil {
					return false
				}

				if e.IsDir() {
					if checkDirSkip(name) {
						if !yield(walkItem{path, info}) {
							return false
						}
					}
				} else {
					if !yield(walkItem{path, info}) {
						return false
					}
				}

				if e.IsDir() && checkDirSkip(name) {
					if !walk(path, relPath, localRules) {
						return false
					}
				}
			}

			return true
		}

		walk(root, "", nil)
	}
}

func gitMatch(pattern, path string, anchored bool) bool {

	pattern = filepath.ToSlash(pattern)
	path = filepath.ToSlash(path)

	if anchored {
		ok, _ := filepath.Match(pattern, path)
		return ok
	}

	// try full match
	ok, _ := filepath.Match(pattern, path)
	if ok {
		return true
	}

	// match any path segment
	parts := strings.Split(path, "/")
	for i := range parts {
		sub := strings.Join(parts[i:], "/")
		ok, _ := filepath.Match(pattern, sub)
		if ok {
			return true
		}
	}

	// basic ** handling
	if strings.Contains(pattern, "**") {

		p := strings.ReplaceAll(pattern, "**", "*")

		ok, _ := filepath.Match(p, path)
		if ok {
			return true
		}

		for i := range parts {
			sub := strings.Join(parts[i:], "/")
			ok, _ := filepath.Match(p, sub)
			if ok {
				return true
			}
		}
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
	hasPattern := pattern != ""

outer:
	for item := range walkDir(dir) {
		select {
		case <-cancel:
			break outer
		default:
		}
		name := item.info.Name()
		isDir := item.info.IsDir()
		if isDir && textSet {
			continue
		}

		if hasPattern {
			matched, err := filepath.Match(pattern, name)
			if err != nil || !matched {
				continue
			}

		}
		var lines []searchLine
		if textSet && !isDir {
			if item.info.Size() > 5_242_880 { // 5M ought to be enough for anybody
				continue
			}
			contains, fileLines, err := fileContainsText(item.path, text)
			if err != nil || !contains {
				continue
			}
			lines = fileLines
		}

		select {
		case result <- searchItem{path: item.path, isDir: isDir, lines: lines}:
		case <-cancel:
			break outer
		}
	}

}

func isBinaryFile(path string) (bool, error) {
	f, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer f.Close()

	buf := make([]byte, 8000)
	n, err := f.Read(buf)
	if err != nil && err != io.EOF {
		return false, err
	}

	buf = buf[:n]

	if slices.Contains(buf, 0) {
		return true, nil
	}

	return false, nil
}

func fileContainsText(path, text string) (bool, []searchLine, error) {
	binary, err := isBinaryFile(path)
	if err != nil {
		return false, nil, err
	}
	if binary {
		return false, nil, err
	}

	f, err := os.Open(path)
	if err != nil {
		return false, nil, err
	}
	defer f.Close()

	var results []searchLine
	scanner := bufio.NewScanner(f)

	lineNumber := 0

	for scanner.Scan() {
		lineNumber++
		line := scanner.Text()

		idx := strings.Index(line, text)
		if idx != -1 {
			results = append(results, searchLine{
				line:       line,
				lineNumber: lineNumber,
				start:      idx,
				end:        idx + len(text),
			})
		}
	}

	if err := scanner.Err(); err != nil {
		return false, nil, err
	}

	return len(results) > 0, results, nil
}

type searchTickMsg struct{}

func searchTick() tea.Cmd {
	return tea.Tick(time.Millisecond*100, func(time.Time) tea.Msg {
		return searchTickMsg{}
	})
}
