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
	"github.com/muesli/reflow/truncate"
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

	whiteColor lipgloss.Color
	grayColor  lipgloss.Color

	accentColor1 lipgloss.Color
	accentColor2 lipgloss.Color
	accentColor3 lipgloss.Color
	accentColor4 lipgloss.Color
	accentColor5 lipgloss.Color
}

func newTheme() theme {
	white := lipgloss.Color("#ffffff")
	gray := lipgloss.Color("#979bb3")
	accent1 := lipgloss.Color("#ff79c6")
	accent2 := lipgloss.Color("#bd93f9")
	accent3 := lipgloss.Color("#8be9fd")
	accent4 := lipgloss.Color("#f1fa8c")
	accent5 := lipgloss.Color("#ffb86c")

	defaultStyle := lipgloss.NewStyle().Foreground(white)
	return theme{
		baseStyle:   defaultStyle.Background(lipgloss.Color("#282a36")),
		emptyStyle:  defaultStyle.Background(lipgloss.Color("#222430")),
		cursorStyle: defaultStyle.Background(lipgloss.Color("#44475a")),

		whiteColor: white,
		grayColor:  gray,

		accentColor1: accent1,
		accentColor2: accent2,
		accentColor3: accent3,
		accentColor4: accent4,
		accentColor5: accent5,
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
		for i := range msg.entries {
			page.items = append(page.items, item{entry: msg.entries[i]})
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
		return lipgloss.Place(
			m.width,
			m.height,
			lipgloss.Center,
			lipgloss.Center,
			msg,
		)
	}

	var s strings.Builder

	base := &m.theme.baseStyle
	empty := &m.theme.emptyStyle

	page := m.getPage()

	// Header (directory)
	dirLine := lipgloss.PlaceHorizontal(m.width, lipgloss.Left, page.dir)
	s.WriteString(empty.Bold(true).Render(dirLine))
	s.WriteRune('\n')

	const (
		cursorWidth = 3
		sizeWidth   = 8  // "123.4KB"
		timeWidth   = 16 // "YYYY-MM-DD HH:MM"
		colGap      = 1
	)

	for i := range page.items {
		if i+1 > m.height-3 {
			break
		}

		style := base
		if i == page.cursor {
			style = &m.theme.cursorStyle
			s.WriteString(
				style.
					Bold(true).
					Foreground(m.theme.accentColor1).
					Render(" > "),
			)
		} else {
			s.WriteString(style.Render("   "))
		}

		entry := page.items[i].entry
		info, err := entry.Info()
		if err != nil {
			s.WriteString("\n")
			continue
		}

		name := entry.Name()
		isDir := info.IsDir()
		isSymlink := info.Mode()&os.ModeSymlink != 0

		// --- name block ---
		var nameBlock strings.Builder

		if isDir {
			nameBlock.WriteString(
				style.Foreground(m.theme.accentColor4).Render(name),
			)
			nameBlock.WriteString(style.Bold(true).Render("/"))
		} else {
			nameBlock.WriteString(
				style.Foreground(m.theme.whiteColor).Render(name),
			)
		}

		if isSymlink {
			target, _ := filepath.EvalSymlinks(filepath.Join(page.dir, name))
			nameBlock.WriteString(style.Render(" -> "))
			nameBlock.WriteString(style.Render(target))
		}

		nameStr := nameBlock.String()

		// --- size ---
		sizeStr := ""
		if !isDir {
			sizeStr = strings.Replace(
				humanize.Bytes(uint64(info.Size())),
				" ",
				"",
				1,
			)
		}

		// --- modified time ---
		modTime := info.ModTime().Format("02-01-2006 15:04")

		// --- layout ---
		nameWidth := max(m.width-
			cursorWidth-
			sizeWidth-
			timeWidth-
			colGap*2, 0)

		if lipgloss.Width(nameStr) > nameWidth {
			nameStr = truncate.StringWithTail(
				nameStr,
				uint(nameWidth),
				"…",
			)
		}

		s.WriteString(nameStr)
		s.WriteString(
			style.Render(strings.Repeat(
				" ",
				nameWidth-lipgloss.Width(nameStr),
			)),
		)

		// time column
		timeStyle := style.Foreground(m.theme.grayColor)
		s.WriteString(timeStyle.Render(modTime))

		// gap
		s.WriteString(style.Render(" "))

		// size column (right-aligned)
		s.WriteString(style.Render(strings.Repeat(
			" ",
			sizeWidth-lipgloss.Width(sizeStr),
		)))
		s.WriteString(style.Render(sizeStr))

		s.WriteRune('\n')
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
