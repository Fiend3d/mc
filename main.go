package main

import (
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
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
	title  string
	items  []item
	cursor int
}

type model struct {
	tabs       []tab
	currentTab int
	pages      map[string]page
	mode       mode
	width      int
	height     int
}

func initialModel(path string) model {
	pages := make(map[string]page)
	pages[path] = page{title: path}

	return model{
		tabs: []tab{
			{dir: path},
		},
		currentTab: 0,
		pages:      pages,
		mode:       normal,
	}
}

type errorMsg struct {
	err error
}

type readDirMsg struct {
	entries []os.DirEntry
	dir     string
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

	case readDirMsg:
		page := m.pages[msg.dir]
		page.items = nil
		for _, entry := range msg.entries {
			page.items = append(page.items, item{entry: entry})
		}
		m.pages[msg.dir] = page

	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q":
			if m.mode == normal {
				return m, tea.Quit
			}
		}
	}

	return m, nil
}

func (m model) View() string {
	var s strings.Builder

	tab := m.tabs[m.currentTab]
	page := m.pages[tab.dir]
	s.WriteString(page.title)
	s.WriteRune('\n')

	for i, item := range page.items {
		if i+1 > m.height-3 {
			break
		}

		if i == page.cursor {
			s.WriteString("> ")
		} else {
			s.WriteString("  ")
		}

		var symlinkPath string
		info, _ := item.entry.Info()
		isSymlink := info.Mode()&os.ModeSymlink != 0
		size := strings.Replace(humanize.Bytes(uint64(info.Size())), " ", "", 1)
		name := item.entry.Name()

		if isSymlink {
			symlinkPath, _ = filepath.EvalSymlinks(filepath.Join(page.title, name)) // HUHUEHUEHUEHUHE
		}

		isDir := info.IsDir()

		if isDir {
			s.WriteRune('\\')
		}
		s.WriteString(name)
		s.WriteRune(' ')
		if isSymlink {
			s.WriteString("-> ")
			s.WriteString(symlinkPath)
			s.WriteRune(' ')
		}

		if !isDir {
			s.WriteString(size)
		}

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
