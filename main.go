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

	isDir     bool
	isSymlink bool
	name      string
	symlink   string
	modTime   string
	size      string
}

func newItem(entry os.DirEntry, dir string) (*item, error) {
	info, err := entry.Info()
	if err != nil {
		return nil, err
	}

	item := &item{entry: entry, selected: false}

	item.name = entry.Name()
	item.isDir = info.IsDir()
	item.isSymlink = info.Mode()&os.ModeSymlink != 0

	if item.isSymlink {
		target, err := filepath.EvalSymlinks(filepath.Join(dir, item.name))
		if err != nil {
			return nil, err
		}
		item.symlink = target
	}

	item.size = ""
	if !item.isDir {
		item.size = strings.Replace(
			humanize.Bytes(uint64(info.Size())),
			" ",
			"",
			1,
		)
	}

	item.modTime = info.ModTime().Format("02-01-2006 15:04")

	return item, nil
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
	gray := lipgloss.Color("#6272a4")
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

func newErr(err error) tea.Cmd {
	return func() tea.Msg {
		return errorMsg{err}
	}
}

func (m model) readDir(dir string) tea.Cmd {
	return func() tea.Msg {
		entries, err := os.ReadDir(dir)
		if err != nil {
			return errorMsg{err}
		}

		filteredEntries := make([]os.DirEntry, 0, len(entries))
		for _, entry := range entries {
			name := entry.Name()
			lowerName := strings.ToLower(name)

			// Skip Windows/system files and folders
			switch lowerName {
			// System files
			case "thumbs.db":
				continue
			case "desktop.ini":
				continue
			case "dumpstack.log.tmp":
				continue

			// System folders (legacy and modern)
			case "$recycle.bin":
				continue
			case "system volume information":
				continue
			case "documents and settings": // XP legacy junction
				continue
			case "recovery": // Windows Recovery folder
				continue
			case "config.msi": // Windows Installer temp
				continue

			// Windows system files
			case "pagefile.sys":
				continue
			case "hiberfil.sys":
				continue
			case "swapfile.sys":
				continue
			case "bootmgr":
				continue
			case "bootnxt":
				continue
			}

			filteredEntries = append(filteredEntries, entry)
		}

		// Sort: directories first, then by name (case-insensitive)
		sort.Slice(filteredEntries, func(i, j int) bool {
			iIsDir := filteredEntries[i].IsDir()
			jIsDir := filteredEntries[j].IsDir()

			if iIsDir && !jIsDir {
				return true
			}
			if !iIsDir && jIsDir {
				return false
			}

			return strings.ToLower(filteredEntries[i].Name()) < strings.ToLower(filteredEntries[j].Name())
		})

		return readDirMsg{entries: filteredEntries, dir: dir}
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
			item, err := newItem(msg.entries[i], page.dir)
			if err != nil {
				return m, newErr(err)
			}
			page.items = append(page.items, *item)
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
			return m, newErr(errors.New("EPIC FAIL"))
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

		item := &page.items[i]

		// --- name block ---
		var nameBlock strings.Builder

		if item.isDir {
			nameBlock.WriteString(
				style.Foreground(m.theme.accentColor4).Render(item.name),
			)
			nameBlock.WriteString(style.Bold(true).Render("/"))
		} else {
			nameBlock.WriteString(
				style.Foreground(m.theme.whiteColor).Render(item.name),
			)
		}

		if item.isSymlink {
			nameBlock.WriteString(style.Render(" -> "))
			nameBlock.WriteString(style.Render(item.symlink))
		}

		nameWidth := max(
			m.width-cursorWidth-sizeWidth-timeWidth-colGap*2+1, 1)

		name := nameBlock.String()

		if lipgloss.Width(name) > nameWidth {
			name = truncate.StringWithTail(
				name,
				uint(nameWidth),
				"…",
			)
		}

		s.WriteString(name)
		nameLen := lipgloss.Width(name)
		if nameLen < nameWidth {
			s.WriteString(style.Render(strings.Repeat(" ", nameWidth-nameLen)))
		}

		// time column
		timeStyle := style.Foreground(m.theme.grayColor)
		s.WriteString(timeStyle.Render(item.modTime))

		s.WriteString(style.Render(" "))

		// size column
		s.WriteString(style.Render(
			lipgloss.PlaceHorizontal(sizeWidth, lipgloss.Center, item.size)))
		s.WriteString(style.Render(item.size))

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
