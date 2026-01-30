package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/dustin/go-humanize"
)

type mode int

const (
	normal mode = iota
	visual
	shell
)

type itemType int

const (
	directory itemType = iota
	file
)

type item struct {
	entry    os.DirEntry
	selected bool
}

type tab struct {
	dir string
}

type page struct {
	dir    string
	items  []item
	cursor int
}

type theme struct {
	baseStyle   lipgloss.Style
	emptyStyle  lipgloss.Style
	cursorStyle lipgloss.Style

	cursorColor   lipgloss.Color
	dirColor      lipgloss.Color
	grayTextColor lipgloss.Color
}

func newTheme() theme {
	return theme{
		baseStyle:   lipgloss.NewStyle().Background(lipgloss.Color("#222222")),
		emptyStyle:  lipgloss.NewStyle().Background(lipgloss.Color("#1a1a1a")),
		cursorStyle: lipgloss.NewStyle().Background(lipgloss.Color("#3a3a3a")),

		cursorColor:   lipgloss.Color("#438a2c"),
		dirColor:      lipgloss.Color("#579ddf"),
		grayTextColor: lipgloss.Color("#8f8f8f"),
	}
}

type model struct {
	err        error
	tabs       []tab
	currentTab int
	pages      map[string]page
	mode       mode
	width      int
	height     int

	theme theme
}

func (m *model) getPage() page {
	dir := m.tabs[m.currentTab].dir
	return m.pages[dir]
}

func (m *model) cursorDown() {
	page := m.getPage()
	if len(page.items)-2 >= page.cursor {
		page.cursor += 1
	}
	m.pages[page.dir] = page
}

func (m *model) cursorUp() {
	page := m.getPage()
	if page.cursor > 0 {
		page.cursor -= 1
	}
	m.pages[page.dir] = page
}

func initialModel(dir string) model {
	pages := make(map[string]page)
	pages[dir] = page{dir: dir}

	return model{
		tabs: []tab{
			{dir: dir},
		},
		currentTab: 0,
		pages:      pages,
		mode:       normal,
		theme:      newTheme(),
	}
}

type errorMsg struct {
	err error
}

type readDirMsg struct {
	entries []os.DirEntry
	dir     string
}

func newErr(msg string) tea.Cmd {
	return func() tea.Msg {
		return errorMsg{errors.New(msg)}
	}
}

func (m model) readDir(dir string) tea.Cmd {
	return func() tea.Msg {
		entries, err := os.ReadDir(dir)

		if err != nil {
			return errorMsg{err}
		}

		sort.Slice(entries, func(i, j int) bool {
			if entries[i].IsDir() == entries[j].IsDir() {
				return entries[i].Name() < entries[j].Name()
			}
			return entries[i].IsDir()
		})

		return readDirMsg{entries: entries, dir: dir}
	}
}

func (m model) Init() tea.Cmd {

	return m.readDir(m.tabs[0].dir)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {

	case errorMsg:
		m.err = msg.err
		return m, nil

	case readDirMsg:
		page := m.pages[msg.dir]
		page.items = nil
		for _, entry := range msg.entries {
			page.items = append(page.items, item{entry: entry})
		}
		m.pages[msg.dir] = page
		return m, nil

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		return m, nil

	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			if m.mode == normal {
				return m, tea.Quit
			}
		case "d":
			return m, newErr("EPIC FAIL")
		case "j", "down":
			m.cursorDown()
			return m, nil
		case "k", "up":
			m.cursorUp()
			return m, nil
		}
	}

	return m, nil
}

func (m model) View() string {
	if m.err != nil {
		msg := fmt.Sprintf("Error: %s", m.err)
		block := lipgloss.PlaceHorizontal(m.width, lipgloss.Center, msg)
		return lipgloss.PlaceVertical(m.height, lipgloss.Center, block)
	}

	var s strings.Builder

	style := &m.theme.baseStyle
	emptyStyle := &m.theme.emptyStyle

	page := m.getPage()
	dir := lipgloss.PlaceHorizontal(m.width, lipgloss.Left, page.dir)
	s.WriteString(emptyStyle.Bold(true).Render(dir))
	s.WriteRune('\n')

	for i, item := range page.items {
		if i+1 > m.height-3 {
			break
		}

		style = &m.theme.baseStyle

		if i == page.cursor {
			style = &m.theme.cursorStyle
			s.WriteString(style.
				Bold(true).
				Foreground(m.theme.cursorColor).
				Render(" ▶ "))
		} else {
			s.WriteString(style.Render("   "))
		}

		var symlinkPath string
		info, _ := item.entry.Info()
		isSymlink := info.Mode()&os.ModeSymlink != 0
		size := strings.Replace(humanize.Bytes(uint64(info.Size())), " ", "", 1)
		name := item.entry.Name()

		if isSymlink {
			symlinkPath, _ = filepath.EvalSymlinks(filepath.Join(page.dir, name)) // HUHUEHUEHUEHUHE
		}

		isDir := info.IsDir()

		name_block := ""

		if isDir {
			name_block += style.Foreground(m.theme.dirColor).Render(name)
			name_block += style.Render("/")
		} else {
			name_block += style.Render(name)
		}

		if isSymlink {
			name_block += style.Render(" ")
			name_block += style.Render("-> ")
			name_block += style.Render(symlinkPath)
			name_block += style.Render(" ")
		}

		info_block := ""

		if !isDir {
			info_block += style.Render(size)
		}

		name_width := m.width - 3 - lipgloss.Width(info_block)
		name_block_len := lipgloss.Width(name_block)
		if name_block_len > name_width {
			name_block = name_block[:name_width]
			name_runes := []rune(name_block)
			name_runes[name_block_len-1] = '…'
			name_block = string(name_runes)
			s.WriteString(name_block)
		} else {
			len_padding := name_width - name_block_len
			padding_str := style.Render(" ")
			s.WriteString(name_block)
			for range len_padding {
				s.WriteString(padding_str)
			}
		}

		s.WriteString(info_block)

		s.WriteString("\n")
	}

	return s.String()
}

func main() {
	p := tea.NewProgram(initialModel("C:\\"), tea.WithAltScreen())
	_, err := p.Run()
	if err != nil {
		log.Fatalf("failed to launch the program: %s", err)
	}
}
